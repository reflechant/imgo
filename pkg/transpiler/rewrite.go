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
			counter := new(int)
			rewriteBlock(d.Body, env, counter)
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
							vs.Values[i] = rewriteExpr(val, nil, new(int), false)
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
		return &ast.IndexListExpr{
			X: &ast.SelectorExpr{
				X:   setPos(ast.NewIdent("persistent"), t.Pos()),
				Sel: ast.NewIdent("Map"),
			},
			Indices: []ast.Expr{rewriteType(t.Key), rewriteType(t.Value)},
			Lbrack:  t.Pos(),
		}
	case *ast.ArrayType:
		if t.Len == nil {
			return &ast.IndexListExpr{
				X: &ast.SelectorExpr{
					X:   setPos(ast.NewIdent("persistent"), t.Pos()),
					Sel: ast.NewIdent("List"),
				},
				Indices: []ast.Expr{rewriteType(t.Elt)},
				Lbrack:  t.Pos(),
			}
		}
	}
	return typ
}

func rewriteBlock(block *ast.BlockStmt, env []map[string]string, counter *int) {
	if block == nil {
		return
	}

	for i, stmt := range block.List {
		block.List[i] = rewriteStmt(stmt, env, counter)
	}
}

func rewriteStmt(stmt ast.Stmt, env []map[string]string, counter *int) ast.Stmt {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		// Specialized check for 2-value indexing context: v, ok := m[k]
		if len(s.Lhs) == 2 && len(s.Rhs) == 1 {
			if _, ok := s.Rhs[0].(*ast.IndexExpr); ok {
				s.Rhs[0] = rewriteExpr(s.Rhs[0], env, counter, true) // Pass true for wantTwoValues
				goto processLHS
			}
		}

		// Default processing
		for j, expr := range s.Rhs {
			s.Rhs[j] = rewriteExpr(expr, env, counter, false)
		}

	processLHS:
		if s.Tok == token.DEFINE {
			newNames := make([]ast.Expr, len(s.Lhs))
			for j, lhs := range s.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok {
					*counter++
					mangled := fmt.Sprintf("%s_%d", ident.Name, *counter)
					env[len(env)-1][ident.Name] = mangled
					newNames[j] = &ast.Ident{Name: mangled, NamePos: ident.Pos()}
				} else {
					newNames[j] = lhs
				}
			}
			s.Lhs = newNames
		}
		return s

	case *ast.ExprStmt:
		s.X = rewriteExpr(s.X, env, counter, false)
		return s
	case *ast.ReturnStmt:
		for i, expr := range s.Results {
			s.Results[i] = rewriteExpr(expr, env, counter, false)
		}
		return s
	case *ast.BlockStmt:
		newEnv := make([]map[string]string, len(env))
		copy(newEnv, env)
		newEnv = append(newEnv, make(map[string]string))
		rewriteBlock(s, newEnv, counter)
		return s
	case *ast.IfStmt:
		if s.Init != nil { s.Init = rewriteStmt(s.Init, env, counter) }
		s.Cond = rewriteExpr(s.Cond, env, counter, false)
		bodyEnv := make([]map[string]string, len(env))
		copy(bodyEnv, env)
		bodyEnv = append(bodyEnv, make(map[string]string))
		rewriteBlock(s.Body, bodyEnv, counter)
		if s.Else != nil {
			elseEnv := make([]map[string]string, len(env))
			copy(elseEnv, env)
			elseEnv = append(elseEnv, make(map[string]string))
			if els, ok := s.Else.(*ast.BlockStmt); ok {
				rewriteBlock(els, elseEnv, counter)
			} else {
				s.Else = rewriteStmt(s.Else, env, counter)
			}
		}
		return s
	case *ast.RangeStmt:
		s.X = rewriteExpr(s.X, env, counter, false)
		bodyEnv := make([]map[string]string, len(env))
		copy(bodyEnv, env)
		bodyEnv = append(bodyEnv, make(map[string]string))
		rewriteBlock(s.Body, bodyEnv, counter)
		return s
	case *ast.SwitchStmt:
		if s.Init != nil { s.Init = rewriteStmt(s.Init, env, counter) }
		if s.Tag != nil { s.Tag = rewriteExpr(s.Tag, env, counter, false) }
		rewriteBlock(s.Body, env, counter)
		return s
	case *ast.CaseClause:
		for i, expr := range s.List {
			s.List[i] = rewriteExpr(expr, env, counter, false)
		}
		for i, stmt := range s.Body {
			s.Body[i] = rewriteStmt(stmt, env, counter)
		}
		return s
	case *ast.DeferStmt:
		s.Call = rewriteExpr(s.Call, env, counter, false).(*ast.CallExpr)
		return s
	case *ast.DeclStmt:
		if d, ok := s.Decl.(*ast.GenDecl); ok && d.Tok == token.VAR {
			for _, spec := range d.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					for i, name := range vs.Names {
						*counter++
						mangled := fmt.Sprintf("%s_%d", name.Name, *counter)
						env[len(env)-1][name.Name] = mangled
						vs.Names[i] = &ast.Ident{Name: mangled, NamePos: name.Pos()}
					}
					if vs.Type != nil { vs.Type = rewriteType(vs.Type) }
					for i, val := range vs.Values {
						vs.Values[i] = rewriteExpr(val, env, counter, false)
					}
				}
			}
		}
		return s
	}
	return stmt
}

func rewriteExpr(expr ast.Expr, env []map[string]string, counter *int, wantTwoValues bool) ast.Expr {
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
		e.X = rewriteExpr(e.X, env, counter, false)
		e.Y = rewriteExpr(e.Y, env, counter, false)
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
					Args: []ast.Expr{rewriteExpr(e.Args[0], env, counter, false)},
				}, e.Pos())
			}
		}

		e.Fun = rewriteExpr(e.Fun, env, counter, false)
		for i, arg := range e.Args {
			e.Args[i] = rewriteExpr(arg, env, counter, false)
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
		e.X = rewriteExpr(e.X, env, counter, false)
		return e
	case *ast.IndexExpr:
		method := "Get"
		if wantTwoValues {
			method = "Lookup"
		}
		return setPos(&ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   rewriteExpr(e.X, env, counter, false),
				Sel: ast.NewIdent(method),
			},
			Args: []ast.Expr{rewriteExpr(e.Index, env, counter, false)},
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
					Args: []ast.Expr{rewriteExpr(kv.Key, env, counter, false), rewriteExpr(kv.Value, env, counter, false)},
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
					Args: []ast.Expr{rewriteExpr(el, env, counter, false)},
				}
			}
			return res
		}
		return e
	case *ast.FuncLit:
		newEnv := make([]map[string]string, len(env))
		copy(newEnv, env)
		newEnv = append(newEnv, make(map[string]string))
		rewriteBlock(e.Body, newEnv, counter)
		return e
	case *ast.ParenExpr:
		e.X = rewriteExpr(e.X, env, counter, false)
		return e
	case *ast.SliceExpr:
		e.X = rewriteExpr(e.X, env, counter, false)
		if e.Low != nil { e.Low = rewriteExpr(e.Low, env, counter, false) }
		if e.High != nil { e.High = rewriteExpr(e.High, env, counter, false) }
		if e.Max != nil { e.Max = rewriteExpr(e.Max, env, counter, false) }
		return e
	case *ast.TypeAssertExpr:
		e.X = rewriteExpr(e.X, env, counter, false)
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
