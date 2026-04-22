// Package persistent provides immutable, high-performance data structures
// that back ImGo's functional semantics.
package persistent

import (
	"fmt"
	"iter"

	"github.com/benbjohnson/immutable"
)

// List is an immutable sequence of elements backed by a bitmapped vector trie.
type List[T any] struct {
	inner *immutable.List[T]
}

// Len returns the number of elements in the list.
func (l List[T]) Len() int {
	if l.inner == nil {
		return 0
	}

	return l.inner.Len()
}

// NewList returns a new, empty List.
func NewList[T any]() List[T] {
	return List[T]{inner: immutable.NewList[T]()}
}

// Append returns a new list with v added to the end.
func (l List[T]) Append(v T) List[T] {
	if l.inner == nil {
		return List[T]{inner: immutable.NewList[T]().Append(v)}
	}

	return List[T]{inner: l.inner.Append(v)}
}

// Get returns the element at index i. It panics if i is out of bounds.
func (l List[T]) Get(i int) T {
	if i < 0 || i >= l.Len() {
		panic(fmt.Sprintf("runtime error: index out of range [%d] with length %d", i, l.Len()))
	}

	return l.inner.Get(i)
}

// Set returns a new list with the element at index i replaced by v.
func (l List[T]) Set(i int, v T) List[T] {
	if i < 0 || i >= l.Len() {
		panic(fmt.Sprintf("runtime error: index out of range [%d] with length %d", i, l.Len()))
	}

	return List[T]{inner: l.inner.Set(i, v)}
}

// All returns an iterator over the index and values of the list.
func (l List[T]) All() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		if l.inner == nil {
			return
		}
		it := l.inner.Iterator()
		for !it.Done() {
			i, v := it.Next()
			if !yield(i, v) {
				return
			}
		}
	}
}

// Slice returns a sub-list spanning [low, high). It panics on out-of-range
// indices, matching Go slice semantics. A nil list with low == high == 0
// returns an empty list.
func Slice[T any](l List[T], low, high int) List[T] {
	if l.inner == nil {
		if low != 0 || high != 0 {
			panic(fmt.Sprintf("runtime error: slice bounds out of range [%d:%d] with length 0", low, high))
		}

		return NewList[T]()
	}

	return List[T]{inner: l.inner.Slice(low, high)}
}

// Values returns an iterator over the values of the list.
func (l List[T]) Values() iter.Seq[T] {
	return func(yield func(T) bool) {
		if l.inner == nil {
			return
		}
		it := l.inner.Iterator()
		for !it.Done() {
			_, v := it.Next()
			if !yield(v) {
				return
			}
		}
	}
}
