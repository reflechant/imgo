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
// body snippet (no package/main/import wrapper) plus:
//
//   - want: expected stdout, built with out(...). For observe-style tests the
//     harness derives the Printf list from the "name=value" keys in want.
//     For raw tests (raw: true) the snippet owns its own output.
//
// All cases are transpiled upfront and compiled into a single binary
// (one go run), keeping total wall time proportional to one compilation.

type langCase struct {
	name string
	src  string
	raw  bool
	want string
}

// out joins output lines into the expected stdout string.
// Use instead of "a=1\nb=2\n" — write out("a=1", "b=2") instead.
func out(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}

// observeFromWant extracts the identifier names from the "name=value" lines
// in want. The result is used to auto-append fmt.Printf statements in source().
func observeFromWant(want string) []string {
	var names []string
	for line := range strings.SplitSeq(strings.TrimRight(want, "\n"), "\n") {
		if i := strings.IndexByte(line, '='); i > 0 {
			names = append(names, line[:i])
		}
	}

	return names
}

func (tc langCase) source() string {
	var b strings.Builder
	b.WriteString("package main\nimport \"fmt\"\nfunc main() {\n")
	b.WriteString(tc.src)
	b.WriteString("\n")
	if !tc.raw {
		for _, name := range observeFromWant(tc.want) {
			fmt.Fprintf(&b, "fmt.Printf(%q, %s)\n", name+"=%v\n", name)
		}
	}
	b.WriteString("}\n")

	return b.String()
}

func TestLanguage(t *testing.T) {
	cases := []langCase{
		// --- SSA / scoping ---------------------------------------------------
		{
			name: "basic arithmetic",
			src:  `x := 5; y := x + 1`,
			want: out("x=5", "y=6"),
		},
		{
			name: "rebinding produces a fresh value",
			src: `
				x := 1
				x := x + 10
			`,
			want: out("x=11"),
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
			want: out("r=10", "s=20"),
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
			want: out("y=1"),
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
			want: out("y=1"),
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
			want: out("y=1"),
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
			want: out("y=1"),
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
			want: out("r=5"),
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
			want: out("a=1", "b=2", "n=2"),
		},
		{
			name: "missing key returns the zero value",
			src: `
				m := map[string]int{"a": 1}
				z := m["missing"]
			`,
			want: out("z=0"),
		},
		{
			name: "two-value index reports presence",
			src: `
				m := map[string]int{"a": 1}
				v1, ok1 := m["a"]
				v2, ok2 := m["x"]
			`,
			want: out("v1=1", "ok1=true", "v2=0", "ok2=false"),
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
			want: out("n=1", "n1=2", "b=2"),
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
			want: out("n=2", "n1=1", "ok=false"),
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
			want: out("a=10", "b=30", "n=3"),
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
			want: out("n=2", "n1=3", "last=3"),
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
			want: out("n=3", "first=20", "last=40"),
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
			want: out("a=3", "b=3", "c=5", "d=0"),
		},

		// --- Builtins --------------------------------------------------------
		{
			name: "fixed-size arrays",
			src: `
				a := [2]int{1, 2}
				v0 := a[0]
				v1 := a[1]
				n := len(a)
			`,
			want: out("v0=1", "v1=2", "n=2"),
		},
		{
			name: "map builtins with mangled names",
			src: `
				m := map[string]int{"a": 1}
				m := delete(m, "a")
				m := set(m, "b", 2)
				m := update(m, "b", func(x int) int { return x + 10 })
				v := get(m, "b")
				n := len(m)
			`,
			want: out("v=12", "n=1"),
		},
		{
			name: "array get/update",
			src: `
				a := [3]int{10, 20, 30}
				v1 := get(a, 1)
				a2 := update(a, 1, func(x int) int { return x + 5 })
				v2 := a2[1]
				a3 := update(a2, 0, func(x int) int { return x + 5 })
				v3 := a3[0]
				n := len(a3)
				orig := a[1]
			`,
			want: out("v1=20", "v2=25", "v3=15", "n=3", "orig=20"),
		},
		{
			name: "append builtin on a list",
			src: `
				l := []int{10, 20}
				l1 := append(l, 30)
				l2 := append(l1, 40, 50)
				n0 := len(l)
				n1 := len(l1)
				n2 := len(l2)
				v0 := l[0]
				v1 := l1[2]
				v2 := l2[4]
			`,
			want: out("n0=2", "n1=3", "n2=5", "v0=10", "v1=30", "v2=50"),
		},
		{
			name: "set/get builtins on a list",
			src: `
				l := []int{10, 20}
				l1 := set(l, 1, 25)
				v := get(l1, 1)
				v0 := l1[0]
			`,
			want: out("v=25", "v0=10"),
		},
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
			want: out("v=1", "w=2", "a=10", "n=1"),
		},
		{
			name: "two-value get builtin reports presence",
			src: `
				m := map[string]int{"a": 1}
				v, ok := get(m, "missing")
			`,
			want: out("v=0", "ok=false"),
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
			want: out("v1=1", "v2=2", "v3=3", "v4=11", "ok=false"),
		},
		{
			name: "builtin shadowed by local function is not rewritten",
			src: `
				set := func(_ any, _ string, v int) int { return v + 100 }
				m := map[string]int{"a": 1}
				x := set(m, "a", 7)
			`,
			want: out("x=107"),
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
			want: out("n=0", "n1=1"),
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
			want: out("n=0", "n1=1", "first=42"),
		},

		// --- Struct builtins -------------------------------------------------
		{
			name: "get/update on a struct lower to field access / IIFE",
			src: `
				type Point struct{ X, Y int }
				p := Point{X: 1, Y: 2}
				a := get(p, "X")
				b := get(p, "Y")
				p2 := update(p, "Y", func(y int) int { return y + 10 })
			`,
			want: out("a=1", "b=2", "p={1 2}", "p2={1 12}"),
		},

		// --- List builtins ---------------------------------------------------
		{
			name: "get/set builtins dispatch to list methods",
			src: `
				l := []int{10, 20, 30}
				v := get(l, 1)
				l2 := set(l, 0, 99)
				first := get(l2, 0)
				orig := get(l, 0)
			`,
			want: out("v=20", "first=99", "orig=10"),
		},

		// --- Nested composite literals (PR6) ---------------------------------
		{
			name: "map of lists: literal creation and element access",
			src: `
				m := map[string][]int{
					"a": {1, 2, 3},
					"b": {4, 5},
				}
				na := m["a"]
				v := na[1]
				nb := len(m)
				nl := len(na)
			`,
			want: out("v=2", "nb=2", "nl=3"),
		},
		{
			name: "list of maps: literal creation and element access",
			src: `
				l := []map[string]int{
					{"a": 1, "b": 2},
					{"c": 3},
				}
				m0 := l[0]
				v := m0["a"]
				n := len(l)
			`,
			want: out("v=1", "n=2"),
		},
		{
			name: "map of structs: implicit struct element gets explicit type",
			src: `
				type Point struct{ X, Y int }
				m := map[string]Point{
					"p": {X: 1, Y: 2},
					"q": {X: 3, Y: 4},
				}
				p := m["p"]
				px := p.X
				py := p.Y
			`,
			want: out("px=1", "py=2"),
		},
		{
			name: "struct with persistent fields: type rewritten, access works",
			src: `
				type Config struct {
					Tags   []string
					Scores map[string]int
				}
				c := Config{
					Tags:   []string{"alpha", "beta"},
					Scores: map[string]int{"x": 10, "y": 20},
				}
				n := len(c.Tags)
				tag := c.Tags[0]
				score := c.Scores["x"]
			`,
			want: out("n=2", "tag=alpha", "score=10"),
		},

		// --- Expression forms ------------------------------------------------
		{
			name: "unary, star, paren exprs and var-with-value decls",
			src: `
				x := 5
				y := -x
				z := (x + 1)
				p := &x
				d := *p
				var v int = 7
				var _ int = 99
				{
					a := x
					_ = a
				}
			`,
			want: out("y=-5", "z=6", "d=5", "v=7"),
		},
		{
			name: "defer runs after the surrounding function returns",
			src: `
				fmt.Println("before")
				defer fmt.Println("deferred")
				fmt.Println("after")
			`,
			raw:  true,
			want: out("before", "after", "deferred"),
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
			want: out("[0]=10", "[1]=20", "[2]=30"),
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
			want: out("captured=10", "shadowed=20"),
		},

		// --- Builtin Edge Cases ----------------------------------------------
		{
			name: "getIn two-value form for maps",
			src: `
				m := map[string]map[string]int{
					"a": {"b": 1},
				}
				v1, ok1 := getIn(m, "a", "b")
				v2, ok2 := getIn(m, "a", "missing")
				v3, ok3 := getIn(m, "missing", "b")
			`,
			want: out("v1=1", "ok1=true", "v2=0", "ok2=false", "v3=0", "ok3=false"),
		},
		{
			name: "list getIn and append multiple",
			src: `
				l := [][]int{{1, 2}, {3, 4}}
				v := getIn(l, 0, 1)
				l2 := append(l, []int{5, 6}, []int{7, 8})
				n := len(l2)
				v2 := getIn(l2, 3, 0)
			`,
			want: out("v=2", "n=4", "v2=7"),
		},
		{
			name: "struct getIn and updateIn",
			src: `
				type Inner struct { V int }
				type Outer struct { I Inner }
				o := Outer{I: Inner{V: 10}}
				v := getIn(o, "I", "V")
				o2 := updateIn(o, "I", "V", func(x int) int { return x + 5 })
				v2 := o2.I.V
			`,
			want: out("v=10", "v2=15"),
		},
	}

	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("abs repo root: %v", err)
	}
	dir := t.TempDir()

	const sep = "\x1c" // ASCII File Separator — not present in any test output

	// Rewrite all cases into per-case files (case0.go, case1.go, …).
	// Separate files mean Go compiler errors include the filename
	// (e.g. "case5.go:12: undefined x") which maps directly to the case index.
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
		err := os.WriteFile(path, []byte(caseSrc), 0o644)
		if err != nil {
			t.Fatalf("write case%d.go: %v", i, err)
		}
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
	if err := os.WriteFile(mainPath, []byte(mb.String()), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	filePaths = append(filePaths, mainPath)

	// One compilation for all cases.
	args := append([]string{"run"}, filePaths...)
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go run failed: %v\nstderr:\n%s\nstdout:\n%s",
			err, stderr.String(), stdout.String())
	}

	// Split on separator; last element is "" (after the final sep).
	parts := strings.Split(stdout.String(), sep)
	if len(parts) < len(cases) {
		t.Fatalf("output split produced %d parts, want at least %d", len(parts), len(cases))
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parts[i]; got != tc.want {
				t.Errorf("output mismatch\n--- want ---\n%s--- got ---\n%s--- generated Go ---\n%s",
					tc.want, got, goSrcs[i])
			}
		})
	}
}
