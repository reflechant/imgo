package persistent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMap(t *testing.T) {
	t.Parallel()
	t.Run("NewMap", func(t *testing.T) {
		t.Parallel()
		m := NewMap[string, int]()
		assert.Equal(t, 0, m.Len())
	})

	t.Run("Set and Lookup/Get", func(t *testing.T) {
		t.Parallel()
		m := NewMap[string, int]()
		m2 := m.Set("a", 1).Set("b", 2)
		assert.Equal(t, 0, m.Len())
		assert.Equal(t, 2, m2.Len())

		v, ok := m2.Lookup("a")
		assert.True(t, ok)
		assert.Equal(t, 1, v)

		v = m2.Get("a")
		assert.Equal(t, 1, v)

		_, ok = m2.Lookup("c")
		assert.False(t, ok)
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		m := NewMap[string, int]().Set("a", 1).Set("b", 2)
		m2 := m.Delete("a")
		assert.Equal(t, 2, m.Len())
		assert.Equal(t, 1, m2.Len())

		_, ok := m2.Lookup("a")
		assert.False(t, ok)

		// Nil map delete
		var nm Map[string, int]
		nm2 := nm.Delete("a")
		assert.Equal(t, 0, nm2.Len())
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		m := NewMap[string, int]().Set("a", 1)
		m2 := m.Update("a", func(v int) int { return v + 10 })
		assert.Equal(t, 11, m2.Get("a"))
	})

	t.Run("Nil handling (Nil punning)", func(t *testing.T) {
		t.Parallel()
		var m Map[string, int]
		assert.Equal(t, 0, m.Len())

		// Test Lookup on nil map
		v, ok := m.Lookup("a")
		assert.False(t, ok)
		assert.Equal(t, 0, v)

		m2 := m.Set("a", 1)
		assert.Equal(t, 1, m2.Len())
		assert.Equal(t, 1, m2.Get("a"))
	})

	t.Run("All() iterator", func(t *testing.T) {
		t.Parallel()
		m := NewMap[string, int]().Set("a", 1).Set("b", 2)
		var sum int
		for _, v := range m.All() {
			sum += v
		}
		assert.Equal(t, 3, sum)

		// Nil map iteration
		var nm Map[string, int]
		for k, v := range nm.All() {
			assert.Fail(t, "Nil map should not yield values", "got %v: %v", k, v)
		}

		// Early break
		count := 0
		for range m.All() {
			count++

			break
		}
		assert.Equal(t, 1, count)
	})

	t.Run("Keys and Values iterators", func(t *testing.T) {
		t.Parallel()
		m := NewMap[string, int]().Set("a", 1).Set("b", 2)

		keys := make(map[string]bool)
		for k := range m.Keys() {
			keys[k] = true
		}
		assert.Len(t, keys, 2)
		assert.True(t, keys["a"])
		assert.True(t, keys["b"])

		sum := 0
		for v := range m.Values() {
			sum += v
		}
		assert.Equal(t, 3, sum)

		// Nil map iteration
		var nm Map[string, int]
		for range nm.Keys() {
			assert.Fail(t, "Nil map Keys() should not yield")
		}
		for range nm.Values() {
			assert.Fail(t, "Nil map Values() should not yield")
		}

		// Early break
		count := 0
		for range m.Keys() {
			count++

			break
		}
		assert.Equal(t, 1, count)

		count = 0
		for range m.Values() {
			count++

			break
		}
		assert.Equal(t, 1, count)
	})

	t.Run("SetIn / UpdateIn (Empty path)", func(t *testing.T) {
		t.Parallel()
		m := NewMap[string, int]().Set("a", 1)
		m2 := m.SetIn([]string{}, 10)
		assert.Equal(t, 1, m2.Len())
		assert.Equal(t, 1, m2.Get("a"))

		m3 := m.UpdateIn([]string{}, func(v int) int { return v + 1 })
		assert.Equal(t, 1, m3.Len())
		assert.Equal(t, 1, m3.Get("a"))
	})

	t.Run("SetIn / UpdateIn (Basic typed cases)", func(t *testing.T) {
		t.Parallel()
		m := NewMap[string, int]()
		m2 := m.SetIn([]string{"a"}, 10)
		assert.Equal(t, 10, m2.Get("a"))

		m3 := m2.UpdateIn([]string{"a"}, func(v int) int { return v * 2 })
		assert.Equal(t, 20, m3.Get("a"))
	})

	t.Run("SetIn / UpdateIn (Nested any cases)", func(t *testing.T) {
		t.Parallel()
		m := NewMap[string, any]()
		m2 := m.SetIn([]string{"a", "b"}, 42)

		val := m2.Get("a")
		subMap, ok := val.(Map[string, any])
		assert.True(t, ok)
		assert.Equal(t, 42, subMap.Get("b"))

		m3 := m2.UpdateIn([]string{"a", "b"}, func(v any) any {
			vInt, _ := v.(int)

			return vInt + 8
		})
		val = m3.Get("a")
		subMap, ok = val.(Map[string, any])
		assert.True(t, ok)
		assert.Equal(t, 50, subMap.Get("b"))
	})

	t.Run("SetIn (Intermediate not a map)", func(t *testing.T) {
		t.Parallel()
		m := NewMap[string, any]().Set("a", 123)
		m2 := m.SetIn([]string{"a", "b"}, 456)
		val := m2.Get("a")
		assert.IsType(t, Map[string, any]{}, val)
	})

	t.Run("UpdateIn (Intermediate not a map)", func(t *testing.T) {
		t.Parallel()
		m := NewMap[string, any]().Set("a", 123)
		m2 := m.UpdateIn([]string{"a", "b"}, func(v any) any { return v })
		val := m2.Get("a")
		assert.IsType(t, Map[string, any]{}, val)
	})

	t.Run("SetIn (Existing submap)", func(t *testing.T) {
		t.Parallel()
		m := NewMap[string, any]()
		m2 := m.SetIn([]string{"a", "b"}, 1)
		m3 := m2.SetIn([]string{"a", "c"}, 2)
		// m3["a"] should have both "b":1 and "c":2
		val := m3.Get("a")
		sub, ok := val.(Map[string, any])
		assert.True(t, ok)
		assert.Equal(t, 2, sub.Len())
	})

	t.Run("DeleteIn", func(t *testing.T) {
		t.Parallel()
		m := NewMap[string, any]().SetIn([]string{"a", "b", "c"}, 100)
		m2 := m.DeleteIn([]string{"a", "b", "c"})
		val := m2.Get("a")
		subA, _ := val.(Map[string, any])
		val = subA.Get("b")
		subB, _ := val.(Map[string, any])
		assert.Equal(t, 0, subB.Len())

		// Empty path
		m3 := m.DeleteIn([]string{})
		assert.Equal(t, 1, m3.Len())

		// Path not found
		m4 := m.DeleteIn([]string{"a", "x"})
		assert.Equal(t, 1, m4.Len())

		// Intermediate not a map
		m5 := NewMap[string, any]().Set("a", 123)
		m6 := m5.DeleteIn([]string{"a", "b"})
		assert.Equal(t, 123, m6.Get("a"))

		// Single element path
		m7 := m5.DeleteIn([]string{"a"})
		assert.Equal(t, 0, m7.Len())

		// Path not found (first key doesn't exist, path len > 1)
		m8 := m.DeleteIn([]string{"nonexistent", "b"})
		assert.Equal(t, 1, m8.Len())
	})

	t.Run("Deep access with list inside map", func(t *testing.T) {
		t.Parallel()
		l := NewList[int]().Append(10).Append(20)
		m := NewMap[string, any]().Set("a", l)

		// This should work (m["a"].Get(0))
		val := m.Get("a")
		l2, ok := val.(List[int])
		assert.True(t, ok)
		assert.Equal(t, 10, l2.Get(0))

		// Test missing map key returning empty list
		m2 := NewMap[string, List[int]]()
		l3 := m2.Get("nonexistent")
		assert.Equal(t, 0, l3.Len())

		// If the user does m2["nonexistent"][0], it will be m2.Get("nonexistent").Get(0)
		// which should panic because Get(0) on empty list panics now.
		assert.Panics(t, func() { l3.Get(0) })
	})
}

func BenchmarkMapInsert(b *testing.B) {
	m := NewMap[int, int]()
	b.ResetTimer()
	for i := range b.N {
		m = m.Set(i, i)
	}
}

func BenchmarkMapLookup(b *testing.B) {
	m := NewMap[int, int]()
	for i := range 1000 {
		m = m.Set(i, i)
	}
	b.ResetTimer()
	for i := range b.N {
		_ = m.Get(i % 1000)
	}
}

func BenchmarkMapDelete(b *testing.B) {
	m := NewMap[int, int]()
	for i := range 1000 {
		m = m.Set(i, i)
	}
	b.ResetTimer()
	for i := range b.N {
		_ = m.Delete(i % 1000)
	}
}
