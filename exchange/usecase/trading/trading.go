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

	latestOrderBook *domain.OrderBookEntity
	lastSequenceID  int
	cancel          context.CancelFunc
	doneCh          chan struct{}
	err             error
}

func CreateTradingUseCase(
	ctx context.Context,
	matchingUseCase domain.MatchingUseCase,
	userAssetUseCase domain.UserAssetUseCase,
	orderUseCase domain.OrderUseCase,
	clearingUseCase domain.ClearingUseCase,
	tradingRepo domain.TradingRepo,
) domain.TradingUseCase {
	ctx, cancel := context.WithCancel(ctx)

	t := &tradingUseCase{
		matchingUseCase:  matchingUseCase,
		userAssetUseCase: userAssetUseCase,
		orderUseCase:     orderUseCase,
		clearingUseCase:  clearingUseCase,
		cancel:           cancel,
		doneCh:           make(chan struct{}),
	}

	return t
}

func (t *tradingUseCase) checkEventSequence(tradingEvent *domain.TradingEvent) error {
	if tradingEvent.SequenceID <= t.lastSequenceID {
		return errors.Wrap(domain.ErrGetDuplicateEvent, "skip duplicate, last sequence id: "+strconv.Itoa(t.lastSequenceID)+", message event sequence id: "+strconv.Itoa(tradingEvent.SequenceID))
	}
	if tradingEvent.PreviousID > t.lastSequenceID {
		// TODO: load from db
		return errors.Wrap(domain.ErrMissEvent, "last sequence id: "+strconv.Itoa(t.lastSequenceID)+", message event previous id: "+strconv.Itoa(tradingEvent.PreviousID))
	}
	if tradingEvent.PreviousID != t.lastSequenceID { // TODO: test think maybe no need previous
		return errors.Wrap(domain.ErrPreviousIDNotCorrect, "last sequence id: "+strconv.Itoa(t.lastSequenceID)+", message event previous id: "+strconv.Itoa(tradingEvent.PreviousID))
	}

	t.lastSequenceID = tradingEvent.SequenceID

	return nil
}

func (t *tradingUseCase) CreateOrder(tradingEvent *domain.TradingEvent) (*domain.MatchResult, error) {
	if err := t.checkEventSequence(tradingEvent); err != nil {
		return nil, errors.Wrap(err, "check event sequence failed")
	}

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
		return nil, errors.Wrap(err, "create order failed")
	}
	matchResult, err := t.matchingUseCase.NewOrder(order)
	if err != nil {
		return nil, errors.Wrap(err, "matching order failed")
	}
	if err := t.clearingUseCase.ClearMatchResult(matchResult); err != nil {
		return nil, errors.Wrap(err, "clear match order failed")
	}

	return matchResult, nil
}

func (t *tradingUseCase) CancelOrder(tradingEvent *domain.TradingEvent) error {
	if err := t.checkEventSequence(tradingEvent); err != nil {
		return errors.Wrap(err, "check event sequence failed")
	}

	order, err := t.orderUseCase.GetOrder(tradingEvent.OrderCancelEvent.OrderId)
	if err != nil {
		return errors.Wrap(err, "get order failed")
	}
	if order.UserID != tradingEvent.OrderCancelEvent.UserID {
		return errors.New("order does not belong to this user")
	}
	if err := t.matchingUseCase.CancelOrder(tradingEvent.CreatedAt, order); err != nil {
		return errors.Wrap(err, "cancel order failed")
	}
	if err := t.clearingUseCase.ClearCancelOrder(order); err != nil {
		return errors.Wrap(err, "clear cancel order failed")
	}

	return nil
}

func (t *tradingUseCase) Transfer(tradingEvent *domain.TradingEvent) error {
	if err := t.checkEventSequence(tradingEvent); err != nil {
		return errors.Wrap(err, "check event sequence failed")
	}

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
