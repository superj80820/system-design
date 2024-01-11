package memory

import "github.com/superj80820/system-design/domain"

type tradingSequencerRepo struct{}

func CreateTradingSequencerRepo() domain.TradingSequencerRepo {
	return &tradingSequencerRepo{}
}

func (*tradingSequencerRepo) GetMaxSequenceID() uint64 {
	return 0
}

// SaveEvent implements domain.TradingSequencerRepo.
func (*tradingSequencerRepo) SaveEvent(tradingEvent *domain.TradingEvent) {
	panic("unimplemented")
}

// SendTradeSequenceMessages implements domain.TradingSequencerRepo.
func (*tradingSequencerRepo) SendTradeSequenceMessages(*domain.TradingEvent) {
	panic("unimplemented")
}

// SubscribeTradeSequenceMessage implements domain.TradingSequencerRepo.
func (*tradingSequencerRepo) SubscribeTradeSequenceMessage(notify func(*domain.TradingEvent)) {
	panic("unimplemented")
}
