package background

import (
	"context"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

func RunAsyncTradingSequencer(
	ctx context.Context,
	quotationUseCase domain.QuotationUseCase,
	candleUseCase domain.CandleUseCase,
	orderUseCase domain.OrderUseCase,
	tradingUseCase domain.TradingUseCase,
	matchingUseCase domain.MatchingUseCase,
) error {
	tradingUseCase.ConsumeTradingEventThenProduce(ctx)
	orderUseCase.ConsumeOrderResultToSave(ctx, "global-save-order")
	quotationUseCase.ConsumeTickToSave(ctx, "global-save-quotation")
	candleUseCase.ConsumeTradingResultToSave(ctx, "global-save-candle") // TODO: error handle
	matchingUseCase.ConsumeMatchResultToSave(ctx, "global-save-matching")

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
	case <-tradingUseCase.Done():
		return errors.Wrap(tradingUseCase.Err(), "trading use case get error")
	case <-candleUseCase.Done():
		return errors.Wrap(candleUseCase.Err(), "candle use case get error")
	case <-quotationUseCase.Done():
		return errors.Wrap(quotationUseCase.Err(), "quotation use case get error")
	}
}
