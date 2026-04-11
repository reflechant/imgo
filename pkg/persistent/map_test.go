package persistent

import (
	"testing"
)

func TestMap(t *testing.T) {
	t.Run("NewMap", func(t *testing.T) {
		m := NewMap[string, int]()
		if m.Len() != 0 {
			t.Errorf("Expected len 0, got %d", m.Len())
		}
	})

	t.Run("Set and Lookup/Get", func(t *testing.T) {
		m := NewMap[string, int]()
		m2 := m.Set("a", 1).Set("b", 2)
		if m.Len() != 0 {
			t.Errorf("Original map should be empty, got %d", m.Len())
		}
		if m2.Len() != 2 {
			t.Errorf("New map should have len 2, got %d", m2.Len())
		}
		v, ok := m2.Lookup("a")
		if !ok || v != 1 {
			t.Errorf("Expected 1 at key 'a', got %v, %v", v, ok)
		}
		v = m2.Get("a")
		if v != 1 {
			t.Errorf("Expected 1 at key 'a' via Get, got %v", v)
		}
		v, ok = m2.Lookup("c")
		if ok {
			t.Errorf("Key 'c' should not exist")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		m := NewMap[string, int]().Set("a", 1).Set("b", 2)
		m2 := m.Delete("a")
		if m.Len() != 2 {
			t.Errorf("Original map should have 2 items, got %d", m.Len())
		}
		if m2.Len() != 1 {
			t.Errorf("New map should have 1 item, got %d", m2.Len())
		}
		if _, ok := m2.Lookup("a"); ok {
			t.Errorf("Key 'a' should be deleted")
		}
		// Nil map delete
		var nm Map[string, int]
		nm2 := nm.Delete("a")
		if nm2.Len() != 0 {
			t.Errorf("Expected 0 items in deleted nil map")
		}
	})

	t.Run("Update", func(t *testing.T) {
		m := NewMap[string, int]().Set("a", 1)
		m2 := m.Update("a", func(v int) int { return v + 10 })
		v := m2.Get("a")
		if v != 11 {
			t.Errorf("Expected 11, got %v", v)
		}
	})

	t.Run("Nil handling (Nil punning)", func(t *testing.T) {
		var m Map[string, int]
		if m.Len() != 0 {
			t.Errorf("Nil map should have len 0, got %d", m.Len())
		}
		// Test Lookup on nil map
		v, ok := m.Lookup("a")
		if ok || v != 0 {
			t.Errorf("Expected (0, false) for nil map Lookup, got (%v, %v)", v, ok)
		}
		m2 := m.Set("a", 1)
		if m2.Len() != 1 {
			t.Errorf("Set nil map should have len 1, got %d", m2.Len())
		}
		v = m2.Get("a")
		if v != 1 {
			t.Errorf("Expected 1, got %v", v)
		}
	})

	t.Run("All() iterator", func(t *testing.T) {
		m := NewMap[string, int]().Set("a", 1).Set("b", 2)
		var sum int
		for _, v := range m.All() {
			sum += v
		}
		if sum != 3 {
			t.Errorf("Expected sum 3, got %d", sum)
		}

		// Nil map iteration
		var nm Map[string, int]
		for k, v := range nm.All() {
			t.Errorf("Nil map should not yield values, got %v: %v", k, v)
		}

		// Early break
		count := 0
		for range m.All() {
			count++
			break
		}
		if count != 1 {
			t.Errorf("Expected count 1 after early break, got %d", count)
		}
	})

	t.Run("SetIn / UpdateIn (Empty path)", func(t *testing.T) {
		m := NewMap[string, int]().Set("a", 1)
		m2 := m.SetIn([]string{}, 10)
		if m2.Len() != 1 || m2.Get("a") != 1 {
			t.Errorf("Expected unchanged map for empty path in SetIn")
		}
		m3 := m.UpdateIn([]string{}, func(v int) int { return v + 1 })
		if m3.Len() != 1 || m3.Get("a") != 1 {
			t.Errorf("Expected unchanged map for empty path in UpdateIn")
		}
	})

	t.Run("SetIn / UpdateIn (Basic typed cases)", func(t *testing.T) {
		m := NewMap[string, int]()
		m2 := m.SetIn([]string{"a"}, 10)
		v := m2.Get("a")
		if v != 10 {
			t.Errorf("Expected 10, got %v", v)
		}

		m3 := m2.UpdateIn([]string{"a"}, func(v int) int { return v * 2 })
		v = m3.Get("a")
		if v != 20 {
			t.Errorf("Expected 20, got %v", v)
		}
	})

	t.Run("SetIn / UpdateIn (Nested any cases)", func(t *testing.T) {
		m := NewMap[string, any]()
		m2 := m.SetIn([]string{"a", "b"}, 42)

		val := m2.Get("a")
		subMap := val.(Map[string, any])
		v := subMap.Get("b")
		if v != 42 {
			t.Errorf("Expected 42, got %v", v)
		}

		m3 := m2.UpdateIn([]string{"a", "b"}, func(v any) any { return v.(int) + 8 })
		val = m3.Get("a")
		subMap = val.(Map[string, any])
		v = subMap.Get("b")
		if v != 50 {
			t.Errorf("Expected 50, got %v", v)
		}
	})

	t.Run("SetIn (Intermediate not a map)", func(t *testing.T) {
		m := NewMap[string, any]().Set("a", 123)
		m2 := m.SetIn([]string{"a", "b"}, 456)
		val := m2.Get("a")
		_, ok := val.(Map[string, any])
		if !ok {
			t.Errorf("Expected 123 to be overwritten by a map, got %T", val)
		}
	})

	t.Run("UpdateIn (Intermediate not a map)", func(t *testing.T) {
		m := NewMap[string, any]().Set("a", 123)
		m2 := m.UpdateIn([]string{"a", "b"}, func(v any) any { return v })
		val := m2.Get("a")
		_, ok := val.(Map[string, any])
		if !ok {
			t.Errorf("Expected 123 to be overwritten by a map in UpdateIn, got %T", val)
		}
	})

	t.Run("SetIn (Existing submap)", func(t *testing.T) {
		m := NewMap[string, any]()
		m2 := m.SetIn([]string{"a", "b"}, 1)
		m3 := m2.SetIn([]string{"a", "c"}, 2)
		// m3["a"] should have both "b":1 and "c":2
		val := m3.Get("a")
		sub := val.(Map[string, any])
		if sub.Len() != 2 {
			t.Errorf("Expected submap len 2, got %d", sub.Len())
		}
	})

	t.Run("DeleteIn", func(t *testing.T) {
		m := NewMap[string, any]().SetIn([]string{"a", "b", "c"}, 100)
		m2 := m.DeleteIn([]string{"a", "b", "c"})
		val := m2.Get("a")
		subA := val.(Map[string, any])
		val = subA.Get("b")
		subB := val.(Map[string, any])
		if subB.Len() != 0 {
			t.Errorf("Expected empty submap after DeleteIn, got %d", subB.Len())
		}

		// Empty path
		m3 := m.DeleteIn([]string{})
		if m3.Len() != 1 {
			t.Errorf("Empty path DeleteIn should not change map")
		}

		// Path not found
		m4 := m.DeleteIn([]string{"a", "x"})
		if m4.Len() != 1 {
			t.Errorf("Non-existent path DeleteIn should not change map")
		}

		// Intermediate not a map
		m5 := NewMap[string, any]().Set("a", 123)
		m6 := m5.DeleteIn([]string{"a", "b"})
		if m6.Get("a") != 123 {
			t.Errorf("DeleteIn on non-map intermediate should not change it")
		}

		// Single element path
		m7 := m5.DeleteIn([]string{"a"})
		if m7.Len() != 0 {
			t.Errorf("Expected empty map after single element DeleteIn")
		}

		// Path not found (first key doesn't exist, path len > 1)
		m8 := m.DeleteIn([]string{"nonexistent", "b"})
		if m8.Len() != 1 {
			t.Errorf("Non-existent path (len > 1) DeleteIn should not change map")
		}
	})
}
