package persistent

import (
	"github.com/benbjohnson/immutable"
	"iter"
)

type List[T any] struct {
	inner *immutable.List[T]
}

func (l List[T]) Len() int {
	if l.inner == nil {
		return 0
	}
	return l.inner.Len()
}

func NewList[T any]() List[T] {
	return List[T]{inner: immutable.NewList[T]()}
}

func (l List[T]) Append(v T) List[T] {
	if l.inner == nil {
		return List[T]{inner: immutable.NewList[T]().Append(v)}
	}
	return List[T]{inner: l.inner.Append(v)}
}

func (l List[T]) Get(i int) T {
	if l.inner == nil {
		panic("index out of bounds (nil list)")
	}
	return l.inner.Get(i)
}

func (l List[T]) Lookup(i int) (T, bool) {
	if l.inner == nil || i < 0 || i >= l.Len() {
		var zero T
		return zero, false
	}
	return l.Get(i), true
}

func (l List[T]) Set(i int, v T) List[T] {
	if l.inner == nil {
		panic("index out of bounds (nil list)")
	}
	return List[T]{inner: l.inner.Set(i, v)}
}

func (l List[T]) All() iter.Seq[T] {
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
