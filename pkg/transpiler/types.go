package transpiler

import (
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"
)

// typeCheck runs go/types over file on a best-effort basis. Errors are
// swallowed so callers receive as much type information as go/types can
// extract — this matters for ImGo source pre-rewrite, which contains
// method calls like .SetIn that only become valid Go after rewriting.
// Callers must treat the returned *types.Info as partial.
func typeCheck(fset *token.FileSet, file *ast.File) *types.Info {
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	conf := &types.Config{
		Importer: importer.Default(),
		Error:    func(error) {},
	}
	_, _ = conf.Check(file.Name.Name, fset, []*ast.File{file}, info)
	return info
}

// isMapLike reports whether x is known to have an underlying map type.
// When info is nil or lacks an entry for x, it returns true so callers
// fall back to the pre-type-info behavior (rewrite unconditionally).
func isMapLike(info *types.Info, x ast.Expr) bool {
	if info == nil {
		return true
	}
	tv, ok := info.Types[x]
	if !ok || tv.Type == nil {
		return true
	}
	_, ok = tv.Type.Underlying().(*types.Map)
	return ok
}
