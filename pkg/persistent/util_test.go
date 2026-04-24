package persistent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUtil(t *testing.T) {
	t.Parallel()
	t.Run("Len helper", func(t *testing.T) {
		t.Parallel()
		m := NewMap[string, int]().Set("a", 1)
		assert.Equal(t, 1, Len(m))

		l := NewList[int]().Append(10).Append(20)
		assert.Equal(t, 2, Len(l))

		var nm Map[string, int]
		assert.Equal(t, 0, Len(nm))
	})
}
