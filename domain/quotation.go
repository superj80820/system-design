package domain

import (
	context "context"
	"time"

	"github.com/shopspring/decimal"
)

type QuotationRepo interface {
	GetTickStrings(ctx context.Context, start int64, stop int64) ([]string, error)
	SaveTickStrings(ctx context.Context, sequenceID int, ticks []*TickEntity) error
}

type QuotationUseCase interface {
	ConsumeTradingResult(key string)
	GetTickStrings(context.Context, int64, int64) ([]string, error)
	Done() <-chan struct{}
	Err() error
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
