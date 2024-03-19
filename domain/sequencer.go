package domain

import (
	"context"
	"time"
)

type SequencerRepo interface {
	GetMaxSequenceID() (uint64, error)
	GetSequenceID() uint64
	SetSequenceID(uint64)

	ConsumeSequenceMessages(notify func(events []*SequencerEvent, commitFn func() error))
	ProduceSequenceMessages(context.Context, *SequencerEvent) error

	SaveEvents([]*SequencerEvent) error
	FilterEvents([]*SequencerEvent) ([]*SequencerEvent, error)

	GetHistoryEvents(offsetSequenceID, page, limit int) (sequencerEvents []*SequencerEvent, isEnd bool, err error)
	GetReferenceIDFilterMap([]*SequencerEvent) (map[int]bool, error)

	CheckEventSequence(sequenceID, lastSequenceID int) error
	RecoverEvents(offsetSequenceID int, processFn func([]*SequencerEvent) error) error
	SequenceAndSave(events []*SequencerEvent, commitFn func() error) ([]*SequencerEvent, error)

	Pause() error
	Continue() error
	Shutdown()
}

type SequencerEvent struct {
	ReferenceID int
	SequenceID  int
	Data        string
	CreatedAt   time.Time
}
