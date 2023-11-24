package util

import "sync"

type GenericSyncMap[K comparable, V any] struct {
	sync.Map
}

func (s *GenericSyncMap[K, V]) Range(f func(key K, value V) bool) {
	s.Map.Range(func(key, value any) bool {
		return f(key.(K), value.(V))
	})
}

type A struct{}

func (s *GenericSyncMap[K, V]) Store(key K, value V) {
	s.Map.Store(key, value)
}

func (s *GenericSyncMap[K, V]) Delete(key K) {
	s.Map.Delete(key)
}

func (s *GenericSyncMap[K, V]) Load(key K) (V, bool) {
	value, ok := s.Map.Load(key)
	return value.(V), ok
}
