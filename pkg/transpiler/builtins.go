package transpiler

import (
	"go/ast"
	"go/token"
	"strconv"
)

// imgoBuiltins lists the value-update builtin names recognized in ImGo
// source. They are not Go builtins; the rewriter lowers each call into a
// receiver-appropriate method or expression chain.
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

// expandListBuiltin handles get/set/update/setIn/updateIn/getIn for slice
// receivers. Lists are rewritten elsewhere to persistent.List[T] which
// also exposes .Set/.Get; that lets the existing inline expansion of
// SetIn/UpdateIn/etc. work uniformly across maps and lists.
//
// delete/deleteIn on lists are not supported — the validator rejects them
// before this expansion runs, but we defensively return nil so a caller
// that bypasses validation reverts to leaving the call alone.
func expandListBuiltin(name string, args []ast.Expr, pos token.Pos) ast.Expr {
	receiver := args[0]
	rest := args[1:]

	switch name {
	case "get":
		// Lists always succeed at indexing (panic on OOB); no two-value form.
		return setPos(&ast.CallExpr{
			Fun:  &ast.SelectorExpr{X: receiver, Sel: ast.NewIdent("Get")},
			Args: rest,
		}, pos)
	case "getIn":
		var res ast.Expr = receiver
		for _, k := range rest {
			res = &ast.CallExpr{
				Fun:  &ast.SelectorExpr{X: res, Sel: ast.NewIdent("Get")},
				Args: []ast.Expr{k},
			}
		}
		return setPos(res, pos)
	case "set", "setIn", "update", "updateIn":
		// Same shape as map dispatch; the existing rewriter handles
		// the SetIn/UpdateIn expansion uniformly.
		return setPos(&ast.CallExpr{
			Fun:  &ast.SelectorExpr{X: receiver, Sel: ast.NewIdent(mapBuiltinMethod[name])},
			Args: rest,
		}, pos)
	}
	return nil
}

// expandStructBuiltin handles get/getIn/update/updateIn for struct
// receivers. Field names must be string literals; dynamic field names
// are rejected by the validator.
//
// get(s, "F")            -> s.F
// getIn(s, "A", "B")     -> s.A.B
// update(s, "F", fn)     -> func(__s T) T { __s.F = fn(__s.F); return __s }(s)
// updateIn(s, "A", "B", fn)
//                        -> func(__s T) T {
//                               __s.A = func(__a A) A {
//                                   __a.B = fn(__a.B); return __a
//                               }(__s.A)
//                               return __s
//                           }(s)
//
// typeExpr is the AST form of the receiver's type (extracted from
// *types.Info) needed to write the IIFE parameter type. When typeExpr is
// nil for an update form, expansion fails and returns nil — the caller
// falls back to leaving the call alone (and the resulting Go won't
// compile, surfacing the missing type info).
func expandStructBuiltin(name string, args []ast.Expr, typeExpr ast.Expr, pos token.Pos) ast.Expr {
	receiver := args[0]

	switch name {
	case "get":
		field, ok := stringLitField(args[1])
		if !ok {
			return nil
		}
		return setPos(&ast.SelectorExpr{X: receiver, Sel: ast.NewIdent(field)}, pos)

	case "getIn":
		var res ast.Expr = receiver
		for _, key := range args[1:] {
			field, ok := stringLitField(key)
			if !ok {
				return nil
			}
			res = &ast.SelectorExpr{X: res, Sel: ast.NewIdent(field)}
		}
		return setPos(res, pos)

	case "update":
		if typeExpr == nil {
			return nil
		}
		field, ok := stringLitField(args[1])
		if !ok {
			return nil
		}
		fn := args[2]
		// Build: func(__s T) T { __s.F = fn(__s.F); return __s }(receiver)
		paramName := "__s"
		body := updateBody(paramName, []string{field}, fn, typeExpr)
		return setPos(buildIIFE(paramName, typeExpr, body, receiver), pos)

	case "updateIn":
		// updateIn(receiver, k1, k2, ..., fn) — all keys must be string
		// literals (struct field names). The receiver type must be known.
		if typeExpr == nil {
			return nil
		}
		n := len(args)
		fields := make([]string, 0, n-2)
		for _, k := range args[1 : n-1] {
			f, ok := stringLitField(k)
			if !ok {
				return nil
			}
			fields = append(fields, f)
		}
		fn := args[n-1]
		paramName := "__s"
		body := updateBody(paramName, fields, fn, typeExpr)
		return setPos(buildIIFE(paramName, typeExpr, body, receiver), pos)
	}
	return nil
}

// stringLitField extracts a Go identifier from a string-literal field
// path argument (e.g., "Name" -> Name). Returns ("", false) if not a
// string literal.
func stringLitField(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return v, true
}

// buildIIFE constructs (func(param T) T { body... return param })(arg).
func buildIIFE(param string, typeExpr ast.Expr, bodyStmts []ast.Stmt, arg ast.Expr) ast.Expr {
	stmts := make([]ast.Stmt, 0, len(bodyStmts)+1)
	stmts = append(stmts, bodyStmts...)
	stmts = append(stmts, &ast.ReturnStmt{
		Results: []ast.Expr{ast.NewIdent(param)},
	})
	return &ast.CallExpr{
		Fun: &ast.FuncLit{
			Type: &ast.FuncType{
				Params: &ast.FieldList{List: []*ast.Field{{
					Names: []*ast.Ident{ast.NewIdent(param)},
					Type:  typeExpr,
				}}},
				Results: &ast.FieldList{List: []*ast.Field{{Type: typeExpr}}},
			},
			Body: &ast.BlockStmt{List: stmts},
		},
		Args: []ast.Expr{arg},
	}
}

// updateBody emits a single assignment statement of the form
//
//	__s.F1.F2... = fn(__s.F1.F2...)
//
// where the LHS and the inner read inside fn() are the same selector
// chain. It's a value-update: in Go, struct copies via assignment, so
// rewriting __s.A.B = ... mutates __s only (the IIFE caller's value is
// untouched).
func updateBody(param string, fields []string, fn ast.Expr, _ ast.Expr) []ast.Stmt {
	var lhs ast.Expr = ast.NewIdent(param)
	for _, f := range fields {
		lhs = &ast.SelectorExpr{X: lhs, Sel: ast.NewIdent(f)}
	}
	var rhsArg ast.Expr = ast.NewIdent(param)
	for _, f := range fields {
		rhsArg = &ast.SelectorExpr{X: rhsArg, Sel: ast.NewIdent(f)}
	}
	rhs := &ast.CallExpr{Fun: fn, Args: []ast.Expr{rhsArg}}
	return []ast.Stmt{&ast.AssignStmt{
		Lhs: []ast.Expr{lhs},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{rhs},
	}}
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
