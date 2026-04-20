package transpiler

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"
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
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
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
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in output:\n%s", w, got)
		}
	}
}

// TestRewrite focuses on the *shape* of the rewriter's Go output — mangled
// identifier names, method choice (Get vs Lookup), desugaring to persistent
// calls. Semantic coverage (does the generated Go actually produce the right
// result at runtime?) lives in TestLanguage.
func TestRewrite(t *testing.T) {
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
				`m_3 := m_2.Set("a", m_2.Get("a").Update("b", func(v any) any { return v }))`,
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
			name: "fixed-size array is not rewritten to persistent.List",
			input: `package main
var a [5]int
func main() {
	x := a[0]
}`,
			want: []string{"var a [5]int", "x_1 := a.Get(0)"},
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
			want: []string{`m_1.Update("a", func(v any) any { return v })`},
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
				`m2_1 := m1_1.Update("a", func(x int) int { return x + 1 })`,
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
			want: []string{`x_1 := set_1(m_1, "b", 2)`},
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
				"r2_1 := s_1.UpdateIn(1, 2, func(v int) int",
				"r3_1 := s_1.DeleteIn(1, 2)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertContainsAll(t, rewriteSrc(t, tt.input), tt.want...)
		})
	}
}

// TestRewriteEdgeCases exercises paths that an end-to-end runner can't reach
// — nil inputs, synthetic AST nodes built by hand, coverage corners. These
// drive the rewriter directly rather than through ImGo source.
func TestRewriteEdgeCases(t *testing.T) {
	newR := func() *rewriter {
		return &rewriter{
			env:      []map[string]string{make(map[string]string)},
			versions: make(map[string]int),
		}
	}

	// nil block / nil expr
	newR().block(nil)
	if got := newR().expr(nil, false); got != nil {
		t.Errorf("expr(nil) = %v, want nil", got)
	}

	// setPos corners
	ident := ast.NewIdent("x")
	if setPos(ident, token.NoPos) != ident {
		t.Errorf("setPos(ident, NoPos) should return ident unchanged")
	}
	setPos(ast.NewIdent("x"), token.Pos(1))
	setPos(&ast.CallExpr{}, token.Pos(1))
	setPos(&ast.SelectorExpr{
		X:   &ast.BasicLit{Kind: token.INT, Value: "1"},
		Sel: ast.NewIdent("Foo"),
	}, token.Pos(1))
	setPos(&ast.IndexListExpr{}, token.Pos(1))
	if setPos(nil, token.Pos(1)) != nil {
		t.Errorf("setPos(nil) should return nil")
	}

	// addPersistentImport when no import decl exists yet
	addPersistentImport(&ast.File{
		Decls: []ast.Decl{
			&ast.FuncDecl{Name: ast.NewIdent("main"), Body: &ast.BlockStmt{}},
		},
	})

	// nil type returns nil
	if newR().typ(nil) != nil {
		t.Errorf("typ(nil) should return nil")
	}

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
