package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Parallel()
	t.Run("Usage", func(t *testing.T) {
		t.Parallel()
		err := run([]string{"imgo"})
		if err == nil || err.Error() != "usage: imgo <path>" {
			t.Errorf("Expected usage error, got %v", err)
		}
	})

	t.Run("No files found", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		err := run([]string{"imgo", tmp})
		if err == nil || !strings.Contains(err.Error(), "no .im files found") {
			t.Errorf("Expected error for no files found, got %v", err)
		}
	})

	t.Run("Non-existent path", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		err := run([]string{"imgo", filepath.Join(tmp, "nonexistent")})
		if err == nil || !strings.Contains(err.Error(), "no such file or directory") {
			t.Errorf("Expected path error, got %v", err)
		}
	})

	t.Run("Valid compilation", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		imFile := filepath.Join(tmp, "test.im")
		code := `package main
func main() {
	println("hello")
}`
		err := os.WriteFile(imFile, []byte(code), 0o600)
		if err != nil {
			t.Fatal(err)
		}

		err = run([]string{"imgo", tmp})
		if err != nil {
			t.Errorf("Expected success, got error: %v", err)
		}

		genFile := filepath.Join(tmp, "test_imgo_gen.go")
		_, statErr := os.Stat(genFile)
		if os.IsNotExist(statErr) {
			t.Errorf("Expected generated file %s to exist", genFile)
		}
	})

	t.Run("Syntax Error", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		imFile := filepath.Join(tmp, "test.im")
		code := `package main
func main() {
	println("hello"
}`
		err := os.WriteFile(imFile, []byte(code), 0o600)
		if err != nil {
			t.Fatal(err)
		}

		err = run([]string{"imgo", tmp})
		if err == nil || !strings.Contains(err.Error(), "missing ',' before newline") {
			t.Errorf("Expected syntax error, got %v", err)
		}
	})

	t.Run("Parse Error", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		imFile := filepath.Join(tmp, "test.im")
		// Not a valid Go file at all
		code := `not a package`
		err := os.WriteFile(imFile, []byte(code), 0o600)
		if err != nil {
			t.Fatal(err)
		}

		err = run([]string{"imgo", tmp})
		if err == nil || !strings.Contains(err.Error(), "expected 'package'") {
			t.Errorf("Expected parse error, got %v", err)
		}
	})

	t.Run("Create Error", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		imFile := filepath.Join(tmp, "test.im")
		code := `package main
func main() {}`
		err := os.WriteFile(imFile, []byte(code), 0o600)
		if err != nil {
			t.Fatal(err)
		}

		// Create a directory with the same name as expected generated file
		genFile := filepath.Join(tmp, "test_imgo_gen.go")
		mkdirErr := os.Mkdir(genFile, 0o750)
		if mkdirErr != nil {
			t.Fatal(mkdirErr)
		}

		err = run([]string{"imgo", tmp})
		if err == nil || !strings.Contains(err.Error(), "error creating file") {
			t.Errorf("Expected create error, got %v", err)
		}
	})
}
