package util

import "github.com/pkg/errors"

type RingBuffer[T any] struct {
	items  []*T
	isFull bool
	cap    int
	head   int
	tail   int
}

func CreateRingBuffer[T any](cap int) *RingBuffer[T] {
	return &RingBuffer[T]{
		items: make([]*T, cap),
		cap:   cap,
	}
}

func (r *RingBuffer[T]) IsFull() bool {
	return r.isFull
}

func (r *RingBuffer[T]) IsEmpty() bool {
	return r.head == r.tail && !r.isFull
}

func (r *RingBuffer[T]) Enqueue(item *T) {
	r.items[r.tail] = item
	r.tail = (r.tail + 1) % r.cap
	r.isFull = r.head == r.tail
}

func (r *RingBuffer[T]) Dequeue() (*T, error) {
	if r.IsEmpty() {
		return nil, errors.New("is empty")
	}

	item := r.items[r.head]
	r.head = (r.head + 1) % r.cap
	r.isFull = false

	return item, nil
}

func (r *RingBuffer[T]) GetCap() int {
	return r.cap
}
