package domain

import "context"

type ClearingUseCase interface {
	ClearMatchResult(ctx context.Context, matchResult *MatchResult) error
	ClearCancelOrder(ctx context.Context, order *OrderEntity) error
}
