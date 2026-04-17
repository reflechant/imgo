package transpiler

import (
	"bytes"
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
	testdata := "testdata"
	entries, err := os.ReadDir(testdata)
	if err != nil {
		t.Fatalf("ReadDir error: %v", err)
	}

	root, _ := filepath.Abs("../..")

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".im") {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			path := filepath.Join(testdata, entry.Name())
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile error: %v", err)
			}

			// Split code and expectation
			parts := strings.Split(string(content), "// -- EXPECT --")
			if len(parts) != 2 {
				t.Fatalf("Test file %s missing '// -- EXPECT --' marker", entry.Name())
			}

			imgoCode := parts[0]
			expectedOutput := strings.TrimSpace(parts[1])
			// Remove leading "// " from each line of expected output
			lines := strings.Split(expectedOutput, "\n")
			for i, line := range lines {
				lines[i] = strings.TrimPrefix(strings.TrimSpace(line), "//")
				lines[i] = strings.TrimSpace(lines[i])
			}
			expectedOutput = strings.Join(lines, "\n")

			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, entry.Name(), imgoCode, 0)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			f, err = Transpile(fset, f)
			if err != nil {
				t.Fatalf("Transpilation error in %s: %v", entry.Name(), err)
			}

			tmp := t.TempDir()
			genPath := filepath.Join(tmp, entry.Name()+"_gen.go")
			file, err := os.Create(genPath)
			if err != nil {
				t.Fatalf("Create error: %v", err)
			}
			_ = printer.Fprint(file, fset, f)
			_ = file.Close()

			cmd := exec.Command("go", "run", genPath)
			cmd.Dir = root // Run from root to find 'persistent' package
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err = cmd.Run()
			if err != nil {
				t.Fatalf("Run error: %v\nStderr: %s\nStdout: %s", err, stderr.String(), stdout.String())
			}

			got := strings.TrimSpace(stdout.String())
			if got != expectedOutput {
				t.Errorf("Output mismatch.\nExpected:\n%s\n\nGot:\n%s", expectedOutput, got)
			}
		})
	}
}
