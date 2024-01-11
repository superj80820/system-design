package sequencer

import (
	"context"

	"github.com/superj80820/system-design/domain"
)

type asyncTradingSequencerUseCase struct {
	tradingSequencerUseCase domain.TradingSequencerUseCase
	tradingRepo             domain.TradingRepo
	tradingSequencerRepo    domain.TradingSequencerRepo
}

func CreateAsyncTradingSequencerUseCase(tradingSequencerUseCase domain.TradingSequencerUseCase, tradingRepo domain.TradingRepo, tradingSequencerRepo domain.TradingSequencerRepo) domain.AsyncTradingSequencerUseCase {
	return &asyncTradingSequencerUseCase{
		tradingSequencerUseCase: tradingSequencerUseCase,
		tradingRepo:             tradingRepo,
		tradingSequencerRepo:    tradingSequencerRepo,
	}
}

func (a *asyncTradingSequencerUseCase) AsyncEventProcess(ctx context.Context) {
	a.tradingSequencerRepo.SubscribeTradeSequenceMessage(func(te *domain.TradingEvent) {
		a.tradingSequencerUseCase.ProcessMessages(te)
	})
}
