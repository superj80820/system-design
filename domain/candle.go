package domain

import (
	"context"

	"github.com/shopspring/decimal"
)

type CandleTimeType int

const (
	CandleTimeTypeSec CandleTimeType = iota + 1
	CandleTimeTypeMin
	CandleTimeTypeHour
	CandleTimeTypeDay
)

type CandleBar struct {
	Type       CandleTimeType `gorm:"-"` // TODO: test
	StartTime  int
	ClosePrice decimal.Decimal
	HighPrice  decimal.Decimal
	LowPrice   decimal.Decimal
	OpenPrice  decimal.Decimal
	Quantity   decimal.Decimal
}

type CandleRepo interface {
	GetBar(ctx context.Context, timeType CandleTimeType, start, stop string, sortOrderBy SortOrderByEnum) ([]string, error)
	SaveBar(candleBar *CandleBar) error

	ProduceCandleMQByMatchResult(ctx context.Context, matchResult *MatchResult) error
	ConsumeCandleMQ(ctx context.Context, key string, notify func(candleBar *CandleBar) error)
}

type CandleUseCase interface {
	GetBar(ctx context.Context, timeType CandleTimeType, start, stop string, sortOrderBy SortOrderByEnum) ([]string, error)
	ConsumeTradingResultToSave(ctx context.Context, key string)
	Done() <-chan struct{}
	Err() error
}
