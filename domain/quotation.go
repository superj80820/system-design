package domain

import (
	context "context"
	"time"

	"github.com/shopspring/decimal"
)

type QuotationRepo interface {
	GetTickStrings(ctx context.Context, start int64, stop int64) ([]string, error)
	SaveTickStrings(ctx context.Context, sequenceID int, ticks []*TickEntity) error

	ProduceTicksMQByTradingResults(ctx context.Context, tradingResults []*TradingResult) error
	ConsumeTicksMQ(ctx context.Context, key string, notify func(sequenceID int, ticks []*TickEntity) error)
	ConsumeTicksMQWithCommit(ctx context.Context, key string, notify func(sequenceID int, ticks []*TickEntity, commitFn func() error) error)
}

type QuotationNotifyRepo interface {
	ConsumeTicksMQWithRingBuffer(ctx context.Context, key string, notify func(sequenceID int, ticks []*TickEntity) error)
	StopConsume(ctx context.Context, key string)
}

type QuotationUseCase interface {
	ConsumeTicksToSave(ctx context.Context, key string)
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
