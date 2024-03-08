package sequencer

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

type tradingSequencerUseCase struct {
	sequencerRepo domain.SequencerRepo
}

func CreateTradingSequencerUseCase(sequencerRepo domain.SequencerRepo) domain.SequenceTradingUseCase {
	return &tradingSequencerUseCase{
		sequencerRepo: sequencerRepo,
	}
}

func (t *tradingSequencerUseCase) CheckEventSequence(sequenceID int, lastSequenceID int) error {
	if err := t.sequencerRepo.CheckEventSequence(sequenceID, lastSequenceID); err != nil {
		return errors.Wrap(err, "check event sequence failed")
	}
	return nil
}

func (t *tradingSequencerUseCase) Continue() error {
	if err := t.sequencerRepo.Continue(); err != nil {
		return errors.Wrap(err, "continue failed")
	}
	return nil
}

func (t *tradingSequencerUseCase) GetSequenceID() uint64 {
	return t.sequencerRepo.GetSequenceID()
}

func (t *tradingSequencerUseCase) Pause() error {
	if err := t.sequencerRepo.Pause(); err != nil {
		return errors.Wrap(err, "pause failed")
	}
	return nil
}

func (t *tradingSequencerUseCase) ProduceSequenceMessages(ctx context.Context, tradingEvent *domain.TradingEvent) error {
	marshalData, err := json.Marshal(*tradingEvent)
	if err != nil {
		return errors.Wrap(err, "marshal failed")
	}
	if err := t.sequencerRepo.ProduceSequenceMessages(ctx, &domain.SequencerEvent{
		ReferenceID: tradingEvent.ReferenceID,
		Data:        string(marshalData),
		CreatedAt:   tradingEvent.CreatedAt,
	}); err != nil {
		return errors.Wrap(err, "produce sequence messages failed")
	}
	return nil
}

func (t *tradingSequencerUseCase) RecoverEvents(offsetSequenceID int, processFn func([]*domain.TradingEvent) error) error {
	if err := t.sequencerRepo.RecoverEvents(offsetSequenceID, func(sequencerEvents []*domain.SequencerEvent) error {
		tradingEvents := make([]*domain.TradingEvent, len(sequencerEvents))
		for idx, sequencerEvent := range sequencerEvents {
			var tradingEvent domain.TradingEvent
			if err := json.Unmarshal([]byte(sequencerEvent.Data), &tradingEvent); err != nil {
				return errors.Wrap(err, "unmarshal failed")
			}
			tradingEvent.ReferenceID = sequencerEvent.ReferenceID
			tradingEvent.SequenceID = sequencerEvent.SequenceID
			tradingEvents[idx] = &tradingEvent
		}
		if err := processFn(tradingEvents); err != nil {
			return errors.Wrap(err, "process failed")
		}
		return nil
	}); err != nil {
		return errors.Wrap(err, "recover events failed")
	}
	return nil
}

func (t *tradingSequencerUseCase) SaveWithFilterEvents(tradingEvents []*domain.TradingEvent, commitFn func() error) ([]*domain.TradingEvent, error) {
	sequenceEvents := make([]*domain.SequencerEvent, len(tradingEvents))
	for idx, tradingEvent := range tradingEvents {
		marshalData, err := json.Marshal(tradingEvent)
		if err != nil {
			return nil, errors.Wrap(err, "marshal failed")
		}
		sequenceEvents[idx] = &domain.SequencerEvent{
			ReferenceID: tradingEvent.ReferenceID,
			SequenceID:  tradingEvent.SequenceID,
			Data:        string(marshalData),
			CreatedAt:   tradingEvent.CreatedAt,
		}
	}
	sequenceEvents, err := t.sequencerRepo.SaveWithFilterEvents(sequenceEvents, commitFn)
	if err != nil {
		return nil, errors.Wrap(err, "save with filter events failed")
	}
	tradingEvents = make([]*domain.TradingEvent, len(sequenceEvents))
	for idx, sequenceEvent := range sequenceEvents {
		var tradingEvent domain.TradingEvent
		if err := json.Unmarshal([]byte(sequenceEvent.Data), &tradingEvent); err != nil {
			return nil, errors.Wrap(err, "unmarshal failed")
		}
		tradingEvent.ReferenceID = sequenceEvent.ReferenceID
		tradingEvent.SequenceID = sequenceEvent.SequenceID
		tradingEvents[idx] = &tradingEvent
	}
	return tradingEvents, nil
}

func (t *tradingSequencerUseCase) Shutdown() {
	t.sequencerRepo.Shutdown()
}

func (t *tradingSequencerUseCase) ConsumeSequenceMessages(notify func(events []*domain.TradingEvent, commitFn func() error)) {
	t.sequencerRepo.ConsumeSequenceMessages(func(sequencerEvents []*domain.SequencerEvent, commitFn func() error) {
		tradingEvents := make([]*domain.TradingEvent, len(sequencerEvents))
		for idx, sequencerEvent := range sequencerEvents {
			var tradingEvent domain.TradingEvent
			if err := json.Unmarshal([]byte(sequencerEvent.Data), &tradingEvent); err != nil {
				panic(errors.Wrap(err, "unmarshal failed")) // TODO
			}
			tradingEvent.ReferenceID = sequencerEvent.ReferenceID
			tradingEvent.SequenceID = sequencerEvent.SequenceID
			tradingEvents[idx] = &tradingEvent
		}
		notify(tradingEvents, commitFn)
	})
}
