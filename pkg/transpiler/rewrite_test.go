package transpiler

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wrap auto-wraps a bare body snippet with package+main unless the snippet
// already starts with a "package " declaration.
func wrap(src string) string {
	if strings.HasPrefix(strings.TrimSpace(src), "package") {
		return src
	}

	return "package main\nfunc main() {\n" + src + "\n}\n"
}

// rewriteSrc runs the full parse → typecheck → Rewrite → print pipeline and
// returns the rewritten Go source text.
func rewriteSrc(t *testing.T, src string) string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.im", wrap(src), 0)
	require.NoError(t, err, "parse")

	f = Rewrite(f, typeCheck(fset, f))
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, fset, f)

	return buf.String()
}

// assertContainsAll reports one error per missing substring, preserving the
// full output for diagnosis.
func assertContainsAll(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		assert.Contains(t, got, w)
	}
}

// TestRewrite focuses on the *shape* of the rewriter's Go output — mangled
// identifier names, method choice (Get vs Lookup), desugaring to persistent
// Semantic coverage (does the generated Go actually produce the right
// result at runtime?) lives in TestLanguage.
func TestRewrite(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name: "shadowed binding gets a fresh version",
			input: `
				x := 5
				f := func() { fmt.Println(x) }
				x := 10
				f()
			`,
			want: []string{"x_1 := 5", "fmt.Println(x_1)", "x_2 := 10"},
		},
		{
			name: "map literal and two-value index pick Get vs Lookup",
			input: `
				m := map[string]int{"a": 1}
				v := m["a"]
				v, ok := m["b"]
			`,
			want: []string{
				`m_1 := persistent.NewMap[string, int]().Set("a", 1)`,
				`v_1 := m_1.Get("a")`,
				`v_2, ok_1 := m_1.Lookup("b")`,
			},
		},
		{
			name: "slice literal and indexing become persistent ops",
			input: `
				l := []int{1, 2}
				l := l.Append(3)
				x := l[0]
			`,
			want: []string{
				"l_1 := persistent.NewList[int]().Append(1).Append(2)",
				"l_2 := l_1.Append(3)",
				"x_1 := l_2.Get(0)",
			},
		},
		{
			name: "append builtin lowers to method calls",
			input: `
				l := []int{1}
				l := append(l, 2)
				l := append(l, 3, 4)
			`,
			want: []string{
				"l_1 := persistent.NewList[int]().Append(1)",
				"l_2 := l_1.Append(2)",
				"l_3 := l_2.Append(3).Append(4)",
			},
		},
		{
			name: "fixed-size array make is left alone",
			input: `
				a := make([5]int)
			`,
			want: []string{"a_1 := make([5]int)"},
		},
		{
			name: "unsupported method on list is left alone",
			input: `
				l := []int{1}
				l.NoMethod(0)
			`,
			want: []string{"l_1 := persistent.NewList[int]().Append(1)", "l_1.NoMethod(0)"},
		},
		{
			name: "make with no args is left alone",
			input: `
				x := make()
			`,
			want: []string{"x_1 := make()"},
		},
		{
			name: "make with non-map/list type is left alone",
			input: `
				x := make(int)
			`,
			want: []string{"x_1 := make(int)"},
		},
		{
			name: "dynamic get on struct is left alone",
			input: `
				type S struct { F int }
				var s S
				f := "F"
				x := get(s, f)
			`,
			want: []string{"x_1 := get(s_1, f_1)"},
		},
		{
			name: "In-update method calls on map",
			input: `
				m := map[string]map[string]int{}
				m1 := m.SetIn("a", "b", 1)
				m2 := m1.UpdateIn("a", "b", func(x int) int { return x + 1 })
				m3 := m2.DeleteIn("a", "b")
			`,
			want: []string{
				"m1_1 := m_1.Set(\"a\", m_1.Get(\"a\").Set(\"b\", 1))",
				"m2_1 := m1_1.Set(\"a\", m1_1.Get(\"a\").Update(\"b\", func(x_1 int) int { return x_1 + 1 }))",
				"m3_1 := m2_1.Set(\"a\", m2_1.Get(\"a\").Delete(\"b\"))",
			},
		},
		{
			name: "non-selector call is left alone",
			input: `
				func() {}()
			`,
			want: []string{"func() {}()"},
		},
		{
			name: "unsupported method name on map is left alone",
			input: `
				m := map[string]int{}
				m.NonExistent(1)
			`,
			want: []string{"m_1.NonExistent(1)"},
		},
		{
			name: "make with slice type is rewritten",
			input: `
				s := make([]int, 10)
			`,
			want: []string{"s_1 := persistent.NewList[int]()"},
		},
		{
			name: "In-updates with insufficient args are left alone",
			input: `
				var m map[string]int
				m1 := m.SetIn()
				m1 := m.SetIn("a")
				m2 := m.UpdateIn()
				m2 := m.UpdateIn("a")
				m3 := m.DeleteIn()
			`,
			want: []string{
				"m1_1 := m_1.SetIn()",
				"m1_2 := m_1.SetIn(\"a\")",
				"m2_1 := m_1.UpdateIn()",
				"m2_2 := m_1.UpdateIn(\"a\")",
				"m3_1 := m_1.DeleteIn()",
			},
		},
		{
			name: "if/else-if init binds don't leak out",
			input: `
				if x := 5; x > 0 {
					x := 10
					fmt.Println(x)
				} else if y := 2; y > 0 {
					fmt.Println(y)
				} else {
					fmt.Println("else")
				}
			`,
			want: []string{"if x_1 := 5; x_1 > 0", "x_2 := 10", "else if y_1 := 2; y_1 > 0"},
		},
		{
			name: "range desugars to .All() across define/assign/blank forms",
			input: `
				l := []int{1, 2}
				for i, v := range l { fmt.Println(i, v) }
				var i int
				for i = range l { _ = i }
				var v int
				for i, v = range l { _, _ = i, v }
				for range l {}
			`,
			want: []string{
				"for i_1, v_1 := range l_1.All()",
				"for i_2 = range l_1.All()",
				"for i_2, v_2 = range l_1.All()",
				"for range l_1.All()",
			},
		},
		{
			name: "SetIn/UpdateIn expand to Set(Get(...)) chains",
			input: `
				m := map[string]any{}
				m := m.SetIn("a", "b", 1)
				m := m.UpdateIn("a", "b", func(v any) any { return v })
			`,
			want: []string{
				`m_2 := m_1.Set("a", m_1.Get("a").Set("b", 1))`,
				`m_3 := m_2.Set("a", m_2.Get("a").Update("b", func(v_1 any) any { return v_1 }))`,
			},
		},
		{
			name: "package-level decls pass through and retype",
			input: `package main
const Pi = 3.14
type MyInt int
var m map[string]int
var l []int
`,
			want: []string{
				"const Pi = 3.14",
				"type MyInt int",
				"var m persistent.Map[string, int]",
				"var l persistent.List[int]",
			},
		},
		{
			name: "package-level var initializer is rewritten",
			input: `package main
var m = map[string]int{"a": 1}
`,
			want: []string{
				`var m = persistent.NewMap[string, int]().Set("a", 1)`,
			},
		},
		{
			name: "type switch with Init clause",
			input: `
				var a any
				switch x := 1; v := a.(type) {
				case int:
					fmt.Println(v, x)
				}
			`,
			want: []string{"switch x_1 := 1; v_1 := a_1.(type)"},
		},
		{
			name: "DeleteIn expands through Set(Get(...)).Delete",
			input: `
				m := map[string]any{"a": 1}
				m := m.Delete("a")
				m := m.DeleteIn("a", "b")
				m := m.DeleteIn("a")
			`,
			want: []string{
				`m_2 := m_1.Delete("a")`,
				`m_3 := m_2.Set("a", m_2.Get("a").Delete("b"))`,
				`m_4 := m_3.Delete("a")`,
			},
		},
		{
			name: "unchanged when nothing needs persistent",
			input: `package main
const X = 1
type Y int
`,
			want: []string{"package main", "const X = 1", "type Y int"},
		},
		{
			name: "persistent import merges into an existing import block",
			input: `package main
import "os"
func main() {
	m := map[string]int{}
	fmt.Println(m)
}`,
			want: []string{"import (", `"os"`, `"github.com/rg/imgo/pkg/persistent"`},
		},
		{
			name: "persistent import is not duplicated",
			input: `package main
import "github.com/rg/imgo/pkg/persistent"
func main() {
	m := map[string]int{}
	_ = m
}
`,
			want: []string{`import "github.com/rg/imgo/pkg/persistent"`},
		},
		{
			name: "array method call expands to IIFE",
			input: `
				a := [3]int{1, 2, 3}
				a2 := a.Update(1, func(x int) int { return x + 5 })
			`,
			want: []string{
				"a_1 := [3]int{1, 2, 3}",
				"a2_1 := func(__a [3]int) [3]int",
			},
		},
		{
			name: "fixed-size array is not rewritten to persistent.List",
			input: `package main
var a [5]int
func main() {
	x := a[0]
}`,
			want: []string{"var a [5]int", "x_1 := a[0]"},
		},
		{
			name: "type assertion target gets retyped",
			input: `
				var a any
				x := a.(map[string]int)
			`,
			want: []string{"x_1 := a_1.(persistent.Map[string, int])"},
		},
		{
			name: "UpdateIn with one key expands to a plain Update",
			input: `
				m := map[string]any{}
				m = m.UpdateIn("a", func(v any) any { return v })
			`,
			want: []string{`m_1.Update("a", func(v_1 any) any { return v_1 })`},
		},
		{
			name: "builtin set/get/update/delete on map",
			input: `
				m := map[string]int{"a": 1}
				m1 := set(m, "b", 2)
				v := get(m1, "a")
				m2 := update(m1, "a", func(x int) int { return x + 1 })
				m3 := delete(m2, "b")
				println(v, m3)
			`,
			want: []string{
				`m_1 := persistent.NewMap[string, int]().Set("a", 1)`,
				`m1_1 := m_1.Set("b", 2)`,
				`v_1 := m1_1.Get("a")`,
				`m2_1 := m1_1.Update("a", func(x_1 int) int { return x_1 + 1 })`,
				`m3_1 := m2_1.Delete("b")`,
			},
		},
		{
			name: "two-value form of get picks Lookup",
			input: `
				m := map[string]int{"a": 1}
				v, ok := get(m, "a")
				println(v, ok)
			`,
			want: []string{`v_1, ok_1 := m_1.Lookup("a")`},
		},
		{
			name: "getIn chains Get; two-value tail is Lookup",
			input: `
				m := map[string]map[string]int{"a": map[string]int{"b": 1}}
				v := getIn(m, "a", "b")
				v2, ok := getIn(m, "a", "b")
				println(v, v2, ok)
			`,
			want: []string{
				`v_1 := m_1.Get("a").Get("b")`,
				`v2_1, ok_1 := m_1.Get("a").Lookup("b")`,
			},
		},
		{
			name: "builtin shadowed by local is left alone",
			input: `
				set := func(m map[string]int, k string, v int) int { return v }
				m := map[string]int{"a": 1}
				x := set(m, "b", 2)
				println(x)
			`,
			want: []string{`x_1 := set_1(m_2, "b", 2)`},
		},
		// --- Nested composite literals (PR6) ------------------------------------
		{
			name: "map-of-list: implicit list element is rewritten to NewList",
			input: `
				m := map[string][]int{
					"a": {1, 2},
				}
			`,
			want: []string{
				`persistent.NewMap[string, persistent.List[int]]()`,
				`.Set("a", persistent.NewList[int]().Append(1).Append(2))`,
			},
		},
		{
			name: "list-of-map: implicit map element is rewritten to NewMap",
			input: `
				l := []map[string]int{
					{"x": 1},
				}
			`,
			want: []string{
				`persistent.NewList[persistent.Map[string, int]]()`,
				`.Append(persistent.NewMap[string, int]().Set("x", 1))`,
			},
		},
		{
			name: "map-of-struct: implicit struct element gets explicit type added",
			input: `
				type Point struct{ X, Y int }
				m := map[string]Point{
					"p": {X: 1, Y: 2},
				}
			`,
			want: []string{
				`persistent.NewMap[string, Point]()`,
				`.Set("p", Point{X: 1, Y: 2})`,
			},
		},
		{
			name: "struct with map/list fields has field types rewritten",
			input: `package main
type Config struct {
	Tags   []string
	Scores map[string]int
}
func main() { c := Config{}; _ = c }
`,
			want: []string{
				"Tags\tpersistent.List[string]",
				"Scores\tpersistent.Map[string, int]",
			},
		},
		{
			name: "user-defined *In methods on a struct stay intact",
			input: `package main
type S struct{ n int }
func (s S) SetIn(a, b, v int) S { return s }
func (s S) UpdateIn(a, b int, f func(int) int) S { return s }
func (s S) DeleteIn(a, b int) S { return s }
func main() {
	s := S{}
	r1 := s.SetIn(1, 2, 3)
	r2 := s.UpdateIn(1, 2, func(v int) int { return v })
	r3 := s.DeleteIn(1, 2)
	fmt.Println(r1, r2, r3)
}`,
			want: []string{
				"r1_1 := s_1.SetIn(1, 2, 3)",
				"r2_1 := s_1.UpdateIn(1, 2, func(v_2 int) int",
				"r3_1 := s_1.DeleteIn(1, 2)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertContainsAll(t, rewriteSrc(t, tt.input), tt.want...)
		})
	}
}

// TestRewriteEdgeCases exercises paths that an end-to-end runner can't reach
// — nil inputs, synthetic AST nodes built by hand, coverage corners. These
// drive the rewriter directly rather than through ImGo source.
func TestRewriteEdgeCases(t *testing.T) {
	t.Parallel()
	newR := func() *rewriter {
		return &rewriter{
			env:      []map[string]string{make(map[string]string)},
			versions: make(map[string]int),
			types:    make(map[string]types.Type),
		}
	}

	// nil block / nil expr
	newR().block(nil)
	got, _ := newR().expr(nil, false)
	assert.Nil(t, got)

	// setPos corners
	ident := ast.NewIdent("x")
	assert.Equal(t, ident, setPos(ident, token.NoPos))

	assert.NotNil(t, setPos(ast.NewIdent("x"), token.Pos(1)))
	assert.NotNil(t, setPos(&ast.CallExpr{}, token.Pos(1)))
	assert.NotNil(t, setPos(&ast.SelectorExpr{
		X:   &ast.BasicLit{Kind: token.INT, Value: "1"},
		Sel: ast.NewIdent("Foo"),
	}, token.Pos(1)))
	assert.NotNil(t, setPos(&ast.IndexListExpr{}, token.Pos(1)))
	assert.Nil(t, setPos(nil, token.Pos(1)))

	// addPersistentImport when no import decl exists yet
	addPersistentImport(&ast.File{
		Decls: []ast.Decl{
			&ast.FuncDecl{Name: ast.NewIdent("main"), Body: &ast.BlockStmt{}},
		},
	})

	// nil type returns nil
	assert.Nil(t, newR().typ(nil))

	// LHS that isn't an Ident in a DEFINE — leave it alone
	newR().stmt(&ast.AssignStmt{
		Lhs: []ast.Expr{&ast.BinaryExpr{}},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "1"}},
	})

	// SwitchStmt with Init
	newR().stmt(&ast.SwitchStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent("x")},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "1"}},
		},
		Tag:  ast.NewIdent("x"),
		Body: &ast.BlockStmt{},
	})

	// IfStmt with non-block Else (else if)
	newR().stmt(&ast.IfStmt{
		Cond: &ast.Ident{Name: "true"},
		Body: &ast.BlockStmt{},
		Else: &ast.IfStmt{
			Cond: &ast.Ident{Name: "false"},
			Body: &ast.BlockStmt{},
		},
	})

	// RangeStmt with nil Key/Value
	newR().stmt(&ast.RangeStmt{
		X:    &ast.Ident{Name: "l"},
		Body: &ast.BlockStmt{},
	})

	// TypeSwitchStmt with Init
	newR().stmt(&ast.TypeSwitchStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent("x")},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "1"}},
		},
		Assign: &ast.ExprStmt{X: &ast.TypeAssertExpr{X: ast.NewIdent("a"), Type: nil}},
		Body:   &ast.BlockStmt{},
	})

	// IndexExpr with nil Index
	newR().expr(&ast.IndexExpr{X: ast.NewIdent("a"), Index: nil}, false)

	// CompositeLit with non-KeyValueExpr elements
	r := newR()
	r.env[0]["x"] = "x_1"
	r.expr(&ast.CompositeLit{
		Type: ast.NewIdent("S"),
		Elts: []ast.Expr{ast.NewIdent("x")},
	}, false)

	// Implicit-type CompositeLit with no type info falls through to general case
	newR().expr(&ast.CompositeLit{
		Elts: []ast.Expr{ast.NewIdent("x")},
	}, false)

	// Type DeclStmt with a non-struct type is left alone
	newR().stmt(&ast.DeclStmt{
		Decl: &ast.GenDecl{
			Tok: token.TYPE,
			Specs: []ast.Spec{
				&ast.TypeSpec{Name: ast.NewIdent("T"), Type: ast.NewIdent("int")},
			},
		},
	})

	// ForStmt in both bare and fully-populated shapes
	newR().stmt(&ast.ForStmt{Body: &ast.BlockStmt{}})
	newR().stmt(&ast.ForStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent("i")},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "0"}},
		},
		Cond: &ast.BinaryExpr{
			X: ast.NewIdent("i"), Op: token.LSS,
			Y: &ast.BasicLit{Kind: token.INT, Value: "10"},
		},
		Post: &ast.ExprStmt{X: ast.NewIdent("i")},
		Body: &ast.BlockStmt{},
	})

	// make with a fixed-size array argument (not rewritten)
	newR().expr(&ast.CallExpr{
		Fun: ast.NewIdent("make"),
		Args: []ast.Expr{
			&ast.ArrayType{Len: &ast.BasicLit{Kind: token.INT, Value: "5"}, Elt: ast.NewIdent("int")},
		},
	}, false)
}
