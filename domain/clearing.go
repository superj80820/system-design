package domain

import "context"

type ClearingUseCase interface {
	ClearMatchResult(ctx context.Context, matchResult *MatchResult) (*TransferResult, error)
	ClearCancelOrder(ctx context.Context, order *OrderEntity) (*TransferResult, error)
}
