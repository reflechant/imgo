package transpiler

import (
	"fmt"
	"go/ast"
	"go/token"
)

type validator struct {
	fset  *token.FileSet
	diags Diagnostics
}

func (v *validator) report(pos token.Pos, code, msg string) {
	v.diags = append(v.diags, Diagnostic{
		Pos:     v.fset.Position(pos),
		Code:    code,
		Message: msg,
	})
}

// Validate checks that the ImGo source file follows the "No Mutation" rules.
// It accumulates every violation and returns them together so users see all
// problems at once. Returns nil when the file passes.
func Validate(fset *token.FileSet, file *ast.File) error {
	v := &validator{fset: fset}
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}

		switch node := n.(type) {
		case *ast.DeclStmt:
			if gen, ok := node.Decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
				v.report(node.Pos(), CodeDisallowedVar,
					"'var' is prohibited inside blocks; use ':=' for shadowing")
			}
		case *ast.AssignStmt:
			if node.Tok != token.DEFINE {
				v.report(node.Pos(), CodeDisallowedAssignment,
					fmt.Sprintf("mutation operator %s is prohibited in ImGo; use := for shadowing", node.Tok))
			}
		case *ast.IncDecStmt:
			v.report(node.Pos(), CodeDisallowedIncDec,
				"mutation (++, --) is prohibited in ImGo")
		case *ast.CallExpr:
			if ident, ok := node.Fun.(*ast.Ident); ok {
				switch ident.Name {
				case "append", "cap", "clear", "close", "copy", "new":
					v.report(node.Pos(), CodeDisallowedBuiltin,
						fmt.Sprintf("builtin '%s' is prohibited in ImGo; use functional equivalents", ident.Name))
				case "delete":
					v.report(node.Pos(), CodeDisallowedBuiltin,
						"'delete' builtin is prohibited in ImGo; use '.Delete(k)' and shadow the result")
				}
			}
		case *ast.ChanType:
			v.report(node.Pos(), CodeDisallowedChanType,
				"channel types are prohibited in ImGo; concurrency is reserved for Stage 8")
		case *ast.GoStmt:
			v.report(node.Pos(), CodeDisallowedGoStmt,
				"'go' statement is prohibited in ImGo; concurrency is reserved for Stage 8")
		case *ast.SendStmt:
			v.report(node.Pos(), CodeDisallowedChanOp,
				"channel send '<-' is prohibited in ImGo; concurrency is reserved for Stage 8")
		case *ast.UnaryExpr:
			if node.Op == token.ARROW {
				v.report(node.Pos(), CodeDisallowedChanOp,
					"channel receive '<-' is prohibited in ImGo; concurrency is reserved for Stage 8")
			}
		case *ast.SelectStmt:
			v.report(node.Pos(), CodeDisallowedSelectStmt,
				"'select' statement is prohibited in ImGo; concurrency is reserved for Stage 8")
		case *ast.SliceExpr:
			if node.Slice3 {
				v.report(node.Pos(), CodeDisallowedFullSlice,
					"three-index slice 'a[low:high:max]' is prohibited in ImGo; capacity has no persistent analog")
			}
		}
		return true
	})
	return v.diags.asError()
}
