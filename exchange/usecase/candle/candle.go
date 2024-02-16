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

func (c *candleUseCase) ConsumeTradingResult(ctx context.Context, key string) {
	c.candleRepo.ConsumeCandleSaveMQ(ctx, key, func(candleBar *domain.CandleBar) error {
		if err := c.candleRepo.SaveBar(candleBar); err != nil {
			return errors.Wrap(err, "save bar failed")
		}
		c.candleRepo.ProduceCandle(ctx, candleBar)
		return nil
	})
}

// GetBar response: [timestamp, openPrice, highPrice, lowPrice, closePrice, quantity]
func (c *candleUseCase) GetBar(ctx context.Context, timeType domain.CandleTimeType, min, max string) ([]string, error) {
	bars, err := c.candleRepo.GetBar(ctx, timeType, min, max)
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
