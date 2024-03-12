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
	errLock       *sync.Mutex
	err           error
	doneCh        chan struct{}
}

func CreateQuotationUseCase(ctx context.Context, tradingRepo domain.TradingRepo, quotationRepo domain.QuotationRepo) domain.QuotationUseCase {
	q := &quotationUseCase{
		tradingRepo:   tradingRepo,
		quotationRepo: quotationRepo,
		errLock:       new(sync.Mutex),
		doneCh:        make(chan struct{}),
	}

	return q
}

func (q *quotationUseCase) ConsumeTicksToSave(ctx context.Context, key string) {
	setErrAndDone := func(err error) error {
		err = errors.Wrap(err, "consume ticks get error")
		q.errLock.Lock()
		q.err = err
		q.errLock.Unlock()
		close(q.doneCh)
		return err
	}

	q.quotationRepo.ConsumeTicksMQWithCommit(ctx, key, func(sequenceID int, ticks []*domain.TickEntity, commitFn func() error) error {
		if err := q.quotationRepo.SaveTickStrings(ctx, sequenceID, ticks); err != nil && !errors.Is(err, domain.ErrNoop) { // TODO: error handle
			return setErrAndDone(errors.Wrap(err, "save tick failed"))
		}
		if err := commitFn(); err != nil {
			return setErrAndDone(errors.Wrap(err, "commit failed"))
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
