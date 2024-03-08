package trading

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

type syncTradingUseCase struct {
	matchingUseCase  domain.MatchingUseCase
	userAssetUseCase domain.UserAssetUseCase
	orderUseCase     domain.OrderUseCase
	clearingUseCase  domain.ClearingUseCase
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

func (t *syncTradingUseCase) CreateOrder(ctx context.Context, tradingEvent *domain.TradingEvent) (*domain.MatchResult, *domain.TransferResult, error) {
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
	clearTransferResult, err := t.clearingUseCase.ClearMatchResult(ctx, matchResult)
	if err != nil {
		return nil, nil, errors.Wrap(err, "clear match order failed")
	}

	transferResult.UserAssets = append(transferResult.UserAssets, clearTransferResult.UserAssets...)

	return matchResult, transferResult, nil
}

func (t *syncTradingUseCase) CancelOrder(ctx context.Context, tradingEvent *domain.TradingEvent) (*domain.CancelResult, *domain.TransferResult, error) {
	order, err := t.orderUseCase.GetOrder(tradingEvent.OrderCancelEvent.OrderId)
	if err != nil {
		return nil, nil, errors.Wrap(err, fmt.Sprintf("get order failed, order id: %d", tradingEvent.OrderCancelEvent.OrderId))
	}
	if order.UserID != tradingEvent.OrderCancelEvent.UserID {
		return nil, nil, errors.New("order does not belong to this user")
	}
	cancelOrderResult, err := t.matchingUseCase.CancelOrder(order, tradingEvent.CreatedAt)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cancel order failed")
	}
	transferResult, err := t.clearingUseCase.ClearCancelOrder(ctx, cancelOrderResult)
	if err != nil {
		return nil, nil, errors.Wrap(err, "clear cancel order failed")
	}

	return cancelOrderResult, transferResult, nil
}

func (t *syncTradingUseCase) Transfer(ctx context.Context, tradingEvent *domain.TradingEvent) (*domain.TransferResult, error) {
	transferResult, err := t.userAssetUseCase.TransferAvailableToAvailable(
		ctx,
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
