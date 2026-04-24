package transpiler

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranspile(t *testing.T) {
	t.Parallel()
	t.Run("Valid code", func(t *testing.T) {
		t.Parallel()
		fset := token.NewFileSet()
		src := `package main
func main() {
	x := 1
	x := x + 1
	println(x)
}`
		f, err := parser.ParseFile(fset, "test.im", src, 0)
		require.NoError(t, err)

		out, err := Transpile(fset, f)
		require.NoError(t, err, "Expected success, got error")
		assert.NotNil(t, out, "Expected output file, got nil")
	})

	t.Run("Validation error", func(t *testing.T) {
		t.Parallel()
		fset := token.NewFileSet()
		// Mutating assignment is prohibited in ImGo
		src := `package main
func main() {
	x := 1
	x = 2
}`
		f, err := parser.ParseFile(fset, "test.im", src, 0)
		require.NoError(t, err)

		_, err = Transpile(fset, f)
		require.Error(t, err, "Expected validation error, got nil")
		assert.ErrorContains(t, err, "is prohibited")
	})
}
