package candle

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

type candleUseCase struct {
	tradingResults     []*domain.TradingResult
	tradingResultsLock *sync.Mutex
	tradingRepo        domain.TradingRepo
	candleRepo         domain.CandleRepo
	done               chan struct{}
	err                error
}

func CreateCandleUseCase(ctx context.Context, tradingRepo domain.TradingRepo, candleRepo domain.CandleRepo) domain.CandleUseCase {
	c := &candleUseCase{
		tradingRepo:        tradingRepo,
		candleRepo:         candleRepo,
		tradingResultsLock: new(sync.Mutex),
		done:               make(chan struct{}),
	}

	go c.collectCandleThenSave(ctx)

	return c
}

func (c *candleUseCase) ConsumeTradingResult(key string) {
	c.tradingRepo.SubscribeTradingResult(key, func(tradingResult *domain.TradingResult) {
		c.tradingResultsLock.Lock()
		defer c.tradingResultsLock.Unlock()
		if tradingResult.TradingResultStatus != domain.TradingResultStatusCreate {
			return
		}
		c.tradingResults = append(c.tradingResults, tradingResult)
	})
}

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

func (c *candleUseCase) collectCandleThenSave(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond) // TODO: is best way?
	defer ticker.Stop()

	for range ticker.C {
		c.tradingResultsLock.Lock()
		tradingResultsClone := make([]*domain.TradingResult, len(c.tradingResults))
		copy(tradingResultsClone, c.tradingResults)
		c.tradingResults = nil
		c.tradingResultsLock.Unlock()

		for _, tradingResult := range tradingResultsClone {
			if err := c.candleRepo.AddData(
				ctx,
				tradingResult.TradingEvent.SequenceID,
				tradingResult.TradingEvent.CreatedAt,
				tradingResult.MatchResult.MatchDetails,
			); err != nil {
				c.err = errors.Wrap(err, "add data failed")
				close(c.done)
				return
			}
		}
	}
}
