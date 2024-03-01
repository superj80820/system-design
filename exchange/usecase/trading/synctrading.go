package trading

import (
	"context"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

type syncTradingUseCase struct {
	matchingUseCase  domain.MatchingUseCase
	userAssetUseCase domain.UserAssetUseCase
	orderUseCase     domain.OrderUseCase
	clearingUseCase  domain.ClearingUseCase

	lastSequenceID int
}

func CreateSyncTradingUseCase(
	ctx context.Context,
	matchingUseCase domain.MatchingUseCase,
	userAssetUseCase domain.UserAssetUseCase,
	orderUseCase domain.OrderUseCase,
	clearingUseCase domain.ClearingUseCase,
) domain.SyncTradingUseCase {
	return &syncTradingUseCase{
		matchingUseCase:  matchingUseCase,
		userAssetUseCase: userAssetUseCase,
		orderUseCase:     orderUseCase,
		clearingUseCase:  clearingUseCase,
	}
}

func (t *syncTradingUseCase) checkEventSequence(tradingEvent *domain.TradingEvent) error {
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

func (t *syncTradingUseCase) CreateOrder(ctx context.Context, tradingEvent *domain.TradingEvent) (*domain.MatchResult, []*domain.TransferResult, error) {
	if err := t.checkEventSequence(tradingEvent); err != nil {
		return nil, nil, errors.Wrap(err, "check event sequence failed")
	}

	order, transferResult, err := t.orderUseCase.CreateOrder(
		ctx,
		tradingEvent.SequenceID,
		tradingEvent.OrderRequestEvent.OrderID,
		tradingEvent.OrderRequestEvent.UserID,
		tradingEvent.OrderRequestEvent.Direction,
		tradingEvent.OrderRequestEvent.Price,
		tradingEvent.OrderRequestEvent.Quantity,
		tradingEvent.CreatedAt,
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "create order failed")
	}
	matchResult, err := t.matchingUseCase.NewOrder(ctx, order)
	if err != nil {
		return nil, nil, errors.Wrap(err, "matching order failed")
	}
	transferResults, err := t.clearingUseCase.ClearMatchResult(ctx, matchResult)
	if err != nil {
		return nil, nil, errors.Wrap(err, "clear match order failed")
	}

	if len(transferResults) > 0 {
		return matchResult, transferResults, nil
	}
	return matchResult, []*domain.TransferResult{transferResult}, nil
}

func (t *syncTradingUseCase) CancelOrder(ctx context.Context, tradingEvent *domain.TradingEvent) (*domain.OrderEntity, *domain.TransferResult, error) {
	if err := t.checkEventSequence(tradingEvent); err != nil {
		return nil, nil, errors.Wrap(err, "check event sequence failed")
	}

	order, err := t.orderUseCase.GetOrder(tradingEvent.OrderCancelEvent.OrderId)
	if err != nil {
		return nil, nil, errors.Wrap(err, fmt.Sprintf("get order failed, order id: %d", tradingEvent.OrderCancelEvent.OrderId))
	}
	if order.UserID != tradingEvent.OrderCancelEvent.UserID {
		return nil, nil, errors.New("order does not belong to this user")
	}
	if err := t.matchingUseCase.CancelOrder(tradingEvent.CreatedAt, order); err != nil {
		return nil, nil, errors.Wrap(err, "cancel order failed")
	}
	transferResult, err := t.clearingUseCase.ClearCancelOrder(ctx, order)
	if err != nil {
		return nil, nil, errors.Wrap(err, "clear cancel order failed")
	}

	return order, transferResult, nil
}

func (t *syncTradingUseCase) Transfer(ctx context.Context, tradingEvent *domain.TradingEvent) (*domain.TransferResult, error) {
	if err := t.checkEventSequence(tradingEvent); err != nil {
		return nil, errors.Wrap(err, "check event sequence failed")
	}

	transferResult, err := t.userAssetUseCase.Transfer(
		ctx,
		domain.AssetTransferAvailableToAvailable,
		tradingEvent.TransferEvent.FromUserID,
		tradingEvent.TransferEvent.ToUserID,
		tradingEvent.TransferEvent.AssetID,
		tradingEvent.TransferEvent.Amount,
	)
	if err != nil {
		return nil, errors.Wrap(err, "transfer error")
	}

	return transferResult, nil
}

func (s *syncTradingUseCase) Deposit(ctx context.Context, tradingEvent *domain.TradingEvent) (*domain.TransferResult, error) {
	if err := s.checkEventSequence(tradingEvent); err != nil {
		return nil, errors.Wrap(err, "check event sequence failed")
	}

	transferResult, err := s.userAssetUseCase.LiabilityUserTransfer(
		ctx,
		tradingEvent.DepositEvent.ToUserID,
		tradingEvent.DepositEvent.AssetID,
		tradingEvent.DepositEvent.Amount,
	)
	if err != nil {
		return nil, errors.Wrap(err, "liability user transfer failed")
	}

	return transferResult, nil
}

func (s *syncTradingUseCase) GetSequenceID() int {
	return s.lastSequenceID
}

func (s *syncTradingUseCase) RecoverBySnapshot(tradingSnapshot *domain.TradingSnapshot) error {
	if err := s.userAssetUseCase.RecoverBySnapshot(tradingSnapshot); err != nil {
		return errors.Wrap(err, "recover by snapshot failed")
	}
	if err := s.orderUseCase.RecoverBySnapshot(tradingSnapshot); err != nil {
		return errors.Wrap(err, "recover by snapshot failed")
	}
	if err := s.matchingUseCase.RecoverBySnapshot(tradingSnapshot); err != nil {
		return errors.Wrap(err, "recover by snapshot failed")
	}
	s.lastSequenceID = tradingSnapshot.SequenceID
	return nil
}
