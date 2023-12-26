package memory

import (
	"github.com/superj80820/system-design/domain"
)

type memoryUseCase[T any] struct {
	memoryConfigRepo domain.ConfigRepo[T]
}

func CreateMemoryUseCase[T any](memoryConfigRepo domain.ConfigRepo[T]) domain.ConfigService[T] {
	return &memoryUseCase[T]{
		memoryConfigRepo: memoryConfigRepo,
	}
}

func (m *memoryUseCase[T]) Get() T {
	return m.memoryConfigRepo.Get()
}

func (m *memoryUseCase[T]) Set(value T) bool {
	return m.memoryConfigRepo.Set(value)
}

func (m *memoryUseCase[T]) CAS(old, new T) bool {
	return m.memoryConfigRepo.CAS(old, new)
}
