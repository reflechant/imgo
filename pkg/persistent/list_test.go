package persistent

import (
	"testing"
)

func TestList(t *testing.T) {
	t.Run("NewList", func(t *testing.T) {
		l := NewList[int]()
		if l.Len() != 0 {
			t.Errorf("Expected len 0, got %d", l.Len())
		}
	})

	t.Run("Append and Get", func(t *testing.T) {
		l := NewList[int]()
		l2 := l.Append(1).Append(2)
		if l.Len() != 0 {
			t.Errorf("Original list should be empty, got %d", l.Len())
		}
		if l2.Len() != 2 {
			t.Errorf("New list should have len 2, got %d", l2.Len())
		}
		if l2.Get(0) != 1 {
			t.Errorf("Expected 1 at index 0, got %v", l2.Get(0))
		}
		if l2.Get(1) != 2 {
			t.Errorf("Expected 2 at index 1, got %v", l2.Get(1))
		}
	})

	t.Run("Set", func(t *testing.T) {
		l := NewList[int]().Append(1).Append(2)
		l2 := l.Set(0, 10)
		if l.Get(0) != 1 {
			t.Errorf("Original list should have 1, got %v", l.Get(0))
		}
		if l2.Get(0) != 10 {
			t.Errorf("New list should have 10, got %v", l2.Get(0))
		}
	})

	t.Run("Nil handling (Nil punning)", func(t *testing.T) {
		var l List[int]
		if l.Len() != 0 {
			t.Errorf("Nil list should have len 0, got %d", l.Len())
		}
		l2 := l.Append(1)
		if l2.Len() != 1 {
			t.Errorf("Appended nil list should have len 1, got %d", l2.Len())
		}
		if l2.Get(0) != 1 {
			t.Errorf("Expected 1, got %v", l2.Get(0))
		}
	})

	t.Run("Panic on out of bounds Get", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic on out of bounds Get, but did not")
			}
		}()
		var l List[int]
		l.Get(0)
	})

	t.Run("Panic on out of bounds Set", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic on out of bounds Set, but did not")
			}
		}()
		var l List[int]
		l.Set(0, 1)
	})

	t.Run("AllList iterator", func(t *testing.T) {
		l := NewList[int]().Append(1).Append(2).Append(3)
		var sum int
		for _, v := range l.All() {
			sum += v
		}
		if sum != 6 {
			t.Errorf("Expected sum 6, got %d", sum)
		}

		// Nil list iteration
		var nl List[int]
		for _, v := range nl.All() {
			t.Errorf("Nil list should not yield values, got %v", v)
		}

		// Early break
		count := 0
		for range l.All() {
			count++
			break
		}
		if count != 1 {
			t.Errorf("Expected count 1 after early break, got %d", count)
		}
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
