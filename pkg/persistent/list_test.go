package persistent

import (
	"testing"
)

func TestList(t *testing.T) {
	t.Parallel()
	t.Run("NewList", func(t *testing.T) {
		t.Parallel()
		l := NewList[int]()
		if l.Len() != 0 {
			t.Errorf("Expected len 0, got %d", l.Len())
		}
	})

	t.Run("Append and Get", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic on out of bounds Get, but did not")
			}
		}()
		var l List[int]
		l.Get(0)
	})

	t.Run("Panic on out of bounds Set", func(t *testing.T) {
		t.Parallel()
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic on out of bounds Set, but did not")
			}
		}()
		var l List[int]
		l.Set(0, 1)
	})

	t.Run("Slice", func(t *testing.T) {
		t.Parallel()
		l := NewList[int]().Append(10).Append(20).Append(30).Append(40)

		s := Slice(l, 1, 3)
		if s.Len() != 2 {
			t.Errorf("expected len 2, got %d", s.Len())
		}
		if s.Get(0) != 20 || s.Get(1) != 30 {
			t.Errorf("expected [20, 30], got [%d, %d]", s.Get(0), s.Get(1))
		}

		empty := Slice(l, 2, 2)
		if empty.Len() != 0 {
			t.Errorf("expected empty slice, got len %d", empty.Len())
		}

		full := Slice(l, 0, l.Len())
		if full.Len() != 4 {
			t.Errorf("expected len 4, got %d", full.Len())
		}
	})

	t.Run("Slice on nil list", func(t *testing.T) {
		t.Parallel()
		var l List[int]
		s := Slice(l, 0, 0)
		if s.Len() != 0 {
			t.Errorf("expected empty slice, got len %d", s.Len())
		}
	})

	t.Run("Slice on nil list panics on non-zero bounds", func(t *testing.T) {
		t.Parallel()
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic on out-of-range slice of nil list")
			}
		}()
		var l List[int]
		Slice(l, 0, 1)
	})

	t.Run("AllList iterator", func(t *testing.T) {
		t.Parallel()
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

	t.Run("Values iterator", func(t *testing.T) {
		t.Parallel()
		l := NewList[int]().Append(10).Append(20).Append(30)
		var sum int
		for v := range l.Values() {
			sum += v
		}
		if sum != 60 {
			t.Errorf("Expected sum 60, got %d", sum)
		}

		// Nil list iteration
		var nl List[int]
		for v := range nl.Values() {
			t.Errorf("Nil list should not yield values, got %v", v)
		}

		// Early break
		count := 0
		for range l.Values() {
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
