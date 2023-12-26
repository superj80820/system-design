package memory

import (
	"sync/atomic"

	"github.com/superj80820/system-design/domain"
)

type memoryConfigRepo[T any] struct {
	config atomic.Value
}

func CreateMemoryConfigRepo[T any](value T) domain.ConfigRepo[T] {
	var atomicConfig atomic.Value
	atomicConfig.Store(value)
	return &memoryConfigRepo[T]{
		config: atomicConfig,
	}
}

func (m *memoryConfigRepo[T]) Get() T {
	return m.config.Load().(T)
}

func (m *memoryConfigRepo[T]) Set(value T) bool {
	m.config.Store(value)
	return true
}

func (m *memoryConfigRepo[T]) CAS(old, new T) bool {
	return m.config.CompareAndSwap(old, new)
}
