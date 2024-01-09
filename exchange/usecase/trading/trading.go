package trading

import (
	"context"
	"strconv"
	"time"

	"container/list"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	loggerKit "github.com/superj80820/system-design/kit/logger"
)

type tradingUseCase struct {
	matchingUseCase  domain.MatchingUseCase
	userAssetUseCase domain.UserAssetUseCase
	orderUseCase     domain.OrderUseCase
	clearingUseCase  domain.ClearingUseCase
	logger           loggerKit.Logger

	lastSequenceID int
	saveOrderCh    chan *domain.OrderEntity
}

func CreateOrderUseCase(
	ctx context.Context,
	matchingUseCase domain.MatchingUseCase,
	userAssetUseCase domain.UserAssetUseCase,
	orderUseCase domain.OrderUseCase,
	clearingUseCase domain.ClearingUseCase,
) domain.TradingUseCase {
	t := &tradingUseCase{
		matchingUseCase:  matchingUseCase,
		userAssetUseCase: userAssetUseCase,
		orderUseCase:     orderUseCase,
		clearingUseCase:  clearingUseCase,
		saveOrderCh:      make(chan *domain.OrderEntity),
	}
	go t.saveOrders(ctx)

	return t
}

func (t *tradingUseCase) ProcessMessages(messages []*domain.TradingEvent) error {
	for _, message := range messages {
		if err := t.ProcessEvent(message); err != nil {
			return errors.New("TODO")
		}
	}
	return nil
}

func (t *tradingUseCase) ProcessEvent(message *domain.TradingEvent) error {
	if message.SequenceID <= t.lastSequenceID {
		t.logger.Warn("skip duplicate event: " + strconv.Itoa(message.SequenceID))
		return nil
	}
	if message.PreviousID > t.lastSequenceID { // TODO: think
		// TODO: load from db
		return errors.New("miss message")
	}
	if message.PreviousID != t.lastSequenceID {
		return errors.New("previous message not correct")
	}
	switch message.EventType {
	case domain.TradingEventCreateOrderType:
		if err := t.CreateOrder(message); err != nil {
			return errors.Wrap(err, "create order failed")
		}
	case domain.TradingEventCancelOrderType:
		if err := t.CancelOrder(message); err != nil {
			return errors.Wrap(err, "cancel order failed")
		}
	case domain.TradingEventTransferType:
	default:
		return errors.New("unknown event type")
	}
	t.lastSequenceID = message.SequenceID
	return nil
}

func (t *tradingUseCase) CreateOrder(tradingEvent *domain.TradingEvent) error {
	timeNow := time.Now()
	year := timeNow.Year()
	month := int(timeNow.Month())
	orderID := tradingEvent.SequenceID*10000 + (year*100 + month)

	order, err := t.orderUseCase.CreateOrder(
		tradingEvent.SequenceID,
		orderID,
		tradingEvent.OrderRequestEvent.UserID,
		tradingEvent.OrderRequestEvent.Direction,
		tradingEvent.OrderRequestEvent.Price,
		tradingEvent.OrderRequestEvent.Quantity,
		tradingEvent.CreatedAt,
	)
	if err != nil {
		return errors.Wrap(err, "create order failed")
	}
	matchResult, err := t.matchingUseCase.NewOrder(order)
	if err != nil {
		return errors.Wrap(err, "matching order failed")
	}
	if err := t.clearingUseCase.ClearMatchResult(matchResult); err != nil {
		return errors.Wrap(err, "clear match order failed")
	}

	if len(matchResult.MatchDetails) != 0 {
		var closedOrders []*domain.OrderEntity
		if matchResult.TakerOrder.Status.IsFinalStatus() {
			closedOrders = append(closedOrders, matchResult.TakerOrder)
		}
		for _, matchDetail := range matchResult.MatchDetails {
			maker := matchDetail.MakerOrder
			if maker.Status.IsFinalStatus() {
				closedOrders = append(closedOrders, maker)
			}
		}
	}

	return nil
}

// TODO
func (t *tradingUseCase) CancelOrder(tradingEvent *domain.TradingEvent) error {
	order, err := t.orderUseCase.GetOrder(tradingEvent.OrderCancelEvent.OrderId)
	if errors.Is(err, domain.ErrNoOrder) {
		return errors.New("TODO")
	} else if err != nil {
		return errors.New("TODO")
	}
	if order.UserID != tradingEvent.OrderCancelEvent.OrderId {
		return errors.New("TODO")
	}
	if err := t.matchingUseCase.CancelOrder(tradingEvent.CreatedAt, order); err != nil {
		return errors.New("TODO")
	}
	if err := t.clearingUseCase.ClearCancelOrder(order); err != nil {
		return errors.New("TODO")
	}
	return nil
}

func (t *tradingUseCase) Transfer(tradingEvent *domain.TradingEvent) error {
	if err := t.userAssetUseCase.Transfer(
		domain.AssetTransferAvailableToAvailable,
		tradingEvent.TransferEvent.FromUserID,
		tradingEvent.TransferEvent.ToUserID,
		tradingEvent.TransferEvent.AssetID,
		tradingEvent.TransferEvent.Amount,
	); err != nil {
		return errors.Wrap(err, "transfer error")
	}
	return nil
}

func (t *tradingUseCase) saveOrders(ctx context.Context) {
	orders := list.New()
	// curFront := orders.Front() // TODO
	ticker := time.NewTicker(100 * time.Millisecond) // TODO: is best way?
	defer ticker.Stop()

	for {
		select {
		case order := <-t.saveOrderCh:
			orders.PushBack(order)
		case <-ticker.C:
			// TODO
			// temp := curFront
			// curFront = orders.Front().Next()
			// orders.Remove(temp)
		case <-ctx.Done():
			return
		}
	}
}

// func (m *tradingUseCase) CancelOrder(ts time.Time, o *domain.OrderEntity) error {
// 	panic("")
// }

// func (m *tradingUseCase) NewOrder(o *domain.OrderEntity) (*domain.MatchResult, error) {
// 	panic("")
// 	// matchResult, err := m.matchingUseCase.NewOrder(o)
// 	// if err != nil {
// 	// 	return nil, errors.Wrap(err, "process order failed")
// 	// }
// 	// return matchResult, nil
// }

// func (m *tradingUseCase) GetMarketPrice() decimal.Decimal {
// 	panic("")
// }

// func (m *tradingUseCase) GetLatestSequenceID() int {
// 	panic("")
// }
