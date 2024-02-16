package quotation

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

type quotationUseCase struct {
	tradingRepo   domain.TradingRepo
	quotationRepo domain.QuotationRepo
	cap           int
	errLock       *sync.Mutex
	err           error
	doneCh        chan struct{}
}

func CreateQuotationUseCase(ctx context.Context, tradingRepo domain.TradingRepo, quotationRepo domain.QuotationRepo, cap int) domain.QuotationUseCase {
	q := &quotationUseCase{
		tradingRepo:   tradingRepo,
		quotationRepo: quotationRepo,
		cap:           cap,
		errLock:       new(sync.Mutex),
		doneCh:        make(chan struct{}),
	}

	return q
}

func (q *quotationUseCase) ConsumeTick(ctx context.Context, key string) {
	q.quotationRepo.ConsumeTicksSaveMQ(ctx, key, func(sequenceID int, ticks []*domain.TickEntity) error {
		if err := q.quotationRepo.SaveTickStrings(ctx, sequenceID, ticks); err != nil && !errors.Is(err, domain.ErrNoop) { // TODO: maybe use batch
			q.errLock.Lock()
			q.err = errors.Wrap(err, "save ticks failed")
			q.errLock.Unlock()
			close(q.doneCh)
			return nil
		}
		if err := q.quotationRepo.ProduceTicks(ctx, sequenceID, ticks); err != nil {
			q.errLock.Lock()
			q.err = errors.Wrap(err, "produce tick failed")
			q.errLock.Unlock()
			close(q.doneCh)
			return nil
		}
		return nil
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
