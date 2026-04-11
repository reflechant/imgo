package persistent

import (
	"github.com/benbjohnson/immutable"
	"iter"
)

type Map[K comparable, V any] struct {
	inner *immutable.Map[K, V]
}

func (m Map[K, V]) Len() int {
	if m.inner == nil {
		return 0
	}
	return m.inner.Len()
}

func NewMap[K comparable, V any]() Map[K, V] {
	return Map[K, V]{inner: immutable.NewMap[K, V](nil)}
}

func (m Map[K, V]) Set(k K, v V) Map[K, V] {
	if m.inner == nil {
		return Map[K, V]{inner: immutable.NewMap[K, V](nil).Set(k, v)}
	}
	return Map[K, V]{inner: m.inner.Set(k, v)}
}

// Lookup returns the value and a boolean indicating if it exists (2-value return).
func (m Map[K, V]) Lookup(k K) (V, bool) {
	if m.inner == nil {
		var zero V
		return zero, false
	}
	return m.inner.Get(k)
}

// Get returns only the value, or zero value if not found (1-value return).
func (m Map[K, V]) Get(k K) V {
	v, _ := m.Lookup(k)
	return v
}

func (m Map[K, V]) Delete(k K) Map[K, V] {
	if m.inner == nil {
		return m
	}
	return Map[K, V]{inner: m.inner.Delete(k)}
}

func (m Map[K, V]) Update(k K, fn func(V) V) Map[K, V] {
	v := m.Get(k)
	return m.Set(k, fn(v))
}

func (m Map[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if m.inner == nil {
			return
		}
		it := m.inner.Iterator()
		for !it.Done() {
			k, v, _ := it.Next()
			if !yield(k, v) {
				return
			}
		}
	}
}

// SetIn implements "assoc-in" logic for maps.
func (m Map[K, V]) SetIn(path []K, value V) Map[K, V] {
	if len(path) == 0 {
		return m
	}
	if len(path) == 1 {
		return m.Set(path[0], value)
	}

	var subMap Map[K, V]
	if val, ok := m.Lookup(path[0]); ok {
		if sm, ok := any(val).(Map[K, V]); ok {
			subMap = sm
		}
	}
	if subMap.inner == nil {
		subMap = NewMap[K, V]()
	}

	return m.Set(path[0], any(subMap.SetIn(path[1:], value)).(V))
}

// UpdateIn implements "update-in" logic for maps.
func (m Map[K, V]) UpdateIn(path []K, fn func(V) V) Map[K, V] {
	if len(path) == 0 {
		return m
	}
	if len(path) == 1 {
		val := m.Get(path[0])
		return m.Set(path[0], fn(val))
	}

	var subMap Map[K, V]
	if val, ok := m.Lookup(path[0]); ok {
		if sm, ok := any(val).(Map[K, V]); ok {
			subMap = sm
		}
	}
	if subMap.inner == nil {
		subMap = NewMap[K, V]()
	}

	return m.Set(path[0], any(subMap.UpdateIn(path[1:], fn)).(V))
}
