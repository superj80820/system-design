package order

import (
	"fmt"
	"sort"
	"sync"
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

	historyClosedOrdersLock     *sync.Mutex
	historyClosedOrders         []*domain.OrderEntity
	isHistoryClosedOrdersFullCh chan struct{}
}

func CreateOrderUseCase(
	assetUseCase domain.UserAssetUseCase,
	orderRepo domain.OrderRepo,
	baseCurrencyID,
	quoteCurrencyID int,
) domain.OrderUseCase {
	o := &orderUseCase{
		assetUseCase:                assetUseCase,
		orderRepo:                   orderRepo,
		baseCurrencyID:              baseCurrencyID,
		quoteCurrencyID:             quoteCurrencyID,
		historyClosedOrdersLock:     new(sync.Mutex),
		isHistoryClosedOrdersFullCh: make(chan struct{}),
	}

	go o.collectHistoryClosedOrdersThenSave()

	return o
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
	userOrdersClone := make(map[int]*domain.OrderEntity)
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

func (o *orderUseCase) SaveHistoryOrdersFromTradingResult(tradingResult *domain.TradingResult) {
	if tradingResult.TradingResultStatus != domain.TradingResultStatusCreate {
		return
	}
	var closedOrders []*domain.OrderEntity
	if tradingResult.MatchResult.TakerOrder.Status.IsFinalStatus() {
		closedOrders = append(closedOrders, tradingResult.MatchResult.TakerOrder)
	}
	for _, matchDetail := range tradingResult.MatchResult.MatchDetails {
		if matchDetail.MakerOrder.Status.IsFinalStatus() {
			closedOrders = append(closedOrders, matchDetail.MakerOrder)
		}
	}
	if len(closedOrders) == 0 {
		return
	}
	sort.Slice(closedOrders, func(i, j int) bool { // TODO: maybe no need
		return closedOrders[i].SequenceID > closedOrders[j].SequenceID
	})
	o.historyClosedOrdersLock.Lock()
	for _, closedOrder := range closedOrders { // TODO: maybe use in one for-loop
		o.historyClosedOrders = append(o.historyClosedOrders, closedOrder)
	}
	historyClosedOrdersLength := len(o.historyClosedOrders)
	o.historyClosedOrdersLock.Unlock()
	if historyClosedOrdersLength >= 1000 {
		o.isHistoryClosedOrdersFullCh <- struct{}{}
	}
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

func (o *orderUseCase) collectHistoryClosedOrdersThenSave() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	errContinue := errors.New("continue")
	fn := func() error {
		historyClosedOrdersClone, err := func() ([]*domain.OrderEntity, error) {
			o.historyClosedOrdersLock.Lock()
			defer o.historyClosedOrdersLock.Unlock()

			if len(o.historyClosedOrders) == 0 {
				return nil, errContinue
			}
			historyClosedOrdersClone := make([]*domain.OrderEntity, len(o.historyClosedOrders))
			copy(historyClosedOrdersClone, o.historyClosedOrders)
			o.historyClosedOrders = nil

			return historyClosedOrdersClone, nil
		}()
		if err != nil {
			return errors.Wrap(err, "clone history closed orders failed")
		}

		if err := o.orderRepo.SaveHistoryOrdersWithIgnore(historyClosedOrdersClone); err != nil { // TODO: use batch
			return errors.Wrap(err, "save history order with ignore failed")
		}

		return nil
	}

	for {
		select {
		case <-ticker.C:
			if err := fn(); errors.Is(err, errContinue) {
				continue
			} else if err != nil {
				panic(fmt.Sprintf("TODO, error: %+v", err))
			}
		case <-o.isHistoryClosedOrdersFullCh:
			if err := fn(); errors.Is(err, errContinue) {
				continue
			} else if err != nil {
				panic(fmt.Sprintf("TODO, error: %+v", err))
			}
		}
	}
}
