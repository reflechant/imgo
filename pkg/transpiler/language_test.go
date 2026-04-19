package transpiler

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Language tests: the primary suite for ImGo semantics. Each case is a bare
// body snippet (no package/main/import wrapper) plus either:
//
//   - observe + want: harness appends `fmt.Printf("name=%v\n", name)` for
//     each observed identifier, then compares the full stdout to want.
//
//   - raw + want: snippet owns its own output via fmt.Println/Printf and
//     the harness compares stdout verbatim.
//
// Each case wraps, rewrites, writes to a tempdir, invokes `go run`, and
// matches the output. Slow compared to pure-unit tests, but this is the
// only place ImGo's runtime behaviour is actually exercised.

type langCase struct {
	name    string
	src     string
	observe []string
	raw     bool
	want    string
}

func (tc langCase) source() string {
	var b strings.Builder
	b.WriteString("package main\nimport \"fmt\"\nfunc main() {\n")
	b.WriteString(tc.src)
	b.WriteString("\n")
	if !tc.raw {
		for _, name := range tc.observe {
			fmt.Fprintf(&b, "fmt.Printf(%q, %s)\n", name+"=%v\n", name)
		}
	}
	b.WriteString("}\n")
	return b.String()
}

func runLang(t *testing.T, tc langCase) {
	t.Helper()
	if !tc.raw && len(tc.observe) == 0 {
		t.Fatal("langCase: set raw=true or provide at least one observe entry")
	}

	goSrc := rewriteSrc(t, tc.source())
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("abs repo root: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(goSrc), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	cmd := exec.Command("go", "run", path)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("go run failed: %v\nstderr:\n%s\nstdout:\n%s\n--- generated Go ---\n%s",
			err, stderr.String(), stdout.String(), goSrc)
	}

	if got := stdout.String(); got != tc.want {
		t.Errorf("output mismatch\n--- want ---\n%s--- got ---\n%s--- generated Go ---\n%s",
			tc.want, got, goSrc)
	}
}

func TestLanguage(t *testing.T) {
	cases := []langCase{
		// --- SSA / scoping ---------------------------------------------------
		{
			name:    "basic arithmetic",
			src:     `x := 5; y := x + 1`,
			observe: []string{"x", "y"},
			want:    "x=5\ny=6\n",
		},
		{
			name: "rebinding produces a fresh value",
			src: `
				x := 1
				x := x + 10
			`,
			observe: []string{"x"},
			want:    "x=11\n",
		},
		{
			name: "closure captures the binding at creation time",
			src: `
				x := 10
				f := func() int { return x }
				x := 20
				r := f()
				s := x
			`,
			observe: []string{"r", "s"},
			want:    "r=10\ns=20\n",
		},
		{
			name: "if init does not leak to the enclosing scope",
			src: `
				x := 1
				if x := 2; x > 0 {
					// inner x is 2 but invisible after the block
				}
				y := x
			`,
			observe: []string{"y"},
			want:    "y=1\n",
		},
		{
			name: "switch init does not leak",
			src: `
				x := 1
				switch x := 2; {
				default:
					_ = x
				}
				y := x
			`,
			observe: []string{"y"},
			want:    "y=1\n",
		},
		{
			name: "for body shadow doesn't leak",
			src: `
				x := 1
				for i := 0; i < 3; {
					x := i + 10
					_ = x
					break
				}
				y := x
			`,
			observe: []string{"y"},
			want:    "y=1\n",
		},
		{
			name: "range body shadow doesn't leak",
			src: `
				x := 1
				l := []int{100, 200}
				for _, x := range l {
					_ = x
				}
				y := x
			`,
			observe: []string{"y"},
			want:    "y=1\n",
		},
		{
			name: "else-if init binds its own scope",
			src: `
				r := 0
				if x := 0; x > 0 {
					r = 1
				} else if y := 5; y > 0 {
					r = y
				}
			`,
			observe: []string{"r"},
			want:    "r=5\n",
		},

		// --- Map literals, get, set, delete ----------------------------------
		{
			name: "map literal + index",
			src: `
				m := map[string]int{"a": 1, "b": 2}
				a := m["a"]
				b := m["b"]
				n := len(m)
			`,
			observe: []string{"a", "b", "n"},
			want:    "a=1\nb=2\nn=2\n",
		},
		{
			name: "missing key returns the zero value",
			src: `
				m := map[string]int{"a": 1}
				z := m["missing"]
			`,
			observe: []string{"z"},
			want:    "z=0\n",
		},
		{
			name: "two-value index reports presence",
			src: `
				m := map[string]int{"a": 1}
				v1, ok1 := m["a"]
				v2, ok2 := m["x"]
			`,
			observe: []string{"v1", "ok1", "v2", "ok2"},
			want:    "v1=1\nok1=true\nv2=0\nok2=false\n",
		},
		{
			name: "Set returns a new map, original unchanged",
			src: `
				m := map[string]int{"a": 1}
				m1 := m.Set("b", 2)
				n := len(m)
				n1 := len(m1)
				b := m1["b"]
			`,
			observe: []string{"n", "n1", "b"},
			want:    "n=1\nn1=2\nb=2\n",
		},
		{
			name: "Delete returns a new map",
			src: `
				m := map[string]int{"a": 1, "b": 2}
				m1 := m.Delete("a")
				n := len(m)
				n1 := len(m1)
				_, ok := m1["a"]
			`,
			observe: []string{"n", "n1", "ok"},
			want:    "n=2\nn1=1\nok=false\n",
		},

		// --- List literals, index, Append, slice -----------------------------
		{
			name: "list literal + index + len",
			src: `
				l := []int{10, 20, 30}
				a := l[0]
				b := l[2]
				n := len(l)
			`,
			observe: []string{"a", "b", "n"},
			want:    "a=10\nb=30\nn=3\n",
		},
		{
			name: "Append returns a new list",
			src: `
				l := []int{1, 2}
				l1 := l.Append(3)
				n := len(l)
				n1 := len(l1)
				last := l1[2]
			`,
			observe: []string{"n", "n1", "last"},
			want:    "n=2\nn1=3\nlast=3\n",
		},
		{
			name: "slice with explicit bounds",
			src: `
				l := []int{10, 20, 30, 40, 50}
				s := l[1:4]
				n := len(s)
				first := s[0]
				last := s[2]
			`,
			observe: []string{"n", "first", "last"},
			want:    "n=3\nfirst=20\nlast=40\n",
		},
		{
			name: "slice with open-ended bounds",
			src: `
				l := []int{10, 20, 30, 40, 50}
				a := len(l[:3])
				b := len(l[2:])
				c := len(l[:])
				d := len(l[2:2])
			`,
			observe: []string{"a", "b", "c", "d"},
			want:    "a=3\nb=3\nc=5\nd=0\n",
		},

		// --- Builtins --------------------------------------------------------
		{
			name: "set/get/update/delete builtins on a map",
			src: `
				m := map[string]int{"a": 1}
				m1 := set(m, "b", 2)
				v := get(m1, "a")
				w := get(m1, "b")
				m2 := update(m1, "a", func(x int) int { return x * 10 })
				a := get(m2, "a")
				m3 := delete(m2, "b")
				n := len(m3)
			`,
			observe: []string{"v", "w", "a", "n"},
			want:    "v=1\nw=2\na=10\nn=1\n",
		},
		{
			name: "two-value get builtin reports presence",
			src: `
				m := map[string]int{"a": 1}
				v, ok := get(m, "missing")
			`,
			observe: []string{"v", "ok"},
			want:    "v=0\nok=false\n",
		},
		{
			name: "getIn/setIn/updateIn/deleteIn walk nested maps",
			src: `
				m := map[string]map[string]int{
					"a": map[string]int{"b": 1},
				}
				v1 := getIn(m, "a", "b")
				m1 := setIn(m, "a", "c", 2)
				v2 := getIn(m1, "a", "c")
				m2 := setIn(m, "x", "y", 3)
				v3 := getIn(m2, "x", "y")
				m3 := updateIn(m1, "a", "b", func(x int) int { return x + 10 })
				v4 := getIn(m3, "a", "b")
				m4 := deleteIn(m1, "a", "b")
				_, ok := get(m4["a"], "b")
			`,
			observe: []string{"v1", "v2", "v3", "v4", "ok"},
			want:    "v1=1\nv2=2\nv3=3\nv4=11\nok=false\n",
		},
		{
			name: "builtin shadowed by local function is not rewritten",
			src: `
				set := func(_ any, _ string, v int) int { return v + 100 }
				m := map[string]int{"a": 1}
				x := set(m, "a", 7)
			`,
			observe: []string{"x"},
			want:    "x=107\n",
		},

		// --- make ------------------------------------------------------------
		{
			name: "make map produces an empty persistent map",
			src: `
				m := make(map[string]int)
				m1 := m.Set("a", 1)
				n := len(m)
				n1 := len(m1)
			`,
			observe: []string{"n", "n1"},
			want:    "n=0\nn1=1\n",
		},
		{
			name: "make list produces an empty persistent list (size hint ignored)",
			src: `
				l := make([]int, 10)
				n := len(l)
				l1 := l.Append(42)
				n1 := len(l1)
				first := l1[0]
			`,
			observe: []string{"n", "n1", "first"},
			want:    "n=0\nn1=1\nfirst=42\n",
		},

		// --- Control flow / side-effectful output (raw) ----------------------
		{
			name: "range over list yields (index, value) pairs",
			src: `
				l := []int{10, 20, 30}
				for i, v := range l {
					fmt.Printf("[%d]=%d\n", i, v)
				}
			`,
			raw:  true,
			want: "[0]=10\n[1]=20\n[2]=30\n",
		},
		{
			name: "closure + rebind emits captured old value",
			src: `
				x := 10
				f := func() { fmt.Printf("captured=%d\n", x) }
				x := 20
				f()
				fmt.Printf("shadowed=%d\n", x)
			`,
			raw:  true,
			want: "captured=10\nshadowed=20\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runLang(t, tc)
		})
	}
}
