package persistent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestList(t *testing.T) {
	t.Parallel()
	t.Run("NewList", func(t *testing.T) {
		t.Parallel()
		l := NewList[int]()
		assert.Equal(t, 0, l.Len())
	})

	t.Run("Append and Get", func(t *testing.T) {
		t.Parallel()
		l := NewList[int]()
		l2 := l.Append(1).Append(2)
		assert.Equal(t, 0, l.Len())
		assert.Equal(t, 2, l2.Len())
		assert.Equal(t, 1, l2.Get(0))
		assert.Equal(t, 2, l2.Get(1))
	})

	t.Run("Set", func(t *testing.T) {
		t.Parallel()
		l := NewList[int]().Append(1).Append(2)
		l2 := l.Set(0, 10)
		assert.Equal(t, 1, l.Get(0))
		assert.Equal(t, 10, l2.Get(0))
	})

	t.Run("Nil handling (Nil punning)", func(t *testing.T) {
		t.Parallel()
		var l List[int]
		assert.Equal(t, 0, l.Len())
		l2 := l.Append(1)
		assert.Equal(t, 1, l2.Len())
		assert.Equal(t, 1, l2.Get(0))
	})

	t.Run("Panic on out of bounds Get", func(t *testing.T) {
		t.Parallel()
		var l List[int]
		assert.Panics(t, func() { l.Get(0) })
	})

	t.Run("Panic on out of bounds Set", func(t *testing.T) {
		t.Parallel()
		var l List[int]
		assert.Panics(t, func() { l.Set(0, 1) })
	})

	t.Run("Slice", func(t *testing.T) {
		t.Parallel()
		l := NewList[int]().Append(10).Append(20).Append(30).Append(40)

		s := Slice(l, 1, 3)
		assert.Equal(t, 2, s.Len())
		assert.Equal(t, 20, s.Get(0))
		assert.Equal(t, 30, s.Get(1))

		empty := Slice(l, 2, 2)
		assert.Equal(t, 0, empty.Len())

		full := Slice(l, 0, l.Len())
		assert.Equal(t, 4, full.Len())
	})

	t.Run("Slice on nil list", func(t *testing.T) {
		t.Parallel()
		var l List[int]
		s := Slice(l, 0, 0)
		assert.Equal(t, 0, s.Len())
	})

	t.Run("Slice on nil list panics on non-zero bounds", func(t *testing.T) {
		t.Parallel()
		var l List[int]
		assert.Panics(t, func() { Slice(l, 0, 1) })
	})

	t.Run("AllList iterator", func(t *testing.T) {
		t.Parallel()
		l := NewList[int]().Append(1).Append(2).Append(3)
		var sum int
		for _, v := range l.All() {
			sum += v
		}
		assert.Equal(t, 6, sum)

		// Nil list iteration
		var nl List[int]
		for _, v := range nl.All() {
			assert.Fail(t, "Nil list should not yield values", "got %v", v)
		}

		// Early break
		count := 0
		for range l.All() {
			count++

			break
		}
		assert.Equal(t, 1, count)
	})

	t.Run("Values iterator", func(t *testing.T) {
		t.Parallel()
		l := NewList[int]().Append(10).Append(20).Append(30)
		var sum int
		for v := range l.Values() {
			sum += v
		}
		assert.Equal(t, 60, sum)

		// Nil list iteration
		var nl List[int]
		for v := range nl.Values() {
			assert.Fail(t, "Nil list should not yield values", "got %v", v)
		}

		// Early break
		count := 0
		for range l.Values() {
			count++

			break
		}
		assert.Equal(t, 1, count)
	})
}

func BenchmarkListAppend(b *testing.B) {
	l := NewList[int]()
	b.ResetTimer()
	for i := range b.N {
		l = l.Append(i)
	}
}

func BenchmarkListGet(b *testing.B) {
	l := NewList[int]()
	for i := range 1000 {
		l = l.Append(i)
	}
	b.ResetTimer()
	for i := range b.N {
		_ = l.Get(i % 1000)
	}
}
