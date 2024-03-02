package order

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
)

type orderUseCase struct {
	assetUseCase domain.UserAssetUseCase
	orderRepo    domain.OrderRepo

	baseCurrencyID  int
	quoteCurrencyID int

	lock          *sync.RWMutex
	activeOrders  map[int]*domain.OrderEntity
	userOrdersMap map[int]map[int]*domain.OrderEntity
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
		lock:            new(sync.RWMutex),
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

	o.lock.Lock()
	o.activeOrders[orderID] = &order
	if _, ok := o.userOrdersMap[userID]; !ok {
		o.userOrdersMap[userID] = make(map[int]*domain.OrderEntity)
	}
	o.userOrdersMap[userID][orderID] = &order
	o.lock.Unlock()

	return &order, transferResult, nil
}

func (o *orderUseCase) GetOrder(orderID int) (*domain.OrderEntity, error) {
	o.lock.RLock()
	defer o.lock.RUnlock()

	order, ok := o.activeOrders[orderID]
	if !ok {
		return nil, errors.Wrap(domain.ErrNoOrder, "not found order")
	}
	return cloneOrder(order), nil
}

func (o *orderUseCase) GetUserOrders(userId int) (map[int]*domain.OrderEntity, error) {
	o.lock.RLock()
	defer o.lock.RUnlock()

	userOrders, ok := o.userOrdersMap[userId]
	if !ok {
		return nil, errors.New("get user orders failed")
	}
	userOrdersClone := make(map[int]*domain.OrderEntity, len(userOrders))
	for orderID, order := range userOrders {
		userOrdersClone[orderID] = cloneOrder(order)
	}
	return userOrdersClone, nil
}

func (o *orderUseCase) RemoveOrder(ctx context.Context, orderID int) error {
	o.lock.Lock()
	defer o.lock.Unlock()

	removedOrder, ok := o.activeOrders[orderID]
	if !ok {
		return errors.New("order not found in active orders")
	}
	delete(o.activeOrders, orderID)

	userOrders, ok := o.userOrdersMap[removedOrder.UserID]
	if !ok {
		return errors.New("user orders not found")
	}
	_, ok = userOrders[orderID]
	if !ok {
		return errors.New("order not found in user orders")
	}
	delete(userOrders, orderID)

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
	o.lock.RLock()
	defer o.lock.RUnlock()

	cloneOrders := make([]*domain.OrderEntity, 0, len(o.activeOrders))
	for _, order := range o.activeOrders {
		cloneOrders = append(cloneOrders, cloneOrder(order))
	}

	return cloneOrders, nil
}

func (o *orderUseCase) RecoverBySnapshot(tradingSnapshot *domain.TradingSnapshot) error {
	o.lock.Lock()
	defer o.lock.Unlock()

	for _, order := range tradingSnapshot.Orders {
		cloneOrderInstance := cloneOrder(order)
		o.activeOrders[order.ID] = cloneOrderInstance
		userOrders, ok := o.userOrdersMap[order.UserID]
		if !ok {
			o.userOrdersMap = make(map[int]map[int]*domain.OrderEntity)
		}
		userOrders[order.ID] = cloneOrderInstance
	}
	return nil
}

func cloneOrder(order *domain.OrderEntity) *domain.OrderEntity {
	return &domain.OrderEntity{
		ID:               order.ID,
		SequenceID:       order.SequenceID,
		UserID:           order.UserID,
		Price:            order.Price,
		Direction:        order.Direction,
		Status:           order.Status,
		Quantity:         order.Quantity,
		UnfilledQuantity: order.UnfilledQuantity,
		CreatedAt:        order.CreatedAt,
		UpdatedAt:        order.UpdatedAt,
	}
}
