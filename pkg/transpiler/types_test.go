package transpiler

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

func TestIsMapLike(t *testing.T) {
	dummy := ast.NewIdent("x")

	if !isMapLike(nil, dummy) {
		t.Errorf("isMapLike(nil, _) = false, want true (fallback)")
	}

	emptyInfo := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue)}
	if !isMapLike(emptyInfo, dummy) {
		t.Errorf("isMapLike(empty, _) = false, want true (fallback)")
	}

	src := `package main
type S struct{}
func main() {
    s := S{}
    m := map[string]int{}
    _, _ = s, m
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info := typeCheck(fset, f)

	var sIdent, mIdent *ast.Ident
	ast.Inspect(f, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if id.Name == "s" && sIdent == nil {
			if _, has := info.Types[id]; has {
				sIdent = id
			}
		}
		if id.Name == "m" && mIdent == nil {
			if _, has := info.Types[id]; has {
				mIdent = id
			}
		}

		return true
	})

	if sIdent == nil || mIdent == nil {
		t.Fatalf("could not locate typed idents: s=%v m=%v", sIdent, mIdent)
	}
	if isMapLike(info, sIdent) {
		t.Errorf("isMapLike struct ident = true, want false")
	}
	if !isMapLike(info, mIdent) {
		t.Errorf("isMapLike map ident = false, want true")
	}
}

func TestIsArrayLike(t *testing.T) {
	dummy := ast.NewIdent("x")

	if isArrayLike(nil, dummy) {
		t.Errorf("isArrayLike(nil, _) = true, want false (fallback)")
	}

	src := `package main
func main() {
    a := [3]int{1, 2, 3}
    s := []int{1, 2, 3}
    _, _ = a, s
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info := typeCheck(fset, f)

	var aIdent, sIdent *ast.Ident
	ast.Inspect(f, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if id.Name == "a" && aIdent == nil {
			if _, has := info.Types[id]; has {
				aIdent = id
			}
		}
		if id.Name == "s" && sIdent == nil {
			if _, has := info.Types[id]; has {
				sIdent = id
			}
		}

		return true
	})

	if aIdent == nil || sIdent == nil {
		t.Fatalf("could not locate typed idents: a=%v s=%v", aIdent, sIdent)
	}
	if !isArrayLike(info, aIdent) {
		t.Errorf("isArrayLike array ident = false, want true")
	}
	if isArrayLike(info, sIdent) {
		t.Errorf("isArrayLike slice ident = true, want false")
	}
}

func TestTypeExprFor(t *testing.T) {
	pkg := types.NewPackage("main", "main")
	typeName := types.NewTypeName(token.NoPos, pkg, "S", nil)
	named := types.NewNamed(typeName, types.NewStruct(nil, nil), nil)

	typeNameNoPkg := types.NewTypeName(token.NoPos, nil, "NoPkg", nil)
	namedNoPkg := types.NewNamed(typeNameNoPkg, types.NewStruct(nil, nil), nil)

	// Type from another package to demonstrate the cross-package issue
	// otherPkg := types.NewPackage("example.com/other", "other")
	// otherTypeName := types.NewTypeName(token.NoPos, otherPkg, "OtherType", nil)
	// namedOtherPkg := types.NewNamed(otherTypeName, types.NewStruct(nil, nil), nil)

	cases := []struct {
		name string
		typ  types.Type
		want string
	}{
		{"nil", nil, ""},
		{"basic int", types.Typ[types.Int], "int"},
		{"basic string", types.Typ[types.String], "string"},
		{"named type", named, "S"},
		{"named type no pkg", namedNoPkg, "NoPkg"},
		// TODO: This test demonstrates the known issue in typeExprFor.
		// It currently returns "OtherType" instead of the expected "other.OtherType".
		// {"named type other pkg", namedOtherPkg, "other.OtherType"},
		{"pointer to int", types.NewPointer(types.Typ[types.Int]), "*int"},
		{"pointer to named", types.NewPointer(named), "*S"},
		{"slice of int", types.NewSlice(types.Typ[types.Int]), "[]int"},
		{"map string to int", types.NewMap(types.Typ[types.String], types.Typ[types.Int]), "map[string]int"},
		{"array of 5 int", types.NewArray(types.Typ[types.Int], 5), "[5]int"},
		{"unsupported chan", types.NewChan(types.SendRecv, types.Typ[types.Int]), ""},
		{"pointer to unsupported", types.NewPointer(types.NewChan(types.SendRecv, types.Typ[types.Int])), ""},
		{"slice of unsupported", types.NewSlice(types.NewChan(types.SendRecv, types.Typ[types.Int])), ""},
		{
			"map of unsupported key",
			types.NewMap(types.NewChan(types.SendRecv, types.Typ[types.Int]), types.Typ[types.Int]),
			"",
		},
		{
			"map of unsupported val",
			types.NewMap(types.Typ[types.String], types.NewChan(types.SendRecv, types.Typ[types.Int])),
			"",
		},
		{"array of unsupported", types.NewArray(types.NewChan(types.SendRecv, types.Typ[types.Int]), 3), ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expr := typeExprFor(tc.typ)
			if tc.want == "" {
				if expr != nil {
					t.Errorf("expected nil for %s, got %T", tc.name, expr)
				}

				return
			}
			if expr == nil {
				t.Errorf("expected %s, got nil", tc.want)

				return
			}

			fset := token.NewFileSet()
			var buf strings.Builder
			err := printer.Fprint(&buf, fset, expr)
			if err != nil {
				t.Fatal(err)
			}
			if got := buf.String(); got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}
