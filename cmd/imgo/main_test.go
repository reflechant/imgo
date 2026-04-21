package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Run("Usage", func(t *testing.T) {
		err := run([]string{"imgo"})
		if err == nil || err.Error() != "usage: imgo <path>" {
			t.Errorf("Expected usage error, got %v", err)
		}
	})

	t.Run("No files found", func(t *testing.T) {
		tmp := t.TempDir()
		err := run([]string{"imgo", tmp})
		if err == nil || !strings.Contains(err.Error(), "no .im files found") {
			t.Errorf("Expected error for no files found, got %v", err)
		}
	})

	t.Run("Non-existent path", func(t *testing.T) {
		err := run([]string{"imgo", "/non/existent/path/at/all/hopefully"})
		if err == nil || !strings.Contains(err.Error(), "no such file or directory") {
			t.Errorf("Expected path error, got %v", err)
		}
	})

	t.Run("Valid compilation", func(t *testing.T) {
		tmp := t.TempDir()
		imFile := filepath.Join(tmp, "test.im")
		code := `package main
func main() {
	println("hello")
}`
		if err := os.WriteFile(imFile, []byte(code), 0644); err != nil {
			t.Fatal(err)
		}

		err := run([]string{"imgo", tmp})
		if err != nil {
			t.Errorf("Expected success, got error: %v", err)
		}

		genFile := filepath.Join(tmp, "test_imgo_gen.go")
		if _, err := os.Stat(genFile); os.IsNotExist(err) {
			t.Errorf("Expected generated file %s to exist", genFile)
		}
	})

	t.Run("Syntax Error", func(t *testing.T) {
		tmp := t.TempDir()
		imFile := filepath.Join(tmp, "test.im")
		code := `package main
func main() {
	println("hello"
}`
		if err := os.WriteFile(imFile, []byte(code), 0644); err != nil {
			t.Fatal(err)
		}

		err := run([]string{"imgo", tmp})
		if err == nil || !strings.Contains(err.Error(), "missing ',' before newline") {
			t.Errorf("Expected syntax error, got %v", err)
		}
	})

	t.Run("Parse Error", func(t *testing.T) {
		tmp := t.TempDir()
		imFile := filepath.Join(tmp, "test.im")
		// Not a valid Go file at all
		code := `not a package`
		if err := os.WriteFile(imFile, []byte(code), 0644); err != nil {
			t.Fatal(err)
		}

		err := run([]string{"imgo", tmp})
		if err == nil || !strings.Contains(err.Error(), "expected 'package'") {
			t.Errorf("Expected parse error, got %v", err)
		}
	})

	t.Run("Create Error", func(t *testing.T) {
		tmp := t.TempDir()
		imFile := filepath.Join(tmp, "test.im")
		code := `package main
func main() {}`
		if err := os.WriteFile(imFile, []byte(code), 0644); err != nil {
			t.Fatal(err)
		}

		// Make generated file name a directory to cause create error
		genFile := filepath.Join(tmp, "test_imgo_gen.go")
		if err := os.Mkdir(genFile, 0755); err != nil {
			t.Fatal(err)
		}

		err := run([]string{"imgo", tmp})
		if err == nil || !strings.Contains(err.Error(), "error creating file") {
			t.Errorf("Expected create error, got %v", err)
		}
	})
}
