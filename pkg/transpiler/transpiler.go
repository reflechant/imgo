package transpiler

import (
	"go/ast"
	"go/token"
)

// Transpile performs the full ImGo transpilation pipeline:
// 1. Validates the AST against ImGo purity rules.
// 2. Rewrites the AST to use persistent data structures and SSA mangling.
func Transpile(fset *token.FileSet, file *ast.File) (*ast.File, error) {
	if err := Validate(fset, file); err != nil {
		return nil, err
	}
	return Rewrite(file), nil
}
