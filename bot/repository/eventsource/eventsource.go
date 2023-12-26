package eventsource

import (
	"sync"

	"github.com/superj80820/system-design/domain"
)

type eventSource[T any] struct {
	eventCh    chan T
	subscriber sync.Map
}

func CreateEventSource[T any]() domain.EventSourceRepo[T] {
	eventCh := make(chan T, 10000)
	e := eventSource[T]{
		eventCh: eventCh,
	}
	go func() {
		for event := range eventCh {
			e.subscriber.Range(func(key, value any) bool {
				ch := value.(chan T)
				ch <- event
				return true
			})
		}
	}()
	return &e
}

func (e *eventSource[T]) AddEvent(event T) {
	e.eventCh <- event
}

func (e *eventSource[T]) ReadEvent(key string) (chan T, error) {
	readCh := make(chan T, 1000)
	e.subscriber.Store(key, readCh)
	return readCh, nil
}
