package domain

import "context"

type TradingSequencerUseCase interface {
	ProcessMessages(tradingEvent *TradingEvent)
	SequenceMessages(tradingEvent *TradingEvent)
	SendMessages(tradingEvent *TradingEvent)
}

type AsyncTradingSequencerUseCase interface {
	AsyncEventProcess(ctx context.Context)
}

type TradingSequencerRepo interface {
	GetMaxSequenceID() uint64
	SaveEvent(tradingEvent *TradingEvent)
}
