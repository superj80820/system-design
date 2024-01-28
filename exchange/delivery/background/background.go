package background

import (
	"context"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

func RunAsyncTradingSequencer(
	ctx context.Context,
	tradingSequencerUseCase domain.TradingSequencerUseCase,
	quotationUseCase domain.QuotationUseCase,
	candleUseCase domain.CandleUseCase,
	orderUseCase domain.OrderUseCase,
	tradingUseCase domain.TradingUseCase,
) error {
	tradingSequencerUseCase.ConsumeTradingEventThenProduce(ctx)
	quotationUseCase.ConsumeTradingResult("global-quotation")
	candleUseCase.ConsumeTradingResult("global-candle") // TODO: error handle
	orderUseCase.ConsumeTradingResult("global-order")
	tradingUseCase.ConsumeTradingResult("global-trading")

	select {
	case <-tradingSequencerUseCase.Done():
		return errors.Wrap(tradingSequencerUseCase.Err(), "trading sequencer use case get error")
	case <-candleUseCase.Done():
		return errors.Wrap(candleUseCase.Err(), "candle use case get error")
	}
}
