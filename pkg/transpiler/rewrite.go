package transpiler

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
)

func Rewrite(file *ast.File, info *types.Info) *ast.File {
	hasPersistent := false
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Type != nil {
				if d.Type.Params != nil {
					for _, field := range d.Type.Params.List {
						field.Type = rewriteType(field.Type, &hasPersistent)
					}
				}
				if d.Type.Results != nil {
					for _, field := range d.Type.Results.List {
						field.Type = rewriteType(field.Type, &hasPersistent)
					}
				}
			}
			env := []map[string]string{make(map[string]string)}
			versions := make(map[string]int)
			rewriteBlock(d.Body, env, versions, info, &hasPersistent)
		case *ast.GenDecl:
			if d.Tok == token.VAR {
				for _, spec := range d.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						if vs.Type != nil {
							vs.Type = rewriteType(vs.Type, &hasPersistent)
						}
						for i, val := range vs.Values {
							vs.Values[i] = rewriteExpr(val, nil, make(map[string]int), false, info, &hasPersistent)
						}
					}
				}
			}
		}
	}

	if hasPersistent {
		addPersistentImport(file)
	}

	return file
}

func addPersistentImport(file *ast.File) {
	path := `"github.com/rg/imgo/pkg/persistent"`
	for _, spec := range file.Imports {
		if spec.Path.Value == path {
			return
		}
	}

	newImport := &ast.ImportSpec{
		Path: &ast.BasicLit{Kind: token.STRING, Value: path},
	}

	for _, decl := range file.Decls {
		if g, ok := decl.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
			g.Specs = append(g.Specs, newImport)
			return
		}
	}

	newDecl := &ast.GenDecl{
		Tok:   token.IMPORT,
		Specs: []ast.Spec{newImport},
	}
	file.Decls = append([]ast.Decl{newDecl}, file.Decls...)
}

func rewriteType(typ ast.Expr, hasPersistent *bool) ast.Expr {
	if typ == nil {
		return nil
	}
	switch t := typ.(type) {
	case *ast.MapType:
		*hasPersistent = true
		return setPos(&ast.IndexListExpr{
			X: &ast.SelectorExpr{
				X:   setPos(ast.NewIdent("persistent"), t.Pos()),
				Sel: ast.NewIdent("Map"),
			},
			Indices: []ast.Expr{rewriteType(t.Key, hasPersistent), rewriteType(t.Value, hasPersistent)},
			Lbrack:  t.Pos(),
		}, t.Pos())
	case *ast.ArrayType:
		if t.Len == nil {
			*hasPersistent = true
			return setPos(&ast.IndexListExpr{
				X: &ast.SelectorExpr{
					X:   setPos(ast.NewIdent("persistent"), t.Pos()),
					Sel: ast.NewIdent("List"),
				},
				Indices: []ast.Expr{rewriteType(t.Elt, hasPersistent)},
				Lbrack:  t.Pos(),
			}, t.Pos())
		}
	}
	return typ
}

func rewriteBlock(block *ast.BlockStmt, env []map[string]string, versions map[string]int, info *types.Info, hasPersistent *bool) {
	if block == nil {
		return
	}

	for i, stmt := range block.List {
		block.List[i] = rewriteStmt(stmt, env, versions, info, hasPersistent)
	}
}

func rewriteStmt(stmt ast.Stmt, env []map[string]string, versions map[string]int, info *types.Info, hasPersistent *bool) ast.Stmt {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		// Specialized check for 2-value indexing context: v, ok := m[k]
		if len(s.Lhs) == 2 && len(s.Rhs) == 1 {
			if _, ok := s.Rhs[0].(*ast.IndexExpr); ok {
				s.Rhs[0] = rewriteExpr(s.Rhs[0], env, versions, true, info, hasPersistent) // Pass true for wantTwoValues
				goto processLHS
			}
			// v, ok := get(m, k)  (or getIn) — propagate the two-value flag
			// so the builtin lowers to .Lookup instead of .Get.
			if call, ok := s.Rhs[0].(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok && (id.Name == "get" || id.Name == "getIn") && !isShadowed(env, id.Name) {
					s.Rhs[0] = rewriteExpr(s.Rhs[0], env, versions, true, info, hasPersistent)
					goto processLHS
				}
			}
		}

		// Default processing
		for j, expr := range s.Rhs {
			s.Rhs[j] = rewriteExpr(expr, env, versions, false, info, hasPersistent)
		}

	processLHS:
		if s.Tok == token.DEFINE {
			newNames := make([]ast.Expr, len(s.Lhs))
			for j, lhs := range s.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok {
					if ident.Name == "_" {
						newNames[j] = ident
						continue
					}
					versions[ident.Name]++
					count := versions[ident.Name]
					mangled := fmt.Sprintf("%s_%d", ident.Name, count)
					env[len(env)-1][ident.Name] = mangled
					newNames[j] = &ast.Ident{Name: mangled, NamePos: ident.Pos()}
				} else {
					newNames[j] = lhs
				}
			}
			s.Lhs = newNames
		} else {
			// Even for non-DEFINE, rewrite identifiers on LHS to latest versions
			for j, lhs := range s.Lhs {
				s.Lhs[j] = rewriteExpr(lhs, env, versions, false, info, hasPersistent)
			}
		}
		return s

	case *ast.ExprStmt:
		s.X = rewriteExpr(s.X, env, versions, false, info, hasPersistent)
		return s
	case *ast.ReturnStmt:
		for i, expr := range s.Results {
			s.Results[i] = rewriteExpr(expr, env, versions, false, info, hasPersistent)
		}
		return s
	case *ast.BlockStmt:
		newEnv := make([]map[string]string, len(env))
		copy(newEnv, env)
		newEnv = append(newEnv, make(map[string]string))
		rewriteBlock(s, newEnv, versions, info, hasPersistent)
		return s
	case *ast.IfStmt:
		newEnv := make([]map[string]string, len(env))
		copy(newEnv, env)
		newEnv = append(newEnv, make(map[string]string))
		if s.Init != nil {
			s.Init = rewriteStmt(s.Init, newEnv, versions, info, hasPersistent)
		}
		s.Cond = rewriteExpr(s.Cond, newEnv, versions, false, info, hasPersistent)
		rewriteBlock(s.Body, newEnv, versions, info, hasPersistent)
		if s.Else != nil {
			if els, ok := s.Else.(*ast.BlockStmt); ok {
				elseEnv := make([]map[string]string, len(newEnv))
				copy(elseEnv, newEnv)
				elseEnv = append(elseEnv, make(map[string]string))
				rewriteBlock(els, elseEnv, versions, info, hasPersistent)
			} else {
				s.Else = rewriteStmt(s.Else, newEnv, versions, info, hasPersistent)
			}
		}
		return s
	case *ast.ForStmt:
		newEnv := make([]map[string]string, len(env))
		copy(newEnv, env)
		newEnv = append(newEnv, make(map[string]string))
		if s.Init != nil {
			s.Init = rewriteStmt(s.Init, newEnv, versions, info, hasPersistent)
		}
		if s.Cond != nil {
			s.Cond = rewriteExpr(s.Cond, newEnv, versions, false, info, hasPersistent)
		}
		if s.Post != nil {
			s.Post = rewriteStmt(s.Post, newEnv, versions, info, hasPersistent)
		}
		rewriteBlock(s.Body, newEnv, versions, info, hasPersistent)
		return s
	case *ast.RangeStmt:
		s.X = rewriteExpr(s.X, env, versions, false, info, hasPersistent)
		// Desugar 'range x' to 'range x.All()'
		s.X = &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   s.X,
				Sel: ast.NewIdent("All"),
			},
		}
		bodyEnv := make([]map[string]string, len(env))
		copy(bodyEnv, env)
		bodyEnv = append(bodyEnv, make(map[string]string))

		if s.Tok == token.DEFINE {
			if ident, ok := s.Key.(*ast.Ident); ok && ident.Name != "_" {
				versions[ident.Name]++
				mangled := fmt.Sprintf("%s_%d", ident.Name, versions[ident.Name])
				bodyEnv[len(bodyEnv)-1][ident.Name] = mangled
				s.Key = &ast.Ident{Name: mangled, NamePos: ident.Pos()}
			}
			if ident, ok := s.Value.(*ast.Ident); ok && ident.Name != "_" {
				versions[ident.Name]++
				mangled := fmt.Sprintf("%s_%d", ident.Name, versions[ident.Name])
				bodyEnv[len(bodyEnv)-1][ident.Name] = mangled
				s.Value = &ast.Ident{Name: mangled, NamePos: ident.Pos()}
			}
		} else {
			if s.Key != nil {
				s.Key = rewriteExpr(s.Key, env, versions, false, info, hasPersistent)
			}
			if s.Value != nil {
				s.Value = rewriteExpr(s.Value, env, versions, false, info, hasPersistent)
			}
		}

		rewriteBlock(s.Body, bodyEnv, versions, info, hasPersistent)
		return s
	case *ast.SwitchStmt:
		newEnv := make([]map[string]string, len(env))
		copy(newEnv, env)
		newEnv = append(newEnv, make(map[string]string))
		if s.Init != nil {
			s.Init = rewriteStmt(s.Init, newEnv, versions, info, hasPersistent)
		}
		if s.Tag != nil {
			s.Tag = rewriteExpr(s.Tag, newEnv, versions, false, info, hasPersistent)
		}
		rewriteBlock(s.Body, newEnv, versions, info, hasPersistent)
		return s
	case *ast.TypeSwitchStmt:
		newEnv := make([]map[string]string, len(env))
		copy(newEnv, env)
		newEnv = append(newEnv, make(map[string]string))
		if s.Init != nil {
			s.Init = rewriteStmt(s.Init, newEnv, versions, info, hasPersistent)
		}
		s.Assign = rewriteStmt(s.Assign, newEnv, versions, info, hasPersistent)
		rewriteBlock(s.Body, newEnv, versions, info, hasPersistent)
		return s
	case *ast.CaseClause:
		for i, expr := range s.List {
			s.List[i] = rewriteExpr(expr, env, versions, false, info, hasPersistent)
		}
		for i, st := range s.Body {
			s.Body[i] = rewriteStmt(st, env, versions, info, hasPersistent)
		}
		return s
	case *ast.DeferStmt:
		if rewritten, ok := rewriteExpr(s.Call, env, versions, false, info, hasPersistent).(*ast.CallExpr); ok {
			s.Call = rewritten
		}
		return s
	case *ast.DeclStmt:
		if d, ok := s.Decl.(*ast.GenDecl); ok && d.Tok == token.VAR {
			for _, spec := range d.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					for i, name := range vs.Names {
						versions[name.Name]++
						count := versions[name.Name]
						mangled := fmt.Sprintf("%s_%d", name.Name, count)
						env[len(env)-1][name.Name] = mangled
						vs.Names[i] = &ast.Ident{Name: mangled, NamePos: name.Pos()}
					}
					if vs.Type != nil {
						vs.Type = rewriteType(vs.Type, hasPersistent)
					}
					for i, val := range vs.Values {
						vs.Values[i] = rewriteExpr(val, env, versions, false, info, hasPersistent)
					}
				}
			}
		}
		return s
	}
	return stmt
}

func rewriteExpr(expr ast.Expr, env []map[string]string, versions map[string]int, wantTwoValues bool, info *types.Info, hasPersistent *bool) ast.Expr {
	if expr == nil {
		return nil
	}

	switch e := expr.(type) {
	case *ast.Ident:
		if env != nil {
			for i := len(env) - 1; i >= 0; i-- {
				if mangled, ok := env[i][e.Name]; ok {
					return &ast.Ident{Name: mangled, NamePos: e.Pos()}
				}
			}
		}
		return e
	case *ast.BinaryExpr:
		e.X = rewriteExpr(e.X, env, versions, false, info, hasPersistent)
		e.Y = rewriteExpr(e.Y, env, versions, false, info, hasPersistent)
		return e
	case *ast.UnaryExpr:
		e.X = rewriteExpr(e.X, env, versions, false, info, hasPersistent)
		return e
	case *ast.StarExpr:
		e.X = rewriteExpr(e.X, env, versions, false, info, hasPersistent)
		return e
	case *ast.MapType, *ast.ArrayType:
		return rewriteType(e.(ast.Expr), hasPersistent)
	case *ast.CallExpr:
		// ImGo value-update builtins (set/get/update/delete and their *In
		// forms). Dispatch on the receiver's static type:
		//   - struct: lower get/getIn to selector chains; lower
		//     update/updateIn to an IIFE that copies the struct and
		//     overwrites the field.
		//   - list (slice): translate to method-call shape using
		//     persistent.List's .Set/.Get; In-forms reuse the existing
		//     map In expansion below.
		//   - map (or unknown): translate to method-call shape and
		//     reuse the existing map lowering.
		// Builtin recognition is suppressed when the name is shadowed
		// by a local binding (Go's own len/append shadowing rule).
		if ident, ok := e.Fun.(*ast.Ident); ok {
			if imgoBuiltins[ident.Name] && len(e.Args) >= 2 && !isShadowed(env, ident.Name) {
				if isStructLike(info, e.Args[0]) {
					typeExpr := typeExprFor(typeOf(info, e.Args[0]))
					if expanded := expandStructBuiltin(ident.Name, e.Args, typeExpr, e.Pos()); expanded != nil {
						return rewriteExpr(expanded, env, versions, wantTwoValues, info, hasPersistent)
					}
				}
				if isListLike(info, e.Args[0]) || isArrayLike(info, e.Args[0]) {
					if expanded := expandListBuiltin(ident.Name, e.Args, e.Pos()); expanded != nil {
						return rewriteExpr(expanded, env, versions, wantTwoValues, info, hasPersistent)
					}
				}
				if isMapLike(info, e.Args[0]) {
					translated := expandMapBuiltin(ident.Name, e.Args, wantTwoValues, e.Pos())
					return rewriteExpr(translated, env, versions, wantTwoValues, info, hasPersistent)
				}
			}
		}

		// Specialized handling for builtins
		if ident, ok := e.Fun.(*ast.Ident); ok {
			if ident.Name == "len" && len(e.Args) == 1 {
				*hasPersistent = true
				return setPos(&ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   setPos(ast.NewIdent("persistent"), e.Pos()),
						Sel: ast.NewIdent("Len"),
					},
					Args: []ast.Expr{rewriteExpr(e.Args[0], env, versions, false, info, hasPersistent)},
				}, e.Pos())
			}
			if ident.Name == "make" && len(e.Args) >= 1 {
				switch typ := e.Args[0].(type) {
				case *ast.MapType:
					*hasPersistent = true
					return setPos(&ast.CallExpr{
						Fun: &ast.IndexListExpr{
							X: &ast.SelectorExpr{
								X:   setPos(ast.NewIdent("persistent"), e.Pos()),
								Sel: ast.NewIdent("NewMap"),
							},
							Indices: []ast.Expr{rewriteType(typ.Key, hasPersistent), rewriteType(typ.Value, hasPersistent)},
						},
					}, e.Pos())
				case *ast.ArrayType:
					if typ.Len == nil {
						*hasPersistent = true
						return setPos(&ast.CallExpr{
							Fun: &ast.IndexListExpr{
								X: &ast.SelectorExpr{
									X:   setPos(ast.NewIdent("persistent"), e.Pos()),
									Sel: ast.NewIdent("NewList"),
								},
								Indices: []ast.Expr{rewriteType(typ.Elt, hasPersistent)},
							},
						}, e.Pos())
					}
				}
			}
		}

		// Capture the receiver before recursive rewriting, since the
		// child rewrite may replace sel.X with a freshly-mangled ident
		// that has no entry in info.
		var origReceiver ast.Expr
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			origReceiver = sel.X
		}

		e.Fun = rewriteExpr(e.Fun, env, versions, false, info, hasPersistent)
		for i, arg := range e.Args {
			e.Args[i] = rewriteExpr(arg, env, versions, false, info, hasPersistent)
		}

		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			// m.Set(k, v) -> m.Set(k, v)
			if sel.Sel.Name == "Set" || sel.Sel.Name == "Append" || sel.Sel.Name == "Delete" {
				return e
			}
			// SetIn / UpdateIn / DeleteIn only apply to map-typed receivers.
			// When type info shows the receiver is some other type (e.g. a
			// user struct that happens to define a method by the same name),
			// leave the call alone.
			if !isMapLike(info, origReceiver) {
				return e
			}
			if sel.Sel.Name == "SetIn" {
				N := len(e.Args) - 1
				targets := make([]ast.Expr, N)
				targets[0] = sel.X
				for i := 1; i < N; i++ {
					targets[i] = &ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   targets[i-1],
							Sel: ast.NewIdent("Get"),
						},
						Args: []ast.Expr{e.Args[i-1]},
					}
				}
				var res = e.Args[N]
				for i := N - 1; i >= 0; i-- {
					res = &ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   targets[i],
							Sel: ast.NewIdent("Set"),
						},
						Args: []ast.Expr{e.Args[i], res},
					}
				}
				return setPos(res, e.Pos())
			}
			if sel.Sel.Name == "UpdateIn" {
				N := len(e.Args) - 1
				targets := make([]ast.Expr, N)
				targets[0] = sel.X
				for i := 1; i < N; i++ {
					targets[i] = &ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   targets[i-1],
							Sel: ast.NewIdent("Get"),
						},
						Args: []ast.Expr{e.Args[i-1]},
					}
				}
				var res = &ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   targets[N-1],
						Sel: ast.NewIdent("Update"),
					},
					Args: []ast.Expr{e.Args[N-1], e.Args[N]},
				}
				for i := N - 2; i >= 0; i-- {
					res = &ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   targets[i],
							Sel: ast.NewIdent("Set"),
						},
						Args: []ast.Expr{e.Args[i], res},
					}
				}
				return setPos(res, e.Pos())
			}
			if sel.Sel.Name == "DeleteIn" {
				N := len(e.Args)
				targets := make([]ast.Expr, N)
				targets[0] = sel.X
				for i := 1; i < N; i++ {
					targets[i] = &ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   targets[i-1],
							Sel: ast.NewIdent("Get"),
						},
						Args: []ast.Expr{e.Args[i-1]},
					}
				}
				var res = &ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   targets[N-1],
						Sel: ast.NewIdent("Delete"),
					},
					Args: []ast.Expr{e.Args[N-1]},
				}
				for i := N - 2; i >= 0; i-- {
					res = &ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   targets[i],
							Sel: ast.NewIdent("Set"),
						},
						Args: []ast.Expr{e.Args[i], res},
					}
				}
				return setPos(res, e.Pos())
			}
		}
		return e
	case *ast.SelectorExpr:
		e.X = rewriteExpr(e.X, env, versions, false, info, hasPersistent)
		return e
	case *ast.IndexExpr:
		method := "Get"
		if wantTwoValues {
			method = "Lookup"
		}
		return setPos(&ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   rewriteExpr(e.X, env, versions, false, info, hasPersistent),
				Sel: ast.NewIdent(method),
			},
			Args: []ast.Expr{rewriteExpr(e.Index, env, versions, false, info, hasPersistent)},
		}, e.Pos())
	case *ast.CompositeLit:
		if mt, ok := e.Type.(*ast.MapType); ok {
			*hasPersistent = true
			var res = setPos(&ast.CallExpr{
				Fun: &ast.IndexListExpr{
					X: &ast.SelectorExpr{
						X:   setPos(ast.NewIdent("persistent"), e.Pos()),
						Sel: ast.NewIdent("NewMap"),
					},
					Indices: []ast.Expr{rewriteType(mt.Key, hasPersistent), rewriteType(mt.Value, hasPersistent)},
				},
			}, e.Pos())
			for _, el := range e.Elts {
				if kv, ok := el.(*ast.KeyValueExpr); ok {
					res = &ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   res,
							Sel: ast.NewIdent("Set"),
						},
						Args: []ast.Expr{rewriteExpr(kv.Key, env, versions, false, info, hasPersistent), rewriteExpr(kv.Value, env, versions, false, info, hasPersistent)},
					}
				}
			}
			return res
		}
		if st, ok := e.Type.(*ast.ArrayType); ok && st.Len == nil {
			*hasPersistent = true
			var res = setPos(&ast.CallExpr{
				Fun: &ast.IndexListExpr{
					X: &ast.SelectorExpr{
						X:   setPos(ast.NewIdent("persistent"), e.Pos()),
						Sel: ast.NewIdent("NewList"),
					},
					Indices: []ast.Expr{rewriteType(st.Elt, hasPersistent)},
				},
			}, e.Pos())
			for _, el := range e.Elts {
				res = &ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   res,
						Sel: ast.NewIdent("Append"),
					},
					Args: []ast.Expr{rewriteExpr(el, env, versions, false, info, hasPersistent)},
				}
			}
			return res
		}
		// General case (e.g. Structs)
		for i, el := range e.Elts {
			if kv, ok := el.(*ast.KeyValueExpr); ok {
				kv.Value = rewriteExpr(kv.Value, env, versions, false, info, hasPersistent)
			} else {
				e.Elts[i] = rewriteExpr(el, env, versions, false, info, hasPersistent)
			}
		}
		return e
	case *ast.FuncLit:
		newEnv := make([]map[string]string, len(env))
		copy(newEnv, env)
		newEnv = append(newEnv, make(map[string]string))
		rewriteBlock(e.Body, newEnv, versions, info, hasPersistent)
		return e
	case *ast.ParenExpr:
		e.X = rewriteExpr(e.X, env, versions, false, info, hasPersistent)
		return e
	case *ast.SliceExpr:
		*hasPersistent = true
		x := rewriteExpr(e.X, env, versions, false, info, hasPersistent)
		var low ast.Expr
		if e.Low != nil {
			low = rewriteExpr(e.Low, env, versions, false, info, hasPersistent)
		} else {
			low = &ast.BasicLit{Kind: token.INT, Value: "0"}
		}
		var high ast.Expr
		if e.High != nil {
			high = rewriteExpr(e.High, env, versions, false, info, hasPersistent)
		} else {
			high = &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   ast.NewIdent("persistent"),
					Sel: ast.NewIdent("Len"),
				},
				Args: []ast.Expr{x},
			}
		}
		return setPos(&ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   setPos(ast.NewIdent("persistent"), e.Pos()),
				Sel: ast.NewIdent("Slice"),
			},
			Args: []ast.Expr{x, low, high},
		}, e.Pos())
	case *ast.TypeAssertExpr:
		e.X = rewriteExpr(e.X, env, versions, false, info, hasPersistent)
		e.Type = rewriteType(e.Type, hasPersistent)
		return e
	}

	return expr
}

func setPos(n ast.Expr, pos token.Pos) ast.Expr {
	if n == nil || pos == token.NoPos {
		return n
	}
	switch e := n.(type) {
	case *ast.Ident:
		e.NamePos = pos
	case *ast.CallExpr:
		e.Lparen = pos
	case *ast.SelectorExpr:
		setPos(e.X, pos)
	case *ast.IndexListExpr:
		e.Lbrack = pos
	}
	return n
}
