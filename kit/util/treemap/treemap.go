package treemap

import (
	"github.com/emirpasic/gods/maps/treemap"
)

type Comparator[K comparable] func(a, b K) int

type GenericTreeMap[K comparable, V any] struct {
	*treemap.Map
}

func NewWith[K comparable, V any](comparator Comparator[K]) *GenericTreeMap[K, V] {
	return &GenericTreeMap[K, V]{
		treemap.NewWith(func(a, b interface{}) int {
			return comparator(a.(K), b.(K))
		}),
	}
}

func (g *GenericTreeMap[K, V]) Max() (K, V) {
	key, value := g.Map.Max()
	return key.(K), value.(V)
}

func (g *GenericTreeMap[K, V]) Min() (K, V) {
	key, value := g.Map.Min()
	return key.(K), value.(V)
}

func (g *GenericTreeMap[K, V]) Remove(key K) {
	g.Map.Remove(key)
}

func (g *GenericTreeMap[K, V]) Put(key K, value V) {
	g.Map.Put(key, value)
}

func (g *GenericTreeMap[K, V]) Get(key K) (V, bool) {
	value, found := g.Map.Get(key)
	if !found {
		var noop V
		return noop, found
	}
	return value.(V), found
}

func (g *GenericTreeMap[K, V]) Each(f func(key K, value V)) {
	g.Map.Each(func(key, value interface{}) {
		f(key.(K), value.(V))
	})
}
