package readevent

import (
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

type eventSource[T any] struct {
	eventRepo domain.EventSourceRepo[T]
}

func CreateReadEvent[T any](eventRepo domain.EventSourceRepo[T]) domain.EventSourceService[T] {
	return &eventSource[T]{
		eventRepo: eventRepo,
	}
}

func (r *eventSource[T]) GetEvent(key string) (chan T, error) {
	readEventCh, err := r.eventRepo.ReadEvent(key)
	if err != nil {
		return nil, errors.Wrap(err, "get read event channel failed")
	}
	return readEventCh, nil
}
