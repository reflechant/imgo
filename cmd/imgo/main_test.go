package main

import (
	"os"
	"path/filepath"
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
		if err == nil || !contains(err.Error(), "no .im files found") {
			t.Errorf("Expected error for no files found, got %v", err)
		}
	})

	t.Run("Non-existent path", func(t *testing.T) {
		err := run([]string{"imgo", "/non/existent/path/imgo"})
		if err == nil {
			t.Errorf("Expected error for non-existent path")
		}
	})

	t.Run("Valid compilation", func(t *testing.T) {
		tmp := t.TempDir()
		src := filepath.Join(tmp, "test.im")
		code := `package main
func main() {
    x := 5
    fmt.Println(x)
}`
		if err := os.WriteFile(src, []byte(code), 0644); err != nil {
			t.Fatal(err)
		}

		err := run([]string{"imgo", tmp})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		gen := filepath.Join(tmp, "test_imgo_gen.go")
		if _, err := os.Stat(gen); os.IsNotExist(err) {
			t.Errorf("Generated file %s not found", gen)
		}
	})

	t.Run("Syntax Error", func(t *testing.T) {
		tmp := t.TempDir()
		src := filepath.Join(tmp, "bad.im")
		code := `package main
func main() {
    x := 5
    x = 10 // Prohibited mutation
}`
		if err := os.WriteFile(src, []byte(code), 0644); err != nil {
			t.Fatal(err)
		}

		err := run([]string{"imgo", tmp})
		if err == nil || !contains(err.Error(), "transpilation error") {
			t.Errorf("Expected transpilation error, got %v", err)
		}
	})

	t.Run("Parse Error", func(t *testing.T) {
		tmp := t.TempDir()
		src := filepath.Join(tmp, "parse.im")
		code := `package main
func main() {`
		if err := os.WriteFile(src, []byte(code), 0644); err != nil {
			t.Fatal(err)
		}

		err := run([]string{"imgo", tmp})
		if err == nil || !contains(err.Error(), "error parsing") {
			t.Errorf("Expected parse error, got %v", err)
		}
	})

	t.Run("Create Error", func(t *testing.T) {
		tmp := t.TempDir()
		src := filepath.Join(tmp, "readonly.im")
		if err := os.WriteFile(src, []byte("package main"), 0644); err != nil {
			t.Fatal(err)
		}
		// Make directory read-only
		if err := os.Chmod(tmp, 0555); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chmod(tmp, 0755) }()

		err := run([]string{"imgo", tmp})
		if err == nil || !contains(err.Error(), "error creating file") {
			t.Errorf("Expected create error, got %v", err)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(substr) > 0 && (len(s) >= len(substr) && (s[0:len(substr)] == substr || contains(s[1:], substr)))))
}
