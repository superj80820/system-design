package sequencer

import (
	"sync/atomic"
	"time"

	"github.com/superj80820/system-design/domain"
)

type tradingSequencerUseCase struct {
	sequence             *atomic.Uint64
	tradingSequencerRepo domain.TradingSequencerRepo
	tradingRepo          domain.TradingRepo
	lastTimestamp        time.Time
}

func CreateTradingUseCase(tradingSequencerRepo domain.TradingSequencerRepo, tradingRepo domain.TradingRepo) domain.TradingSequencerUseCase {
	var sequence atomic.Uint64
	sequence.Add(tradingSequencerRepo.GetMaxSequenceID())

	return &tradingSequencerUseCase{
		sequence:             &sequence,
		tradingSequencerRepo: tradingSequencerRepo,
		tradingRepo:          tradingRepo,
	}
}

func (t *tradingSequencerUseCase) ProcessMessages(tradingEvent *domain.TradingEvent) {
	t.SequenceMessages(tradingEvent)
	t.tradingSequencerRepo.SaveEvent(tradingEvent)
	t.SendMessages(tradingEvent)
}

// SendMessages implements domain.TradingSequencerUseCase.
func (t *tradingSequencerUseCase) SendMessages(tradingEvent *domain.TradingEvent) {
	t.tradingRepo.SendTradeMessages(tradingEvent)
}

// TODO implements
func (t *tradingSequencerUseCase) SequenceMessages(tradingEvent *domain.TradingEvent) {
	timeNow := time.Now()
	if timeNow.Before(t.lastTimestamp) {
		panic("TODO")
	} else {
		t.lastTimestamp = timeNow
	}
	previousID := t.sequence.Load()
	currentID := t.sequence.Add(1)
	tradingEvent.PreviousID = int(previousID) // TODO: test type save
	tradingEvent.SequenceID = int(currentID)  // TODO: test type save
	tradingEvent.CreatedAt = t.lastTimestamp
}
