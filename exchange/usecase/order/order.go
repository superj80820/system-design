package order

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/util"
)

type orderUseCase struct {
	assetUseCase domain.UserAssetUseCase
	orderRepo    domain.OrderRepo

	baseCurrencyID  int
	quoteCurrencyID int

	activeOrders  util.GenericSyncMap[int, *domain.OrderEntity]
	userOrdersMap util.GenericSyncMap[int, *util.GenericSyncMap[int, *domain.OrderEntity]]
}

func CreateOrderUseCase(
	assetUseCase domain.UserAssetUseCase,
	orderRepo domain.OrderRepo,
) domain.OrderUseCase {
	o := &orderUseCase{
		assetUseCase:    assetUseCase,
		orderRepo:       orderRepo,
		baseCurrencyID:  int(domain.BaseCurrencyType),
		quoteCurrencyID: int(domain.QuoteCurrencyType),
	}

	return o
}

func (o *orderUseCase) CreateOrder(ctx context.Context, sequenceID int, orderID int, userID int, direction domain.DirectionEnum, price decimal.Decimal, quantity decimal.Decimal, ts time.Time) (*domain.OrderEntity, *domain.TransferResult, error) {
	var err error
	transferResult := new(domain.TransferResult)

	switch direction {
	case domain.DirectionSell:
		transferResult, err = o.assetUseCase.Freeze(ctx, userID, o.baseCurrencyID, quantity)
		if err != nil {
			return nil, nil, errors.Wrap(err, "freeze base currency failed")
		}
	case domain.DirectionBuy:
		transferResult, err = o.assetUseCase.Freeze(ctx, userID, o.quoteCurrencyID, price.Mul(quantity))
		if err != nil {
			return nil, nil, errors.Wrap(err, "freeze base currency failed")
		}
	default:
		return nil, nil, errors.New("unknown direction")
	}

	order := domain.OrderEntity{
		ID:               orderID,
		SequenceID:       sequenceID,
		UserID:           userID,
		Direction:        direction,
		Price:            price,
		Quantity:         quantity,
		UnfilledQuantity: quantity,
		Status:           domain.OrderStatusPending,
		CreatedAt:        ts,
		UpdatedAt:        ts,
	}

	o.activeOrders.Store(orderID, &order) // TODO: test performance
	var userOrders util.GenericSyncMap[int, *domain.OrderEntity]
	userOrders.Store(orderID, &order)
	if userOrders, loaded := o.userOrdersMap.LoadOrStore(userID, &userOrders); loaded {
		userOrders.Store(orderID, &order)
	}

	return &order, transferResult, nil
}

func (o *orderUseCase) GetOrder(orderID int) (*domain.OrderEntity, error) {
	order, ok := o.activeOrders.Load(orderID)
	if !ok {
		return nil, domain.ErrNoOrder
	}
	return order, nil
}

// TODO: is clone logic correct?
func (o *orderUseCase) GetUserOrders(userId int) (map[int]*domain.OrderEntity, error) {
	userOrders, ok := o.userOrdersMap.Load(userId)
	if !ok {
		return nil, errors.New("get user orders failed")
	}
	userOrdersClone := make(map[int]*domain.OrderEntity)
	userOrders.Range(func(key int, value *domain.OrderEntity) bool {
		userOrdersClone[key] = value
		return true
	})
	return userOrdersClone, nil
}

func (o *orderUseCase) RemoveOrder(ctx context.Context, orderID int) error {
	removedOrder, loaded := o.activeOrders.LoadAndDelete(orderID)
	if !loaded {
		return errors.New("order not found in active orders")
	}
	userOrders, ok := o.userOrdersMap.Load(removedOrder.UserID)
	if !ok {
		return errors.New("user orders not found")
	}
	_, loaded = userOrders.LoadAndDelete(orderID)
	if !loaded {
		return errors.New("order not found in user orders")
	}

	return nil
}

func (o *orderUseCase) ConsumeOrderResultToSave(ctx context.Context, key string) {
	o.orderRepo.ConsumeOrderMQBatch(ctx, key, func(sequenceID int, orders []*domain.OrderEntity) error {
		// TODO: york 冪等性
		var saveOrders []*domain.OrderEntity
		for _, order := range orders {
			if order.Status.IsFinalStatus() {
				saveOrders = append(saveOrders, order)
			}
		}

		if err := o.orderRepo.SaveHistoryOrdersWithIgnore(saveOrders); err != nil {
			return errors.Wrap(err, "save history order with ignore failed") // TODO: async error handle
		}

		return nil
	})
}

func (o *orderUseCase) GetHistoryOrder(userID int, orderID int) (*domain.OrderEntity, error) {
	order, err := o.orderRepo.GetHistoryOrder(userID, orderID)
	if err != nil {
		return nil, errors.Wrap(err, "get history order failed")
	}
	return order, nil
}

func (o *orderUseCase) GetHistoryOrders(orderID, maxResults int) ([]*domain.OrderEntity, error) {
	if maxResults < 1 {
		return nil, errors.New("max results can not less 1")
	} else if maxResults > 1000 {
		return nil, errors.New("max results can not more 1000")
	}
	orders, err := o.orderRepo.GetHistoryOrders(orderID, maxResults)
	if err != nil {
		return nil, errors.Wrap(err, "get history orders failed")
	}
	return orders, nil
}

func (o *orderUseCase) GetOrdersData() ([]*domain.OrderEntity, error) {
	var cloneOrders []*domain.OrderEntity
	o.activeOrders.Range(func(orderID int, order *domain.OrderEntity) bool {
		cloneOrders = append(cloneOrders, &domain.OrderEntity{
			ID:         order.ID,
			SequenceID: order.SequenceID,
			UserID:     order.UserID,

			Price:     order.Price,
			Direction: order.Direction,
			Status:    order.Status,

			Quantity:         order.Quantity,
			UnfilledQuantity: order.UnfilledQuantity,

			CreatedAt: order.CreatedAt,
			UpdatedAt: order.UpdatedAt,
		})
		return true
	})
	return cloneOrders, nil
}

func (o *orderUseCase) RecoverBySnapshot(tradingSnapshot *domain.TradingSnapshot) error {
	for _, order := range tradingSnapshot.Orders { // TODO: test performance
		o.activeOrders.Store(order.ID, order)
		var userOrders util.GenericSyncMap[int, *domain.OrderEntity]
		userOrders.Store(order.ID, order)
		if userOrders, loaded := o.userOrdersMap.LoadOrStore(order.UserID, &userOrders); loaded {
			userOrders.Store(order.ID, order)
		}
	}
	return nil
}
