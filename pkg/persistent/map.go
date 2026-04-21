package persistent

import (
	"iter"

	"github.com/benbjohnson/immutable"
)

// Map is an immutable collection of key-value pairs backed by a hash array mapped trie (HAMT).
type Map[K comparable, V any] struct {
	inner *immutable.Map[K, V]
}

// Len returns the number of key-value pairs in the map.
func (m Map[K, V]) Len() int {
	if m.inner == nil {
		return 0
	}
	return m.inner.Len()
}

// NewMap returns a new, empty Map.
func NewMap[K comparable, V any]() Map[K, V] {
	return Map[K, V]{inner: immutable.NewMap[K, V](nil)}
}

// Set returns a new map with the value for key k set to v.
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

// Delete returns a new map with the entry for key k removed.
func (m Map[K, V]) Delete(k K) Map[K, V] {
	if m.inner == nil {
		return m
	}
	return Map[K, V]{inner: m.inner.Delete(k)}
}

// Update returns a new map with the value for key k replaced by the result of fn.
func (m Map[K, V]) Update(k K, fn func(V) V) Map[K, V] {
	v := m.Get(k)
	return m.Set(k, fn(v))
}

// All returns an iterator over the key-value pairs of the map.
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

// Keys returns an iterator over the keys of the map.
func (m Map[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		if m.inner == nil {
			return
		}
		it := m.inner.Iterator()
		for !it.Done() {
			k, _, _ := it.Next()
			if !yield(k) {
				return
			}
		}
	}
}

// Values returns an iterator over the values of the map.
func (m Map[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		if m.inner == nil {
			return
		}
		it := m.inner.Iterator()
		for !it.Done() {
			_, v, _ := it.Next()
			if !yield(v) {
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

	newSub, _ := any(subMap.SetIn(path[1:], value)).(V)
	return m.Set(path[0], newSub)
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

	newSub, _ := any(subMap.UpdateIn(path[1:], fn)).(V)
	return m.Set(path[0], newSub)
}

// DeleteIn implements "dissoc-in" logic for maps.
func (m Map[K, V]) DeleteIn(path []K) Map[K, V] {
	if len(path) == 0 {
		return m
	}
	if len(path) == 1 {
		return m.Delete(path[0])
	}

	var subMap Map[K, V]
	if val, ok := m.Lookup(path[0]); ok {
		if sm, ok := any(val).(Map[K, V]); ok {
			subMap = sm
		} else {
			// Subpath is not a map, can't descend to delete.
			return m
		}
	} else {
		// Key doesn't exist.
		return m
	}

	newSub, _ := any(subMap.DeleteIn(path[1:])).(V)
	return m.Set(path[0], newSub)
}
