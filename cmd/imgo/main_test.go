package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	t.Parallel()
	t.Run("Usage", func(t *testing.T) {
		t.Parallel()
		err := run([]string{"imgo"})
		require.EqualError(t, err, "usage: imgo <path>")
	})

	t.Run("No files found", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		err := run([]string{"imgo", tmp})
		require.NoError(t, err)
	})

	t.Run("Non-existent path", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		err := run([]string{"imgo", filepath.Join(tmp, "nonexistent")})
		require.ErrorContains(t, err, "no such file or directory")
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
		require.NoError(t, err)

		err = run([]string{"imgo", tmp})
		require.NoError(t, err)

		genFile := filepath.Join(tmp, "test_imgo_gen.go")
		assert.FileExists(t, genFile)
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
		require.NoError(t, err)

		err = run([]string{"imgo", tmp})
		require.ErrorContains(t, err, "missing ',' before newline")
	})

	t.Run("Parse Error", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		imFile := filepath.Join(tmp, "test.im")
		// Not a valid Go file at all
		code := `not a package`
		err := os.WriteFile(imFile, []byte(code), 0o600)
		require.NoError(t, err)

		err = run([]string{"imgo", tmp})
		require.ErrorContains(t, err, "expected 'package'")
	})

	t.Run("Create Error", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		imFile := filepath.Join(tmp, "test.im")
		code := `package main
func main() {}`
		err := os.WriteFile(imFile, []byte(code), 0o600)
		require.NoError(t, err)

		// Create a directory with the same name as expected generated file
		genFile := filepath.Join(tmp, "test_imgo_gen.go")
		mkdirErr := os.Mkdir(genFile, 0o750)
		require.NoError(t, mkdirErr)

		err = run([]string{"imgo", tmp})
		require.ErrorContains(t, err, "error creating file")
	})
}
