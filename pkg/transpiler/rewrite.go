package transpiler

import (
	"fmt"
	"go/ast"
	"go/token"
)

func Rewrite(file *ast.File) *ast.File {
	hasPersistent := false
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			env := []map[string]string{make(map[string]string)}
			versions := make(map[string]int)
			rewriteBlock(d.Body, env, versions)
			hasPersistent = true
		case *ast.GenDecl:
			if d.Tok == token.VAR {
				for _, spec := range d.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						if vs.Type != nil {
							vs.Type = rewriteType(vs.Type)
							hasPersistent = true
						}
						for i, val := range vs.Values {
							vs.Values[i] = rewriteExpr(val, nil, make(map[string]int), false)
							hasPersistent = true
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

func rewriteType(typ ast.Expr) ast.Expr {
	if typ == nil {
		return nil
	}
	switch t := typ.(type) {
	case *ast.MapType:
		return setPos(&ast.IndexListExpr{
			X: &ast.SelectorExpr{
				X:   setPos(ast.NewIdent("persistent"), t.Pos()),
				Sel: ast.NewIdent("Map"),
			},
			Indices: []ast.Expr{rewriteType(t.Key), rewriteType(t.Value)},
			Lbrack:  t.Pos(),
		}, t.Pos())
	case *ast.ArrayType:
		if t.Len == nil {
			return setPos(&ast.IndexListExpr{
				X: &ast.SelectorExpr{
					X:   setPos(ast.NewIdent("persistent"), t.Pos()),
					Sel: ast.NewIdent("List"),
				},
				Indices: []ast.Expr{rewriteType(t.Elt)},
				Lbrack:  t.Pos(),
			}, t.Pos())
		}
	}
	return typ
}

func rewriteBlock(block *ast.BlockStmt, env []map[string]string, versions map[string]int) {
	if block == nil {
		return
	}

	for i, stmt := range block.List {
		block.List[i] = rewriteStmt(stmt, env, versions)
	}
}

func rewriteStmt(stmt ast.Stmt, env []map[string]string, versions map[string]int) ast.Stmt {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		// Specialized check for 2-value indexing context: v, ok := m[k]
		if len(s.Lhs) == 2 && len(s.Rhs) == 1 {
			if _, ok := s.Rhs[0].(*ast.IndexExpr); ok {
				s.Rhs[0] = rewriteExpr(s.Rhs[0], env, versions, true) // Pass true for wantTwoValues
				goto processLHS
			}
		}

		// Default processing
		for j, expr := range s.Rhs {
			s.Rhs[j] = rewriteExpr(expr, env, versions, false)
		}

	processLHS:
		if s.Tok == token.DEFINE {
			newNames := make([]ast.Expr, len(s.Lhs))
			for j, lhs := range s.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok {
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
				s.Lhs[j] = rewriteExpr(lhs, env, versions, false)
			}
		}
		return s

	case *ast.ExprStmt:
		s.X = rewriteExpr(s.X, env, versions, false)
		return s
	case *ast.ReturnStmt:
		for i, expr := range s.Results {
			s.Results[i] = rewriteExpr(expr, env, versions, false)
		}
		return s
	case *ast.BlockStmt:
		newEnv := make([]map[string]string, len(env))
		copy(newEnv, env)
		newEnv = append(newEnv, make(map[string]string))
		rewriteBlock(s, newEnv, versions)
		return s
	case *ast.IfStmt:
		if s.Init != nil { s.Init = rewriteStmt(s.Init, env, versions) }
		s.Cond = rewriteExpr(s.Cond, env, versions, false)
		bodyEnv := make([]map[string]string, len(env))
		copy(bodyEnv, env)
		bodyEnv = append(bodyEnv, make(map[string]string))
		rewriteBlock(s.Body, bodyEnv, versions)
		if s.Else != nil {
			elseEnv := make([]map[string]string, len(env))
			copy(elseEnv, env)
			elseEnv = append(elseEnv, make(map[string]string))
			if els, ok := s.Else.(*ast.BlockStmt); ok {
				rewriteBlock(els, elseEnv, versions)
			} else {
				s.Else = rewriteStmt(s.Else, env, versions)
			}
		}
		return s
	case *ast.RangeStmt:
		s.X = rewriteExpr(s.X, env, versions, false)
		bodyEnv := make([]map[string]string, len(env))
		copy(bodyEnv, env)
		bodyEnv = append(bodyEnv, make(map[string]string))
		
		if s.Tok == token.DEFINE {
			if ident, ok := s.Key.(*ast.Ident); ok {
				versions[ident.Name]++
				mangled := fmt.Sprintf("%s_%d", ident.Name, versions[ident.Name])
				bodyEnv[len(bodyEnv)-1][ident.Name] = mangled
				s.Key = &ast.Ident{Name: mangled, NamePos: ident.Pos()}
			}
			if ident, ok := s.Value.(*ast.Ident); ok {
				versions[ident.Name]++
				mangled := fmt.Sprintf("%s_%d", ident.Name, versions[ident.Name])
				bodyEnv[len(bodyEnv)-1][ident.Name] = mangled
				s.Value = &ast.Ident{Name: mangled, NamePos: ident.Pos()}
			}
		} else {
			if s.Key != nil { s.Key = rewriteExpr(s.Key, env, versions, false) }
			if s.Value != nil { s.Value = rewriteExpr(s.Value, env, versions, false) }
		}
		
		rewriteBlock(s.Body, bodyEnv, versions)
		return s
	case *ast.SwitchStmt:
		if s.Init != nil { s.Init = rewriteStmt(s.Init, env, versions) }
		if s.Tag != nil { s.Tag = rewriteExpr(s.Tag, env, versions, false) }
		rewriteBlock(s.Body, env, versions)
		return s
	case *ast.TypeSwitchStmt:
		if s.Init != nil { s.Init = rewriteStmt(s.Init, env, versions) }
		s.Assign = rewriteStmt(s.Assign, env, versions)
		rewriteBlock(s.Body, env, versions)
		return s
	case *ast.CaseClause:
		for i, expr := range s.List {
			s.List[i] = rewriteExpr(expr, env, versions, false)
		}
		for i, st := range s.Body {
			s.Body[i] = rewriteStmt(st, env, versions)
		}
		return s
	case *ast.DeferStmt:
		s.Call = rewriteExpr(s.Call, env, versions, false).(*ast.CallExpr)
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
					if vs.Type != nil { vs.Type = rewriteType(vs.Type) }
					for i, val := range vs.Values {
						vs.Values[i] = rewriteExpr(val, env, versions, false)
					}
				}
			}
		}
		return s
	}
	return stmt
}

func rewriteExpr(expr ast.Expr, env []map[string]string, versions map[string]int, wantTwoValues bool) ast.Expr {
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
		e.X = rewriteExpr(e.X, env, versions, false)
		e.Y = rewriteExpr(e.Y, env, versions, false)
		return e
	case *ast.CallExpr:
		// Specialized handling for builtins
		if ident, ok := e.Fun.(*ast.Ident); ok {
			if ident.Name == "len" && len(e.Args) == 1 {
				return setPos(&ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   setPos(ast.NewIdent("persistent"), e.Pos()),
						Sel: ast.NewIdent("Len"),
					},
					Args: []ast.Expr{rewriteExpr(e.Args[0], env, versions, false)},
				}, e.Pos())
			}
			if ident.Name == "make" && len(e.Args) >= 1 {
				switch typ := e.Args[0].(type) {
				case *ast.MapType:
					return setPos(&ast.CallExpr{
						Fun: &ast.IndexListExpr{
							X: &ast.SelectorExpr{
								X:   setPos(ast.NewIdent("persistent"), e.Pos()),
								Sel: ast.NewIdent("NewMap"),
							},
							Indices: []ast.Expr{rewriteType(typ.Key), rewriteType(typ.Value)},
						},
					}, e.Pos())
				case *ast.ArrayType:
					if typ.Len == nil {
						return setPos(&ast.CallExpr{
							Fun: &ast.IndexListExpr{
								X: &ast.SelectorExpr{
									X:   setPos(ast.NewIdent("persistent"), e.Pos()),
									Sel: ast.NewIdent("NewList"),
								},
								Indices: []ast.Expr{rewriteType(typ.Elt)},
							},
						}, e.Pos())
					}
				}
			}
		}

		e.Fun = rewriteExpr(e.Fun, env, versions, false)
		for i, arg := range e.Args {
			e.Args[i] = rewriteExpr(arg, env, versions, false)
		}

		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			// m.Set(k, v) -> m.Set(k, v)
			if sel.Sel.Name == "Set" || sel.Sel.Name == "Append" || sel.Sel.Name == "Delete" {
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
				var res ast.Expr = e.Args[N]
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
				var res ast.Expr = &ast.CallExpr{
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
				var res ast.Expr = &ast.CallExpr{
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
		e.X = rewriteExpr(e.X, env, versions, false)
		return e
	case *ast.IndexExpr:
		method := "Get"
		if wantTwoValues {
			method = "Lookup"
		}
		return setPos(&ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   rewriteExpr(e.X, env, versions, false),
				Sel: ast.NewIdent(method),
			},
			Args: []ast.Expr{rewriteExpr(e.Index, env, versions, false)},
		}, e.Pos())
	case *ast.CompositeLit:
		if mt, ok := e.Type.(*ast.MapType); ok {
			var res ast.Expr = setPos(&ast.CallExpr{
				Fun: &ast.IndexListExpr{
					X: &ast.SelectorExpr{
						X:   setPos(ast.NewIdent("persistent"), e.Pos()),
						Sel: ast.NewIdent("NewMap"),
					},
					Indices: []ast.Expr{rewriteType(mt.Key), rewriteType(mt.Value)},
				},
			}, e.Pos())
			for _, el := range e.Elts {
				kv := el.(*ast.KeyValueExpr)
				res = &ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   res,
						Sel: ast.NewIdent("Set"),
					},
					Args: []ast.Expr{rewriteExpr(kv.Key, env, versions, false), rewriteExpr(kv.Value, env, versions, false)},
				}
			}
			return res
		}
		if st, ok := e.Type.(*ast.ArrayType); ok && st.Len == nil {
			var res ast.Expr = setPos(&ast.CallExpr{
				Fun: &ast.IndexListExpr{
					X: &ast.SelectorExpr{
						X:   setPos(ast.NewIdent("persistent"), e.Pos()),
						Sel: ast.NewIdent("NewList"),
					},
					Indices: []ast.Expr{rewriteType(st.Elt)},
				},
			}, e.Pos())
			for _, el := range e.Elts {
				res = &ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   res,
						Sel: ast.NewIdent("Append"),
					},
					Args: []ast.Expr{rewriteExpr(el, env, versions, false)},
				}
			}
			return res
		}
		return e
	case *ast.FuncLit:
		newEnv := make([]map[string]string, len(env))
		copy(newEnv, env)
		newEnv = append(newEnv, make(map[string]string))
		rewriteBlock(e.Body, newEnv, versions)
		return e
	case *ast.ParenExpr:
		e.X = rewriteExpr(e.X, env, versions, false)
		return e
	case *ast.SliceExpr:
		e.X = rewriteExpr(e.X, env, versions, false)
		if e.Low != nil { e.Low = rewriteExpr(e.Low, env, versions, false) }
		if e.High != nil { e.High = rewriteExpr(e.High, env, versions, false) }
		if e.Max != nil { e.Max = rewriteExpr(e.Max, env, versions, false) }
		return e
	case *ast.TypeAssertExpr:
		e.X = rewriteExpr(e.X, env, versions, false)
		e.Type = rewriteType(e.Type)
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
