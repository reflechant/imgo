package transpiler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
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
