package background

import (
	"context"

	"github.com/superj80820/system-design/domain"
)

func RunAsyncTrading(ctx context.Context, asyncTradingUseCase domain.AsyncTradingUseCase) error {
	<-asyncTradingUseCase.Done()
	return nil // TODO: error
}

func RunAsyncTradingSequencer(ctx context.Context, asyncTradingSequencerUseCase domain.AsyncTradingSequencerUseCase) {
	asyncTradingSequencerUseCase.AsyncEventProcess(ctx)
}
