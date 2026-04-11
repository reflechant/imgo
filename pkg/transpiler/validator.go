package transpiler

import (
	"fmt"
	"go/ast"
	"go/token"
)

// Validate checks that the ImGo source file follows the "No Mutation" and "No Pointers" rules.
func Validate(fset *token.FileSet, file *ast.File) error {
	var walkErr error
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}

		switch node := n.(type) {
		case *ast.GenDecl:
			if node.Tok == token.VAR {
				// Package-level vars are allowed without explicit initialization.
				// They will follow Go's zero-value defaults.
			}
		case *ast.DeclStmt:
			// Prohibit 'var' inside functions/blocks. Only ':=', 'const', or type decls allowed.
			if gen, ok := node.Decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
				walkErr = fmt.Errorf("'var' is prohibited inside blocks at %v. Use ':=' for shadowing.", fset.Position(node.Pos()))
				return false
			}
		case *ast.AssignStmt:
			// Allow token.DEFINE (:=), reject all others (=, +=, -=, *=, etc.)
			if node.Tok != token.DEFINE {
				walkErr = fmt.Errorf("mutation operator %s is prohibited in ImGo at %v. Use := for shadowing.", node.Tok, fset.Position(node.Pos()))
				return false
			}
		case *ast.IncDecStmt:
			// Reject x++ and x--
			walkErr = fmt.Errorf("mutation (++, --) is prohibited in ImGo at %v.", fset.Position(node.Pos()))
			return false
		case *ast.StarExpr:
			// Reject pointer types (*T) and dereferences (*p)
			walkErr = fmt.Errorf("pointers are prohibited in ImGo at %v.", fset.Position(node.Pos()))
			return false
		case *ast.UnaryExpr:
			// Reject address-of (&x)
			if node.Op == token.AND {
				walkErr = fmt.Errorf("address-of (&) is prohibited in ImGo at %v.", fset.Position(node.Pos()))
				return false
			}
		case *ast.CallExpr:
			// Prohibit builtins that imply in-place mutation or return pointers.
			if ident, ok := node.Fun.(*ast.Ident); ok {
				switch ident.Name {
				case "append", "cap", "clear", "close", "copy", "new":
					walkErr = fmt.Errorf("builtin '%s' is prohibited in ImGo at %v. Use functional equivalents.", ident.Name, fset.Position(node.Pos()))
					return false
				case "delete":
					walkErr = fmt.Errorf("'delete' builtin is prohibited in ImGo at %v. Use '.Delete(k)' and shadow the result.", fset.Position(node.Pos()))
					return false
				}
			}
		}
		return true
	})
	return walkErr
}
