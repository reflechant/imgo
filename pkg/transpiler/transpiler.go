// Package transpiler implements the two-phase ImGo transpilation pipeline:
// validate ImGo source for immutability violations, then rewrite it to
// idiomatic Go using persistent data structures from pkg/persistent.
package transpiler

import (
	"go/ast"
	"go/token"
)

// Transpile performs the full ImGo transpilation pipeline:
// 1. Type-checks the AST on a best-effort basis (errors swallowed).
// 2. Validates the AST against ImGo purity rules.
// 3. Rewrites the AST to use persistent data structures and SSA mangling.
func Transpile(fset *token.FileSet, file *ast.File) (*ast.File, error) {
	info := typeCheck(fset, file)
	err := Validate(fset, file, info)
	if err != nil {
		return nil, err
	}

	return Rewrite(file, info), nil
}
