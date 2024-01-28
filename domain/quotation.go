package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type QuotationUseCase interface {
	ConsumeTradingResult(key string)
	GetTicks() ([]string, error)
}

type TickEntity struct {
	ID             int
	SequenceID     int
	TakerOrderID   int
	MakerOrderID   int
	TakerDirection DirectionEnum
	Price          decimal.Decimal
	Quantity       decimal.Decimal
	CreatedAt      time.Time
}
