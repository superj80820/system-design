package util

import (
	"sync"
)

type GenericSyncMap[K comparable, V any] struct {
	sync.Map
}

func (s *GenericSyncMap[K, V]) Range(f func(key K, value V) bool) {
	s.Map.Range(func(key, value any) bool {
		return f(key.(K), value.(V))
	})
}

func (s *GenericSyncMap[K, V]) Store(key K, value V) {
	s.Map.Store(key, value)
}

func (s *GenericSyncMap[K, V]) Delete(key K) {
	s.Map.Delete(key)
}

func (s *GenericSyncMap[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	v, loaded := s.Map.LoadAndDelete(key)
	if !loaded {
		return value, loaded
	}
	return v.(V), loaded
}

func (s *GenericSyncMap[K, V]) Load(key K) (value V, ok bool) {
	v, ok := s.Map.Load(key)
	if !ok {
		return value, ok
	}
	return v.(V), ok
}
