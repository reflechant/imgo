package transpiler

import (
	"go/ast"
	"go/token"
)

// imgoBuiltins lists the value-update builtin names recognized in ImGo
// source. They are not Go builtins; the rewriter lowers each call into a
// receiver-appropriate method or free-function chain. Stage 5 (this PR)
// implements the map-receiver dispatch only — struct/list/array
// receivers are deferred.
var imgoBuiltins = map[string]bool{
	"get":      true,
	"getIn":    true,
	"set":      true,
	"setIn":    true,
	"update":   true,
	"updateIn": true,
	"delete":   true,
	"deleteIn": true,
}

// mapBuiltinMethod maps a single-key builtin name to the equivalent
// method on persistent.Map. Multi-key forms (setIn/updateIn/deleteIn)
// have their own method names that the existing rewriter expands further.
var mapBuiltinMethod = map[string]string{
	"set":      "Set",
	"update":   "Update",
	"delete":   "Delete",
	"setIn":    "SetIn",
	"updateIn": "UpdateIn",
	"deleteIn": "DeleteIn",
}

// expandMapBuiltin translates a builtin call with a map receiver into
// method-call form. The first argument is the receiver; the remaining
// arguments become the method arguments. The `get`/`getIn` cases produce
// a chain of .Get (or a single .Lookup when wantTwoValues is true).
func expandMapBuiltin(name string, args []ast.Expr, wantTwoValues bool, pos token.Pos) ast.Expr {
	receiver := args[0]
	rest := args[1:]

	if name == "get" {
		method := "Get"
		if wantTwoValues {
			method = "Lookup"
		}
		return setPos(&ast.CallExpr{
			Fun:  &ast.SelectorExpr{X: receiver, Sel: ast.NewIdent(method)},
			Args: rest,
		}, pos)
	}

	if name == "getIn" {
		var res ast.Expr = receiver
		for i, k := range rest {
			method := "Get"
			// Two-value form only applies to the final lookup of the chain.
			if wantTwoValues && i == len(rest)-1 {
				method = "Lookup"
			}
			res = &ast.CallExpr{
				Fun:  &ast.SelectorExpr{X: res, Sel: ast.NewIdent(method)},
				Args: []ast.Expr{k},
			}
		}
		return setPos(res, pos)
	}

	return setPos(&ast.CallExpr{
		Fun:  &ast.SelectorExpr{X: receiver, Sel: ast.NewIdent(mapBuiltinMethod[name])},
		Args: rest,
	}, pos)
}

// isShadowed reports whether name has been bound to a local variable in
// the current scope chain. If so, the rewriter must treat the identifier
// as a regular reference rather than as a builtin call site.
func isShadowed(env []map[string]string, name string) bool {
	for _, scope := range env {
		if _, ok := scope[name]; ok {
			return true
		}
	}
	return false
}
