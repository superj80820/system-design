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
	matchingUseCase domain.MatchingUseCase,
) error {
	tradingSequencerUseCase.ConsumeTradingEventThenProduce(ctx)
	orderUseCase.ConsumeOrderResult(ctx, "global-order")
	quotationUseCase.ConsumeTick(ctx, "global-quotation")
	candleUseCase.ConsumeTradingResult(ctx, "global-candle") // TODO: error handle
	matchingUseCase.ConsumeMatchResult(ctx, "global-matching")

	tradingSnapshot, err := tradingUseCase.GetHistorySnapshot(ctx)
	if !errors.Is(err, domain.ErrNoData) && err != nil {
		return errors.Wrap(err, "get history snapshot failed")
	}
	if tradingSnapshot != nil {
		if err := tradingUseCase.RecoverBySnapshot(tradingSnapshot); err != nil {
			return errors.Wrap(err, "recover by snapshot failed")
		}
	}

	select {
	case <-tradingSequencerUseCase.Done():
		return errors.Wrap(tradingSequencerUseCase.Err(), "trading sequencer use case get error")
	case <-candleUseCase.Done():
		return errors.Wrap(candleUseCase.Err(), "candle use case get error")
	case <-quotationUseCase.Done():
		return errors.Wrap(quotationUseCase.Err(), "quotation use case get error")
	}
}
