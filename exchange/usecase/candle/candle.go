package candle

import (
	"context"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

type candleUseCase struct {
	candleRepo domain.CandleRepo
	done       chan struct{}
	err        error
}

func CreateCandleUseCase(ctx context.Context, candleRepo domain.CandleRepo) domain.CandleUseCase {
	c := &candleUseCase{
		candleRepo: candleRepo,
		done:       make(chan struct{}),
	}

	return c
}

func (c *candleUseCase) ConsumeTradingResultToSave(ctx context.Context, key string) {
	c.candleRepo.ConsumeCandleMQByTradingResultWithCommit(ctx, key, func(tradingResult *domain.TradingResult, commitFn func() error) error {
		if err := c.candleRepo.SaveBarByMatchResult(ctx, tradingResult.MatchResult); err != nil {
			return errors.Wrap(err, "save bar failed")
		}
		if err := commitFn(); err != nil {
			return errors.Wrap(err, "commit failed")
		}
		return nil
	})
}

// GetBar response: [timestamp, openPrice, highPrice, lowPrice, closePrice, quantity]
func (c *candleUseCase) GetBar(ctx context.Context, timeType domain.CandleTimeType, start, stop string, sortOrderBy domain.SortOrderByEnum) ([]string, error) {
	bars, err := c.candleRepo.GetBar(ctx, timeType, start, stop, sortOrderBy)
	if err != nil {
		return nil, errors.Wrap(err, "get bar failed")
	}
	return bars, nil
}

func (c *candleUseCase) Done() <-chan struct{} {
	return c.done
}

func (c *candleUseCase) Err() error {
	return c.err
}
