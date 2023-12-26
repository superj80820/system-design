package domain

import "time"

type EventSourceRepo[T any] interface {
	AddEvent(event T)
	ReadEvent(key string) (chan T, error)
}

type TicketPlusEvent struct {
	Key         string
	Name        string
	Status      TicketPlusResponseStatus
	SortedIndex int
	Time        time.Time
}

type EventSourceService[T any] interface {
	GetEvent(key string) (chan T, error)
}
