package sequencer

import (
	"context"

	"github.com/superj80820/system-design/domain"
)

type asyncTradingSequencerUseCase struct {
	tradingSequencerUseCase domain.TradingSequencerUseCase
	tradingRepo             domain.TradingRepo
}

func CreateAsyncTradingSequencerUseCase(tradingSequencerUseCase domain.TradingSequencerUseCase, tradingRepo domain.TradingRepo) domain.AsyncTradingSequencerUseCase {
	return &asyncTradingSequencerUseCase{
		tradingSequencerUseCase: tradingSequencerUseCase,
		tradingRepo:             tradingRepo,
	}
}

func (a *asyncTradingSequencerUseCase) AsyncEventProcess(ctx context.Context) {
	a.tradingRepo.SubscribeTradeMessage(func(te *domain.TradingEvent) {
		a.tradingSequencerUseCase.ProcessMessages(te)
	})
}
