package quotation

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

type quotationUseCase struct {
	tradingRepo        domain.TradingRepo
	quotationRepo      domain.QuotationRepo
	tradingResults     []*domain.TradingResult
	tradingResultsLock *sync.Mutex
	cap                int
	errLock            *sync.Mutex
	err                error
	doneCh             chan struct{}
}

func CreateQuotationUseCase(ctx context.Context, tradingRepo domain.TradingRepo, quotationRepo domain.QuotationRepo, cap int) domain.QuotationUseCase {
	q := &quotationUseCase{
		tradingRepo:        tradingRepo,
		quotationRepo:      quotationRepo,
		cap:                cap,
		tradingResultsLock: new(sync.Mutex),
		errLock:            new(sync.Mutex),
		doneCh:             make(chan struct{}),
	}

	go q.collectTickThenSave(ctx)

	return q
}

func (q *quotationUseCase) ConsumeTradingResult(key string) {
	q.tradingRepo.SubscribeTradingResult(key, func(tradingResult *domain.TradingResult) {
		q.tradingResultsLock.Lock()
		defer q.tradingResultsLock.Unlock()
		if tradingResult.TradingResultStatus != domain.TradingResultStatusCreate {
			return
		}
		q.tradingResults = append(q.tradingResults, tradingResult)
	})
}

func (q *quotationUseCase) GetTickStrings(ctx context.Context, start int64, stop int64) ([]string, error) {
	ticks, err := q.quotationRepo.GetTickStrings(ctx, start, stop)
	if err != nil {
		return nil, errors.Wrap(err, "get ticks failed")
	}
	return ticks, nil
}

func (q *quotationUseCase) Done() <-chan struct{} {
	return q.doneCh
}

func (q *quotationUseCase) Err() error {
	q.errLock.Lock()
	defer q.errLock.Unlock()

	return q.err
}

func (q *quotationUseCase) collectTickThenSave(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond) // TODO: is best way?
	defer ticker.Stop()

	for range ticker.C {
		q.tradingResultsLock.Lock()
		tradingResultsClone := make([]*domain.TradingResult, len(q.tradingResults))
		copy(tradingResultsClone, q.tradingResults)
		q.tradingResults = nil
		q.tradingResultsLock.Unlock()

		for _, tradingResult := range tradingResultsClone {
			ticks := make([]*domain.TickEntity, len(tradingResult.MatchResult.MatchDetails))
			for idx, matchDetail := range tradingResult.MatchResult.MatchDetails {
				tick := domain.TickEntity{
					SequenceID:     tradingResult.TradingEvent.SequenceID,
					TakerOrderID:   matchDetail.TakerOrder.ID,
					MakerOrderID:   matchDetail.MakerOrder.ID,
					Price:          matchDetail.Price,
					Quantity:       matchDetail.Quantity,
					TakerDirection: matchDetail.TakerOrder.Direction,
					CreatedAt:      tradingResult.TradingEvent.CreatedAt,
				}
				ticks[idx] = &tick
			}
			if err := q.quotationRepo.SaveTickStrings(ctx, tradingResult.TradingEvent.SequenceID, ticks); err != nil && !errors.Is(err, domain.ErrNoop) {
				q.errLock.Lock()
				q.err = errors.Wrap(err, "save ticks failed")
				q.errLock.Unlock()
				close(q.doneCh)
				return
			}
		}
	}
}
