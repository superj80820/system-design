package domain

import (
	"context"
	"time"

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
	StartTime  int
	ClosePrice decimal.Decimal
	HighPrice  decimal.Decimal
	LowPrice   decimal.Decimal
	OpenPrice  decimal.Decimal
	Quantity   decimal.Decimal
}

type CandleRepo interface {
	AddData(ctx context.Context, sequenceID int, createdAt time.Time, matchDetails []*MatchDetail) error
	GetBar(ctx context.Context, timeType CandleTimeType, min, max string) ([]string, error)
}

type CandleUseCase interface {
	ConsumeTradingResult(key string)
	GetBar(ctx context.Context, timeType CandleTimeType, min, max string) ([]string, error)
	Done() <-chan struct{}
	Err() error
}
