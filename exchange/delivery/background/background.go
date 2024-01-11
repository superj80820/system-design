package background

import (
	"context"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"golang.org/x/sync/errgroup"
)

func RunAsyncTrading(ctx context.Context, asyncTradingUseCase domain.AsyncTradingUseCase) error {
	var eg errgroup.Group
	eg.Go(func() error {
		return asyncTradingUseCase.AsyncEventProcess(ctx)
	})
	eg.Go(func() error {
		return asyncTradingUseCase.AsyncDBProcess(ctx)
	})
	eg.Go(func() error {
		return asyncTradingUseCase.AsyncTickProcess(ctx)
	})
	eg.Go(func() error {
		return asyncTradingUseCase.AsyncNotifyProcess(ctx)
	})
	eg.Go(func() error {
		return asyncTradingUseCase.AsyncOrderBookProcess(ctx)
	})
	eg.Go(func() error {
		return asyncTradingUseCase.AsyncAPIResultProcess(ctx)
	})
	if err := eg.Wait(); err != nil {
		return errors.Wrap(err, "get async process error")
	}
	return nil
}

func RunAsyncTradingSequencer(ctx context.Context, asyncTradingSequencerUseCase domain.AsyncTradingSequencerUseCase) {
	asyncTradingSequencerUseCase.AsyncEventProcess(ctx)
}
