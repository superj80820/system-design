package trading

import (
	"context"
	"strconv"
	"time"

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

	orderBookDepth     int
	isOrderBookChanged bool
	latestOrderBook    *domain.OrderBookEntity
	lastSequenceID     int
	cancel             context.CancelFunc
	doneCh             chan struct{}
	err                error
}

func CreateTradingUseCase(
	ctx context.Context,
	matchingUseCase domain.MatchingUseCase,
	userAssetUseCase domain.UserAssetUseCase,
	orderUseCase domain.OrderUseCase,
	clearingUseCase domain.ClearingUseCase,
	tradingRepo domain.TradingRepo,
	orderBookDepth int,
) domain.TradingUseCase {
	ctx, cancel := context.WithCancel(ctx)

	t := &tradingUseCase{
		matchingUseCase:  matchingUseCase,
		userAssetUseCase: userAssetUseCase,
		orderUseCase:     orderUseCase,
		clearingUseCase:  clearingUseCase,
		orderBookDepth:   orderBookDepth,
		cancel:           cancel,
		doneCh:           make(chan struct{}),
	}

	return t
}

func (t *tradingUseCase) ProcessMessages(messages []*domain.TradingEvent) error {
	t.isOrderBookChanged = false
	for _, message := range messages {
		if err := t.processMessage(message); err != nil {
			return errors.Wrap(err, "process event failed")
		}
	}
	if t.isOrderBookChanged {
		t.latestOrderBook = t.matchingUseCase.GetOrderBook(t.orderBookDepth)
	}
	return nil
}

func (t *tradingUseCase) processMessage(message *domain.TradingEvent) error {
	if message.SequenceID <= t.lastSequenceID {
		return errors.New("skip duplicate event: " + strconv.Itoa(message.SequenceID))
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
		if err := t.createOrder(message); err != nil {
			return errors.Wrap(err, "create order failed")
		}
	case domain.TradingEventCancelOrderType:
		if err := t.cancelOrder(message); err != nil {
			return errors.Wrap(err, "cancel order failed")
		}
	case domain.TradingEventTransferType:
		if err := t.transfer(message); err != nil {
			return errors.Wrap(err, "transfer failed")
		}
	default:
		return errors.New("unknown event type")
	}
	t.lastSequenceID = message.SequenceID
	return nil
}

func (t *tradingUseCase) createOrder(tradingEvent *domain.TradingEvent) error {
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
	t.isOrderBookChanged = true

	return nil
}

func (t *tradingUseCase) cancelOrder(tradingEvent *domain.TradingEvent) error {
	order, err := t.orderUseCase.GetOrder(tradingEvent.OrderCancelEvent.OrderId)
	if err != nil {
		return errors.Wrap(err, "get order failed")
	}
	if order.UserID != tradingEvent.OrderCancelEvent.OrderId {
		return errors.New("order does not belong to this user")
	}
	if err := t.matchingUseCase.CancelOrder(tradingEvent.CreatedAt, order); err != nil {
		return errors.Wrap(err, "cancel order failed")
	}
	if err := t.clearingUseCase.ClearCancelOrder(order); err != nil {
		return errors.Wrap(err, "clear cancel order failed")
	}
	t.isOrderBookChanged = true
	return nil
}

func (t *tradingUseCase) transfer(tradingEvent *domain.TradingEvent) error {
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

func (t *tradingUseCase) Shutdown() error {
	t.cancel()
	<-t.doneCh
	return t.err
}
