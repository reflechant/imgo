package transpiler

import (
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

func TestIsMapLike(t *testing.T) {
	t.Parallel()
	dummy := ast.NewIdent("x")

	assert.True(t, isMapLike(nil, dummy))

	emptyInfo := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue)}
	assert.True(t, isMapLike(emptyInfo, dummy))

	src := `package main
type S struct{}
func main() {
    s := S{}
    m := map[string]int{}
    _, _ = s, m
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, 0)
	require.NoError(t, err)

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

	require.NotNil(t, sIdent, "could not locate sIdent")
	require.NotNil(t, mIdent, "could not locate mIdent")

	assert.False(t, isMapLike(info, sIdent))
	assert.True(t, isMapLike(info, mIdent))
}

func TestIsArrayLike(t *testing.T) {
	t.Parallel()
	dummy := ast.NewIdent("x")

	assert.False(t, isArrayLike(nil, dummy))

	src := `package main
func main() {
    a := [3]int{1, 2, 3}
    s := []int{1, 2, 3}
    _, _ = a, s
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, 0)
	require.NoError(t, err)

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

	require.NotNil(t, aIdent, "could not locate aIdent")
	require.NotNil(t, sIdent, "could not locate sIdent")

	assert.True(t, isArrayLike(info, aIdent))
	assert.False(t, isArrayLike(info, sIdent))
}

func TestTypeExprFor(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			expr := typeExprFor(tc.typ)
			if tc.want == "" {
				assert.Nil(t, expr)

				return
			}
			require.NotNil(t, expr)

			fset := token.NewFileSet()
			var buf strings.Builder
			err := printer.Fprint(&buf, fset, expr)
			require.NoError(t, err)
			assert.Equal(t, tc.want, buf.String())
		})
	}
}
