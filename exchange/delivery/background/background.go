package background

import (
	"context"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

func RunAsyncTrading(ctx context.Context, asyncTradingUseCase domain.AsyncTradingUseCase) error {
	<-asyncTradingUseCase.Done()
	return nil // TODO: error
}

func RunAsyncTradingSequencer(
	ctx context.Context,
	tradingSequencerUseCase domain.TradingSequencerUseCase,
	asyncTradingUseCase domain.AsyncTradingUseCase,
	quotationUseCase domain.QuotationUseCase,
	candleUseCase domain.CandleUseCase,
) error {
	tradingSequencerUseCase.ConsumeTradingEvent(ctx)
	asyncTradingUseCase.SubscribeTradingResult(quotationUseCase.AddTick)
	asyncTradingUseCase.SubscribeTradingResult(candleUseCase.AddData) // TODO: error handle

	select {
	case <-tradingSequencerUseCase.Done():
		return errors.Wrap(tradingSequencerUseCase.Err(), "trading sequencer use case get error")
	}
}
