package transpiler

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestTranspile(t *testing.T) {
	t.Run("Valid code", func(t *testing.T) {
		fset := token.NewFileSet()
		src := `package main
func main() {
	x := 1
	x := x + 1
	println(x)
}`
		f, err := parser.ParseFile(fset, "test.im", src, 0)
		if err != nil {
			t.Fatal(err)
		}

		out, err := Transpile(fset, f)
		if err != nil {
			t.Fatalf("Expected success, got error: %v", err)
		}
		if out == nil {
			t.Fatal("Expected output file, got nil")
		}
	})

	t.Run("Validation error", func(t *testing.T) {
		fset := token.NewFileSet()
		// Mutating assignment is prohibited in ImGo
		src := `package main
func main() {
	x := 1
	x = 2
}`
		f, err := parser.ParseFile(fset, "test.im", src, 0)
		if err != nil {
			t.Fatal(err)
		}

		_, err = Transpile(fset, f)
		if err == nil {
			t.Fatal("Expected validation error, got nil")
		}
		if !strings.Contains(err.Error(), "is prohibited") {
			t.Errorf("Expected mutation error, got: %v", err)
		}
	})
}
