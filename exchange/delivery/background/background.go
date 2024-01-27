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
	quotationUseCase domain.QuotationUseCase,
	candleUseCase domain.CandleUseCase,
	orderUseCase domain.OrderUseCase,
	tradingUseCase domain.TradingUseCase,
) error {
	tradingSequencerUseCase.ConsumeTradingEventThenProduce(ctx)
	tradingUseCase.SubscribeTradingResult(quotationUseCase.AddTick)
	tradingUseCase.SubscribeTradingResult(candleUseCase.AddData) // TODO: error handle
	tradingUseCase.SubscribeTradingResult(orderUseCase.SaveHistoryOrdersFromTradingResult)
	tradingUseCase.SubscribeTradingResult(tradingUseCase.SaveHistoryMatchDetailsFromTradingResult)

	select {
	case <-tradingSequencerUseCase.Done():
		return errors.Wrap(tradingSequencerUseCase.Err(), "trading sequencer use case get error")
	}
}
