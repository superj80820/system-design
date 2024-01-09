package domain

import (
	"time"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
)

var (
	ErrEmptyOrderBook = errors.New("empty order book error")
	ErrNoOrder        = errors.New("order not found error")
)

type MatchingUseCase interface {
	NewOrder(o *OrderEntity) (*MatchResult, error)
	CancelOrder(ts time.Time, o *OrderEntity) error
}

type MatchResult struct {
	TakerOrder   *OrderEntity
	MatchDetails []*MatchDetail
}

type MatchDetail struct {
	Price      decimal.Decimal
	Quantity   decimal.Decimal
	TakerOrder *OrderEntity
	MakerOrder *OrderEntity
}
