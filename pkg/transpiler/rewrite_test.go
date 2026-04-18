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

func TestRewrite(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name: "True shadowing with mangling",
			input: `package main
func main() {
	x := 5
	f := func() { fmt.Println(x) }
	x := 10
	f()
}`,
			expected: []string{
				"x_1 := 5",
				"fmt.Println(x_1)",
				"x_2 := 10",
			},
		},
		{
			name: "Map literal and indexing",
			input: `package main
func main() {
	m := map[string]int{"a": 1}
	v := m["a"]
	v, ok := m["b"]
}`,
			expected: []string{
				"m_1 := persistent.NewMap[string, int]().Set(\"a\", 1)",
				"v_1 := m_1.Get(\"a\")",
				"v_2, ok_1 := m_1.Lookup(\"b\")",
			},
		},
		{
			name: "Slice literal and appending",
			input: `package main
func main() {
	l := []int{1, 2}
	l := l.Append(3)
	x := l[0]
}`,
			expected: []string{
				"l_1 := persistent.NewList[int]().Append(1).Append(2)",
				"l_2 := l_1.Append(3)",
				"x_1 := l_2.Get(0)",
			},
		},
		{
			name: "If statement with init and else if",
			input: `package main
func main() {
	if x := 5; x > 0 {
		x := 10
		fmt.Println(x)
	} else if y := 2; y > 0 {
        fmt.Println(y)
    } else {
        fmt.Println("else")
    }
}`,
			expected: []string{
				"if x_1 := 5; x_1 > 0",
				"x_2 := 10",
				"else if y_1 := 2; y_1 > 0",
			},
		},
		{
			name: "Range statement",
			input: `package main
func main() {
	l := []int{1, 2}
	for i, v := range l {
		fmt.Println(i, v)
	}
    var i int
    for i = range l {
        _ = i
    }
    var v int
    for i, v = range l {
        _ = i
        _ = v
    }
    for range l {
    }
}`,
			expected: []string{
				"for i_1, v_1 := range l_1.All()",
				"for i_2 = range l_1.All()",
				"for i_2, v_2 = range l_1.All()",
				"for range l_1.All()",
			},
		},
		{
			name: "SetIn and UpdateIn expansion",
			input: `package main
func main() {
	m := map[string]any{}
	m := m.SetIn("a", "b", 1)
	m := m.UpdateIn("a", "b", func(v any) any { return v })
}`,
			expected: []string{
				"m_2 := m_1.Set(\"a\", m_1.Get(\"a\").Set(\"b\", 1))",
				"m_3 := m_2.Set(\"a\", m_2.Get(\"a\").Update(\"b\", func(v any) any { return v }))",
			},
		},
		{
			name: "Package level var types and non-value specs",
			input: `package main
const Pi = 3.14
type MyInt int
var m map[string]int
var l []int`,
			expected: []string{
				"const Pi = 3.14",
				"type MyInt int",
				"var m persistent.Map[string, int]",
				"var l persistent.List[int]",
			},
		},
		{
			name: "Return statement",
			input: `package main
func f() int {
    x := 5
    return x
}`,
			expected: []string{
				"return x_1",
			},
		},
		{
			name: "Nested blocks and const",
			input: `package main
func main() {
    x := 1
    const y = 2
    {
        x := 2
        fmt.Println(x, y)
    }
    fmt.Println(x)
}`,
			expected: []string{
				"x_1 := 1",
				"const y = 2",
				"x_2 := 2",
				"fmt.Println(x_2, y)",
				"fmt.Println(x_1)",
			},
		},
		{
			name: "Switch and defer and expressions",
			input: `package main
func main() {
    x := 5
    defer fmt.Println(x)
    switch y := (x + 1); y {
    case 6:
        fmt.Println(y)
    default:
        fmt.Println("default")
    }
    s := []int{1, 2, 3}
    s2 := s[1:2]
    var a any = s2
    s3 := a.([]int)
    _ = s[1:]
    _ = s[:2]
    _ = s[:]
}`,
			expected: []string{
				"x_1 := 5",
				"defer fmt.Println(x_1)",
				"switch y_1 := (x_1 + 1); y_1",
				"case 6:",
				"fmt.Println(y_1)",
				"s_1 := persistent.NewList[int]().Append(1).Append(2).Append(3)",
				"s2_1 := persistent.Slice(s_1, 1, 2)",
				"var a_1 any = s2_1",
				"s3_1 := a_1.(persistent.List[int])",
				"persistent.Slice(s_1, 1, persistent.Len(s_1))",
				"persistent.Slice(s_1, 0, 2)",
				"persistent.Slice(s_1, 0, persistent.Len(s_1))",
			},
		},
		{
			name: "Type switch with Init",
			input: `package main
func main() {
    var a any
    switch x := 1; v := a.(type) {
    case int:
        fmt.Println(v, x)
    }
}`,
			expected: []string{
				"switch x_1 := 1; v_1 := a_1.(type)",
			},
		},
		{
			name: "Delete and DeleteIn expansion",
			input: `package main
func main() {
    m := map[string]any{"a": 1}
    m := m.Delete("a")
    m := m.DeleteIn("a", "b")
    m := m.DeleteIn("a")
}`,
			expected: []string{
				"m_2 := m_1.Delete(\"a\")",
				"m_3 := m_2.Set(\"a\", m_2.Get(\"a\").Delete(\"b\"))",
				"m_4 := m_3.Delete(\"a\")",
			},
		},
		{
			name: "No persistent needed",
			input: `package main
const X = 1
type Y int
`,
			expected: []string{
				"package main",
				"const X = 1",
				"type Y int",
			},
		},
		{
			name: "Mixed imports",
			input: `package main
import "os"
func main() {
    m := map[string]int{}
    fmt.Println(m)
}`,
			expected: []string{
				`import (`,
				`"os"`,
				`"github.com/rg/imgo/pkg/persistent"`,
			},
		},
		{
			name: "Import already exists",
			input: `package main
import "github.com/rg/imgo/pkg/persistent"
func main() {
	m := map[string]int{}
    _ = m
}
`,
			expected: []string{
				`import "github.com/rg/imgo/pkg/persistent"`,
			},
		},
		{
			name: "Fixed array desugaring",
			input: `package main
var a [5]int
func main() {
    x := a[0]
}`,
			expected: []string{
				"var a [5]int",
				"x_1 := a.Get(0)",
			},
		},
		{
			name: "Complex type assertion",
			input: `package main
func main() {
    var a any
    x := a.(map[string]int)
}`,
			expected: []string{
				"x_1 := a_1.(persistent.Map[string, int])",
			},
		},
		{
			name: "UpdateIn with 1 arg",
			input: `package main
func main() {
    m := map[string]any{}
    m = m.UpdateIn("a", func(v any) any { return v })
}`,
			expected: []string{
				"m_1.Update(\"a\", func(v any) any { return v })",
			},
		},
		{
			name: "Special cases for coverage",
			input: `package main
var m = map[string]int{}
func main() {
    len()
    len(1, 2)
    var a, b int
    _, _ = a, b
    for { break }
    x := 1
    type S struct { X int }
    _ = S{X: x}
}
`,
			expected: []string{
				"var m = persistent.NewMap[string, int]()",
				"len()",
				"len(1, 2)",
				"var a_1, b_1 int",
				"for {",
				"break",
				"x_1 := 1",
				"S{X: x_1}",
			},
		},
		{
			name: "Struct literal bare elements must be rewritten",
			input: `package main
type S struct { X, Y int }
func main() {
    x := 5
    y := 10
    s := S{x, y}
}`,
			expected: []string{
				"s_1 := S{x_1, y_1}",
			},
		},
		{
			name: "Make builtin desugaring",
			input: `package main
func main() {
	m := make(map[string]int)
	l := make([]int, 10)
}`,
			expected: []string{
				"m_1 := persistent.NewMap[string, int]()",
				"l_1 := persistent.NewList[int]()",
			},
		},


		{
			name: "UnaryExpr operands must be rewritten",
			input: `package main
func main() {
    x := 5
    y := -x
    b := true
    c := !b
    p := &x
}`,
			expected: []string{
				"y_1 := -x_1",
				"c_1 := !b_1",
				"p_1 := &x_1",
			},
		},
		{
			name: "StarExpr operand must be rewritten",
			input: `package main
func main() {
    x := 5
    p := &x
    y := *p
}`,
			expected: []string{
				"y_1 := *p_1",
			},
		},
		{
			name: "ForStmt condition must be rewritten",
			input: `package main
func main() {
    x := true
    for x {
        break
    }
}`,
			expected: []string{
				"for x_1",
			},
		},
		{
			name: "ForStmt body must be rewritten",
			input: `package main
func main() {
    for {
        x := 5
        fmt.Println(x)
        break
    }
}`,
			expected: []string{
				"x_1 := 5",
				"fmt.Println(x_1)",
			},
		},
		{
			name: "Struct literal values must be rewritten",
			input: `package main
type S struct { X int }
func main() {
    x := 5
    s := S{X: x}
}`,
			expected: []string{
				"s_1 := S{X: x_1}",
			},
		},
		{
			name: "IfStmt init must not leak into parent scope",
			input: `package main
func main() {
    x := 1
    if x := 2; x > 0 {
    }
    fmt.Println(x)
}`,
			expected: []string{
				"fmt.Println(x_1)",
			},
		},
		{
			name: "SwitchStmt init must not leak into parent scope",
			input: `package main
func main() {
    x := 1
    switch x := 2; {
    }
    fmt.Println(x)
}`,
			expected: []string{
				"fmt.Println(x_1)",
			},
		},
		{
			name: "User struct with SetIn method is not rewritten",
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
			expected: []string{
				"r1_1 := s_1.SetIn(1, 2, 3)",
				"r2_1 := s_1.UpdateIn(1, 2, func(v int) int",
				"r3_1 := s_1.DeleteIn(1, 2)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "test.im", tt.input, 0)
			if err != nil {
				t.Fatalf("Failed to parse input code: %v", err)
			}

			info := typeCheck(fset, f)
			f = Rewrite(f, info)

			var buf bytes.Buffer
			_ = printer.Fprint(&buf, fset, f)
			got := buf.String()

			for _, exp := range tt.expected {
				if !strings.Contains(got, exp) {
					t.Errorf("Rewrite() output missing %q. Got:\n%s", exp, got)
				}
			}
		})
	}
}

func TestRewriteEdgeCases(t *testing.T) {
	hp := false
	hasPersistent := &hp

	// Test rewriteBlock(nil)
	rewriteBlock(nil, nil, make(map[string]int), nil, hasPersistent)

	// Test rewriteExpr(nil)
	if rewriteExpr(nil, nil, make(map[string]int), false, nil, hasPersistent) != nil {
		t.Errorf("Expected nil for rewriteExpr(nil)")
	}

	// Test setPos with token.NoPos
	ident := ast.NewIdent("x")
	if setPos(ident, token.NoPos) != ident {
		t.Errorf("Expected unchanged ident for NoPos")
	}

	// Test setPos Ident
	setPos(ast.NewIdent("x"), token.Pos(1))

	// Test setPos CallExpr
	setPos(&ast.CallExpr{}, token.Pos(1))

	// Test setPos SelectorExpr with non-settable X
	sel := &ast.SelectorExpr{
		X:   &ast.BasicLit{Kind: token.INT, Value: "1"},
		Sel: ast.NewIdent("Foo"),
	}
	setPos(sel, token.Pos(1))

	// Test setPos IndexListExpr
	setPos(&ast.IndexListExpr{}, token.Pos(1))

	// Test addPersistentImport with no existing imports GenDecl
	f := &ast.File{
		Decls: []ast.Decl{
			&ast.FuncDecl{
				Name: ast.NewIdent("main"),
				Body: &ast.BlockStmt{},
			},
		},
	}
	addPersistentImport(f)

	// Test rewriteType(nil)
	if rewriteType(nil, hasPersistent) != nil {
		t.Errorf("Expected nil for rewriteType(nil)")
	}

	// Test LHS not Ident in DEFINE
	stmt := &ast.AssignStmt{
		Lhs: []ast.Expr{&ast.BinaryExpr{}},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "1"}},
	}
	rewriteStmt(stmt, []map[string]string{make(map[string]string)}, make(map[string]int), nil, hasPersistent)

	// Test SwitchStmt with Init
	sw := &ast.SwitchStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent("x")},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "1"}},
		},
		Tag:  ast.NewIdent("x"),
		Body: &ast.BlockStmt{},
	}
	rewriteStmt(sw, []map[string]string{make(map[string]string)}, make(map[string]int), nil, hasPersistent)

	// Test IfStmt with Else as non-block (else if)
	ifStmt := &ast.IfStmt{
		Cond: &ast.Ident{Name: "true"},
		Body: &ast.BlockStmt{},
		Else: &ast.IfStmt{
			Cond: &ast.Ident{Name: "false"},
			Body: &ast.BlockStmt{},
		},
	}
	rewriteStmt(ifStmt, []map[string]string{make(map[string]string)}, make(map[string]int), nil, hasPersistent)

	// Test RangeStmt with nil Key/Value
	rangeStmt := &ast.RangeStmt{
		X:    &ast.Ident{Name: "l"},
		Body: &ast.BlockStmt{},
	}
	rewriteStmt(rangeStmt, []map[string]string{make(map[string]string)}, make(map[string]int), nil, hasPersistent)

	// Test TypeSwitchStmt with Init
	tsStmt := &ast.TypeSwitchStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent("x")},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "1"}},
		},
		Assign: &ast.ExprStmt{X: &ast.TypeAssertExpr{X: ast.NewIdent("a"), Type: nil}},
		Body:   &ast.BlockStmt{},
	}
	rewriteStmt(tsStmt, []map[string]string{make(map[string]string)}, make(map[string]int), nil, hasPersistent)

	// Test IndexExpr with nil Index
	idxExpr := &ast.IndexExpr{
		X:     ast.NewIdent("a"),
		Index: nil,
	}
	rewriteExpr(idxExpr, []map[string]string{make(map[string]string)}, make(map[string]int), false, nil, hasPersistent)

	// Test CompositeLit with non-KeyValueExpr elements
	compLit := &ast.CompositeLit{
		Type: ast.NewIdent("S"),
		Elts: []ast.Expr{ast.NewIdent("x")},
	}
	rewriteExpr(compLit, []map[string]string{{"x": "x_1"}}, make(map[string]int), false, nil, hasPersistent)

	// Test ForStmt with nil Init, Cond, Post (for { ... })
	forNilStmt := &ast.ForStmt{
		Body: &ast.BlockStmt{},
	}
	rewriteStmt(forNilStmt, []map[string]string{make(map[string]string)}, make(map[string]int), nil, hasPersistent)

	// Test ForStmt with non-nil Init, Cond, Post
	forFullStmt := &ast.ForStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent("i")},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "0"}},
		},
		Cond: &ast.BinaryExpr{
			X:  ast.NewIdent("i"),
			Op: token.LSS,
			Y:  &ast.BasicLit{Kind: token.INT, Value: "10"},
		},
		Post: &ast.ExprStmt{X: ast.NewIdent("i")},
		Body: &ast.BlockStmt{},
	}
	rewriteStmt(forFullStmt, []map[string]string{make(map[string]string)}, make(map[string]int), nil, hasPersistent)

	// Test make with fixed array (should not be rewritten)
	makeArr := &ast.CallExpr{
		Fun:  ast.NewIdent("make"),
		Args: []ast.Expr{&ast.ArrayType{Len: &ast.BasicLit{Kind: token.INT, Value: "5"}, Elt: ast.NewIdent("int")}},
	}
	rewriteExpr(makeArr, nil, make(map[string]int), false, nil, hasPersistent)

	// Test setPos(nil)
	if setPos(nil, token.Pos(1)) != nil {
		t.Errorf("expected nil")
	}
}
