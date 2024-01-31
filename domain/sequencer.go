package domain

import (
	"context"
	"time"
)

type SequencerRepo[T any] interface {
	GetMaxSequenceID() (uint64, error)
	ResetSequence() error
	GetCurrentSequenceID() uint64
	GenerateNextSequenceID() uint64
	SubscribeTradeSequenceMessage(notify func(any *T, commitFn func() error))
	SendTradeSequenceMessages(context.Context, *T) error
	SaveEvent(*SequencerEvent) error
	SaveEvents([]*SequencerEvent) error
	GetFilterEventsMap([]*SequencerEvent) (map[int64]bool, error)
	Pause() error
	Continue() error
	Shutdown()
}

type SequencerEvent struct {
	ReferenceID int64
	SequenceID  int64
	PreviousID  int64
	Data        string
	CreatedAt   time.Time
}
