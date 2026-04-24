package persistent

import "testing"

func TestUtil(t *testing.T) {
	t.Parallel()
	t.Run("Len helper", func(t *testing.T) {
		t.Parallel()
		m := NewMap[string, int]().Set("a", 1)
		if Len(m) != 1 {
			t.Errorf("Expected 1, got %d", Len(m))
		}

		l := NewList[int]().Append(10).Append(20)
		if Len(l) != 2 {
			t.Errorf("Expected 2, got %d", Len(l))
		}

		var nm Map[string, int]
		if Len(nm) != 0 {
			t.Errorf("Expected 0 for nil map, got %d", Len(nm))
		}
	})
}
