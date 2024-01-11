package sequencer

import "github.com/superj80820/system-design/domain"

type tradingSequencerRepo struct{}

func CreateTradingSequencerRepo() domain.TradingSequencerRepo {
	return &tradingSequencerRepo{}
}

// GetMaxSequenceID implements domain.TradingSequencerRepo.
func (*tradingSequencerRepo) GetMaxSequenceID() uint64 {
	panic("unimplemented")
}

// SaveEvent implements domain.TradingSequencerRepo.
func (*tradingSequencerRepo) SaveEvent(tradingEvent *domain.TradingEvent) {
	panic("unimplemented")
}
