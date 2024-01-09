package domain

type ClearingUseCase interface {
	ClearMatchResult(matchResult *MatchResult) error
	ClearCancelOrder(order *OrderEntity) error
}
