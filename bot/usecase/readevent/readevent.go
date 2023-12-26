package readevent

import (
	"sort"
	"sync"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

type readEvent struct {
	eventRepo domain.EventSourceRepo[*domain.TicketPlusEvent]
}

func CreateReadEvent(eventRepo domain.EventSourceRepo[*domain.TicketPlusEvent]) domain.ReadEventService {
	return &readEvent{
		eventRepo: eventRepo,
	}
}

func (r *readEvent) GetTicketPlusStatus(key, eventKey string) (func() []*domain.ReadEventStatus, error) {
	readEventCh, err := r.eventRepo.ReadEvent(key)
	if err != nil {
		return nil, errors.Wrap(err, "get read event channel failed")
	}

	lock := new(sync.RWMutex)
	eventStatusMap := make(map[int]*domain.ReadEventStatus)

	go func() {
		for event := range readEventCh {
			lock.Lock()

			if event.Key != eventKey {
				continue
			}

			if _, ok := eventStatusMap[event.SortedIndex]; !ok {
				eventStatusMap[event.SortedIndex] = &domain.ReadEventStatus{
					Key:         event.Key,
					Name:        event.Name,
					SortedIndex: event.SortedIndex,
				}
			}
			if len(eventStatusMap[event.SortedIndex].Count) <= int(event.Status) { // TODO: len?
				newCount := make([]int, event.Status+1)
				copy(newCount, eventStatusMap[event.SortedIndex].Count)
				eventStatusMap[event.SortedIndex].Count = newCount
			}
			eventStatusMap[event.SortedIndex].Count[event.Status]++
			if event.Status == domain.TicketPlusResponseStatusOK ||
				event.Status == domain.TicketPlusResponseStatusReserved {
				eventStatusMap[event.SortedIndex].Done = true
			}
			lock.Unlock()
		}
	}()

	return func() []*domain.ReadEventStatus {
		lock.RLock()
		defer lock.RUnlock()

		eventStatuses := make([]*domain.ReadEventStatus, 0, len(eventStatusMap))
		for _, eventStatus := range eventStatusMap {
			eventStatuses = append(eventStatuses, eventStatus)
		}
		sort.Slice(eventStatuses, func(i, j int) bool { // TODO: use heap
			return eventStatuses[i].SortedIndex < eventStatuses[j].SortedIndex
		})
		return eventStatuses
	}, nil
}
