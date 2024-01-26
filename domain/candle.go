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

type CandleRepo interface {
	AddData(
		ctx context.Context,
		sequenceID int,
		createdAt time.Time,
		openPrice, highPrice, lowPrice, closePrice, quantity decimal.Decimal,
	) error
	GetBar(ctx context.Context, timeType CandleTimeType, min, max string) ([]string, error)
}

type CandleUseCase interface {
	AddData(tradingResult *TradingResult)
	GetBar(ctx context.Context, timeType CandleTimeType, min, max string) ([]string, error)
	Done() <-chan struct{}
	Err() error
}
