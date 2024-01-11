package domain

import "context"

type TradingSequencerUseCase interface {
	ProcessMessages(tradingEvent *TradingEvent)
	SequenceMessages(tradingEvent *TradingEvent)
	SendTradeSequenceMessages(*TradingEvent)
}

type AsyncTradingSequencerUseCase interface {
	AsyncEventProcess(ctx context.Context)
}

type TradingSequencerRepo interface {
	GetMaxSequenceID() uint64
	SaveEvent(tradingEvent *TradingEvent)
	SubscribeTradeSequenceMessage(notify func(*TradingEvent))
	SendTradeSequenceMessages(*TradingEvent)
	Shutdown()
}
