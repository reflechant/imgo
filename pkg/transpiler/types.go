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
// builtin calls like set/setIn that aren't valid Go without lowering.
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

// typeOf returns the resolved type of x or nil when info is missing the
// entry. Used by the rewriter and validator to dispatch on receiver kind.
func typeOf(info *types.Info, x ast.Expr) types.Type {
	if info == nil {
		return nil
	}
	tv, ok := info.Types[x]
	if !ok {
		return nil
	}
	return tv.Type
}

// isMapLike reports whether x is known to have an underlying map type.
// When info is nil or lacks an entry for x, it returns true so callers
// fall back to the pre-type-info behavior (rewrite unconditionally).
func isMapLike(info *types.Info, x ast.Expr) bool {
	t := typeOf(info, x)
	if t == nil {
		return true
	}
	_, ok := t.Underlying().(*types.Map)
	return ok
}

// isListLike reports whether x is a Go slice. Unlike isMapLike, the
// fallback for unknown types is false: builtins on lists must be
// dispatched only when we can prove the receiver is a slice, otherwise
// the map fallback (which appends method-call shape) wins.
func isListLike(info *types.Info, x ast.Expr) bool {
	t := typeOf(info, x)
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Slice)
	return ok
}

// isArrayLike reports whether x is a Go fixed-size array.
func isArrayLike(info *types.Info, x ast.Expr) bool {
	t := typeOf(info, x)
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Array)
	return ok
}

// isStructLike reports whether x is a struct value. Pointer-to-struct
// is excluded — Stage 1 supports the value form only; an explicit deref
// keeps the user's intent visible.
func isStructLike(info *types.Info, x ast.Expr) bool {
	t := typeOf(info, x)
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Struct)
	return ok
}

// typeExprFor converts a types.Type into an ast.Expr suitable for use as
// a function parameter or return type in generated Go. It handles the
// shapes needed by struct update IIFEs:
//   - Named types (`type S struct{...}`) → *ast.Ident "S" (same package)
//     or *ast.SelectorExpr "pkg.S" (other package).
//   - Pointer to named → *ast.StarExpr.
//
// Returns nil for shapes the rewriter can't safely emit (anonymous
// structs, generics, etc.); the caller treats nil as "fall back".
func typeExprFor(t types.Type) ast.Expr {
	if t == nil {
		return nil
	}
	switch tt := t.(type) {
	case *types.Named:
		obj := tt.Obj()
		if obj == nil {
			return nil
		}
		pkg := obj.Pkg()
		if pkg == nil {
			return ast.NewIdent(obj.Name())
		}
		// Same-package named type: use the bare name.
		return ast.NewIdent(obj.Name())
	case *types.Pointer:
		inner := typeExprFor(tt.Elem())
		if inner == nil {
			return nil
		}
		return &ast.StarExpr{X: inner}
	}
	return nil
}
