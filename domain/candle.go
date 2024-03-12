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
	SaveBarByMatchResult(ctx context.Context, matchResult *MatchResult) error

	ProduceCandleMQByTradingResults(ctx context.Context, tradingResults []*TradingResult) error
	ConsumeCandleMQByTradingResultWithCommit(ctx context.Context, key string, notify func(tradingResult *TradingResult, commitFn func() error) error)

	ProduceCandleMQ(ctx context.Context, candleBar *CandleBar) error
	ConsumeCandleMQ(ctx context.Context, key string, notify func(candleBar *CandleBar) error)
}

type CandleNotifyRepo interface {
	ConsumeCandleMQWithRingBuffer(ctx context.Context, key string, notify func(candleBar *CandleBar) error)
	StopConsume(ctx context.Context, key string)
}

type CandleUseCase interface {
	GetBar(ctx context.Context, timeType CandleTimeType, start, stop string, sortOrderBy SortOrderByEnum) ([]string, error)
	ConsumeTradingResultToSave(ctx context.Context, key string)
	Done() <-chan struct{}
	Err() error
}
