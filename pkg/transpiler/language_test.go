package transpiler

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Language tests: the primary suite for ImGo semantics. Each case is a bare
// body snippet (no package/main/import wrapper) plus one or more @assert
// directives.
//
// All cases are transpiled upfront and compiled into a single binary
// (one go run), keeping total wall time proportional to one compilation.

type langCase struct {
	name string
	src  string
}

func (tc langCase) source() string {
	var b strings.Builder
	b.WriteString("package main\nimport \"fmt\"\nfunc main() {\n")
	b.WriteString(tc.src)
	b.WriteString("\n}\n")

	return b.String()
}

func parseLanguageTests(path string) ([]langCase, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	var cases []langCase
	var current *langCase

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if name, ok := strings.CutPrefix(trimmed, "// @test"); ok {
			if current != nil {
				cases = append(cases, *current)
			}
			current = &langCase{
				name: strings.TrimSpace(name),
			}

			continue
		}

		if current == nil {
			continue
		}

		if expr, ok := strings.CutPrefix(trimmed, "// @assert"); ok {
			expr = strings.TrimSpace(expr)
			// Inject assertion check directly into src.
			// The transpiler will mangle the identifiers in expr correctly.
			current.src += fmt.Sprintf("\nif !(%s) { fmt.Printf(\"ASSERTION FAILED at %s:%d: %%s\\n\", %q) }\n",
				expr, filepath.Base(path), i+1, expr)

			continue
		}

		current.src += line + "\n"
	}

	if current != nil {
		cases = append(cases, *current)
	}

	return cases, nil
}

func TestLanguage(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "tests", "language.im")
	cases, err := parseLanguageTests(path)
	require.NoError(t, err, "parse tests")

	root, err := filepath.Abs("../..")
	require.NoError(t, err, "abs repo root")
	dir := t.TempDir()

	const sep = "\x1c" // ASCII File Separator — not present in any test output

	// Rewrite all cases into per-case files (case0.go, case1.go, …).
	goSrcs := make([]string, len(cases))
	filePaths := make([]string, 0, len(cases)+1)
	for i, tc := range cases {
		src := rewriteSrc(t, tc.source())
		goSrcs[i] = src
		// Rename func main() → func caseN() so all files share package main.
		caseSrc := strings.Replace(src, "func main()", fmt.Sprintf("func case%d()", i), 1)
		// Prefix with a comment identifying the case for human readers.
		caseSrc = fmt.Sprintf("// case %d: %s\n", i, tc.name) + caseSrc
		path := filepath.Join(dir, fmt.Sprintf("case%d.go", i))
		err := os.WriteFile(path, []byte(caseSrc), 0o600)
		require.NoError(t, err, "write case%d.go", i)
		filePaths = append(filePaths, path)
	}

	// Build main.go: calls caseN(); fmt.Print(sep) for each N.
	var mb strings.Builder
	mb.WriteString("package main\nimport \"fmt\"\nfunc main() {\n")
	for i := range cases {
		fmt.Fprintf(&mb, "\tcase%d()\n\tfmt.Print(%q)\n", i, sep)
	}
	mb.WriteString("}\n")
	mainPath := filepath.Join(dir, "main.go")
	err = os.WriteFile(mainPath, []byte(mb.String()), 0o600)
	require.NoError(t, err, "write main.go")
	filePaths = append(filePaths, mainPath)

	// One compilation for all cases.
	args := append([]string{"run"}, filePaths...)
	cmd := exec.CommandContext(t.Context(), "go", args...) //nolint:gosec
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	require.NoErrorf(t, err, "go run failed\nstderr:\n%s\nstdout:\n%s",
		stderr.String(), stdout.String())

	// Split on separator; last element is "" (after the final sep).
	parts := strings.Split(stdout.String(), sep)
	require.GreaterOrEqual(t, len(parts), len(cases), "output split produced too few parts")

	for i, tc := range cases {
		name := fmt.Sprintf("%03d_%s", i+1, tc.name)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			output := strings.TrimSpace(parts[i])
			if output != "" {
				t.Errorf("Assertion failure(s):\n%s\n--- generated Go ---\n%s", output, goSrcs[i])
			}
		})
	}
}
