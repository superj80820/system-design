package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type TradingUseCase interface {
	NewOrder(order *Order) (*MatchResult, error)
	CancelOrder(ts time.Time, order *Order) error
	GetMarketPrice() decimal.Decimal
	GetLatestSequenceID() int
}
