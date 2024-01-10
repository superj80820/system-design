package order

import (
	"time"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/util"
)

type orderUseCase struct {
	assetUseCase domain.UserAssetUseCase

	baseCurrencyID  int
	quoteCurrencyID int

	activeOrders  util.GenericSyncMap[int, *domain.OrderEntity]
	userOrdersMap util.GenericSyncMap[int, *util.GenericSyncMap[int, *domain.OrderEntity]]
}

func (o *orderUseCase) CreateOrder(sequenceID int, orderID int, userID int, direction domain.DirectionEnum, price decimal.Decimal, quantity decimal.Decimal, ts time.Time) (*domain.OrderEntity, error) {
	switch direction {
	case domain.DirectionSell:
		if err := o.assetUseCase.Freeze(userID, o.baseCurrencyID, quantity); err != nil {
			return nil, errors.Wrap(err, "freeze base currency failed")
		}
	case domain.DirectionBuy:
		if err := o.assetUseCase.Freeze(userID, o.quoteCurrencyID, price.Mul(quantity)); err != nil {
			return nil, errors.Wrap(err, "freeze base currency failed")
		}
	default:
		return nil, errors.New("unknown direction")
	}

	order := domain.OrderEntity{
		ID:               orderID,
		SequenceID:       sequenceID,
		UserID:           userID,
		Direction:        direction,
		Price:            price,
		Quantity:         quantity,
		UnfilledQuantity: quantity,
		CreatedAt:        ts,
		UpdatedAt:        ts,
	}

	o.activeOrders.Store(orderID, &order)
	var userOrders util.GenericSyncMap[int, *domain.OrderEntity]
	userOrders.Store(orderID, &order)
	if userOrders, loaded := o.userOrdersMap.LoadOrStore(userID, &userOrders); loaded {
		userOrders.Store(orderID, &order)
	}

	return &order, nil
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
	var userOrdersClone map[int]*domain.OrderEntity
	userOrders.Range(func(key int, value *domain.OrderEntity) bool {
		userOrdersClone[key] = value
		return true
	})
	return userOrdersClone, nil
}

func (o *orderUseCase) RemoveOrder(orderID int) error {
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

func CreateOrderUseCase(
	assetUseCase domain.UserAssetUseCase,
	baseCurrencyID,
	quoteCurrencyID int,
) domain.OrderUseCase {
	return &orderUseCase{
		assetUseCase:    assetUseCase,
		baseCurrencyID:  baseCurrencyID,
		quoteCurrencyID: quoteCurrencyID,
	}
}
