package transpiler

import (
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegration(t *testing.T) {
	code := `package main
import "fmt"
func main() {
    m := map[string]int{"a": 1}
    m := m.Set("b", 2)
    l := []int{10, 20}
    l := l.Append(30)
    
    v, ok := m["a"]
    fmt.Printf("m[a]=%d,%v ", v, ok)
    
    fmt.Printf("len(m)=%d ", len(m))
    fmt.Printf("l[2]=%d", l[2])
}
`

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.im", code, 0)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	f = Rewrite(f)

	tmp := t.TempDir()
	genPath := filepath.Join(tmp, "main_gen.go")
	file, err := os.Create(genPath)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	printer.Fprint(file, fset, f)
	file.Close()

	// We need to run this in the context of the module to find 'persistent'
	// Since we are in 'pkg/transpiler', the module root is '../..'
	root, _ := filepath.Abs("../..")
	
	cmd := exec.Command("go", "run", genPath)
	cmd.Dir = root // Run from root to use the module's go.mod
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Run error: %v\nOutput: %s", err, string(output))
	}

	expected := "m[a]=1,true len(m)=2 l[2]=30"
	got := strings.TrimSpace(string(output))
	if got != expected {
		t.Errorf("Expected %q, got %q", expected, got)
	}
}
