package util

import "github.com/pkg/errors"

type RingBuffer[T any] struct {
	items []*T
	size  int
	cap   int
	head  int
	tail  int
}

func CreateRingBuffer[T any](cap int) *RingBuffer[T] {
	return &RingBuffer[T]{
		items: make([]*T, cap),
		cap:   cap,
	}
}

func (r *RingBuffer[T]) IsFull() bool {
	return r.size == r.cap
}

func (r *RingBuffer[T]) IsEmpty() bool {
	return r.size == 0
}

func (r *RingBuffer[T]) Enqueue(item *T) {
	r.items[r.tail] = item
	r.tail = (r.tail + 1) % r.cap
	if !r.IsFull() {
		r.size++
	}
}

func (r *RingBuffer[T]) Dequeue() (*T, error) {
	if r.IsEmpty() {
		return nil, errors.New("is empty")
	}

	item := r.items[r.head]
	r.head = (r.head + 1) % r.cap
	r.size--

	return item, nil
}

func (r *RingBuffer[T]) GetSize() int {
	return r.size
}
