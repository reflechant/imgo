package transpiler

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// rewriter threads the mutable state of an AST rewrite pass: the scoped
// environment stack, the monotonic version counter for SSA-style mangling,
// resolved type information, and a flag tracking whether any node in the
// tree ended up needing the persistent package.
type rewriter struct {
	env           []map[string]string
	types         map[string]types.Type
	versions      map[string]int
	info          *types.Info
	hasPersistent bool
}

// Rewrite performs the core AST transformation of an ImGo source file into Go.
// It mangles identifiers for SSA-style immutability and desugars persistent operations.
func Rewrite(file *ast.File, info *types.Info) *ast.File {
	r := &rewriter{
		versions: make(map[string]int),
		types:    make(map[string]types.Type),
		info:     info,
	}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			r.env = []map[string]string{make(map[string]string)}
			if d.Type != nil {
				if d.Type.Params != nil {
					for _, field := range d.Type.Params.List {
						for i, name := range field.Names {
							if id, ok := r.defineIdent(name).(*ast.Ident); ok {
								field.Names[i] = id
							}
						}
						field.Type = r.typ(field.Type)
					}
				}
				if d.Type.Results != nil {
					for _, field := range d.Type.Results.List {
						field.Type = r.typ(field.Type)
					}
				}
			}
			r.block(d.Body)
		case *ast.GenDecl:
			if d.Tok == token.VAR {
				r.env = nil
				for _, spec := range d.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						if vs.Type != nil {
							vs.Type = r.typ(vs.Type)
						}
						for i, val := range vs.Values {
							vs.Values[i], _ = r.expr(val, false)
						}
					}
				}
			}
			if d.Tok == token.TYPE {
				for _, spec := range d.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						if st, ok := ts.Type.(*ast.StructType); ok {
							for _, field := range st.Fields.List {
								field.Type = r.typ(field.Type)
							}
						}
					}
				}
			}
		}
	}

	if r.hasPersistent {
		addPersistentImport(file)
	}

	return file
}

func (r *rewriter) push() {
	r.env = append(r.env, make(map[string]string))
}

func (r *rewriter) pop() {
	r.env = r.env[:len(r.env)-1]
}

func (r *rewriter) scoped(fn func()) {
	r.push()
	fn()
	r.pop()
}

// define introduces a fresh binding for name in the top frame and returns
// the mangled form. "_" is passed through without bumping the version.
func (r *rewriter) define(name string, typ types.Type) string {
	if name == "_" {
		return name
	}
	r.versions[name]++
	mangled := fmt.Sprintf("%s_%d", name, r.versions[name])
	r.env[len(r.env)-1][name] = mangled
	if typ != nil {
		r.types[mangled] = typ
	}

	return mangled
}

// lookup resolves name to its latest mangled form by walking the scope
// stack bottom-up. Reports whether a binding was found.
func (r *rewriter) lookup(name string) (string, bool) {
	for i := len(r.env) - 1; i >= 0; i-- {
		if m, ok := r.env[i][name]; ok {
			return m, true
		}
	}

	return "", false
}

// methodCall builds recv.name(args...). No position information is set;
// callers that care wrap the result in setPos.
func methodCall(recv ast.Expr, name string, args ...ast.Expr) *ast.CallExpr {
	return &ast.CallExpr{
		Fun:  &ast.SelectorExpr{X: recv, Sel: ast.NewIdent(name)},
		Args: args,
	}
}

// persistentSel builds the selector `persistent.Name` and pins pos onto the
// persistent identifier so downstream printers emit sensible locations.
func persistentSel(name string, pos token.Pos) *ast.SelectorExpr {
	return &ast.SelectorExpr{
		X:   setPos(ast.NewIdent("persistent"), pos),
		Sel: ast.NewIdent(name),
	}
}

// persistentGeneric builds `persistent.Name[Idx0, Idx1, ...]`. Used both
// for generic types (Map, List) and generic constructors (NewMap, NewList).
func persistentGeneric(name string, indices []ast.Expr, pos token.Pos) ast.Expr {
	return setPos(&ast.IndexListExpr{
		X:       persistentSel(name, pos),
		Indices: indices,
		Lbrack:  pos,
	}, pos)
}

// expandInChain lowers SetIn/UpdateIn/DeleteIn into a nested Set(..., Get(...))
// chain. keys carries the full path including the innermost key. tail is
// invoked once on the innermost receiver and its key to construct the
// bottom-level operation (Set / Update / Delete); outer levels are always
// wrapped in Set(key, ...).
func expandInChain(recv ast.Expr, keys []ast.Expr, tail func(x, k ast.Expr) ast.Expr) ast.Expr {
	targets := make([]ast.Expr, len(keys))
	targets[0] = recv
	for i := 1; i < len(keys); i++ {
		targets[i] = methodCall(targets[i-1], "Get", keys[i-1])
	}
	n := len(keys)
	last := n - 1
	res := tail(targets[last], keys[last])
	for i := last - 1; i >= 0; i-- {
		res = methodCall(targets[i], "Set", keys[i], res)
	}

	return res
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

	file.Decls = append([]ast.Decl{&ast.GenDecl{
		Tok:   token.IMPORT,
		Specs: []ast.Spec{newImport},
	}}, file.Decls...)
}

func (r *rewriter) typ(typ ast.Expr) ast.Expr {
	if typ == nil {
		return nil
	}
	switch t := typ.(type) {
	case *ast.MapType:
		r.hasPersistent = true

		return persistentGeneric("Map", []ast.Expr{r.typ(t.Key), r.typ(t.Value)}, t.Pos())
	case *ast.ArrayType:
		if t.Len == nil {
			r.hasPersistent = true

			return persistentGeneric("List", []ast.Expr{r.typ(t.Elt)}, t.Pos())
		}
	}

	return typ
}

// block iterates stmt-by-stmt without creating a new scope; callers are
// responsible for scope management. stmt() handles lone BlockStmt by
// pushing a frame and delegating to block().
func (r *rewriter) block(b *ast.BlockStmt) {
	if b == nil {
		return
	}
	for i, stmt := range b.List {
		b.List[i] = r.stmt(stmt)
	}
}

// typeOf returns the resolved type of x, prioritizing rewriter's inferred types.
func (r *rewriter) typeOf(x ast.Expr) types.Type {
	if ident, ok := x.(*ast.Ident); ok {
		name := ident.Name
		if mangled, ok := r.lookup(name); ok {
			name = mangled
		}
		if t, ok := r.types[name]; ok {
			return t
		}
	}

	return typeOf(r.info, x)
}

func (r *rewriter) isArrayLike(x ast.Expr) bool {
	t := r.typeOf(x)
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Array)

	return ok
}

func (r *rewriter) isListLike(x ast.Expr) bool {
	t := r.typeOf(x)
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Slice)

	return ok
}

func (r *rewriter) isStructLike(x ast.Expr) bool {
	t := r.typeOf(x)
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Struct)

	return ok
}

func (r *rewriter) isMapLike(x ast.Expr) bool {
	t := r.typeOf(x)
	if t == nil {
		// Fallback for untyped literals during rewrite.
		return true
	}
	_, ok := t.Underlying().(*types.Map)

	return ok
}

// defineIdent mangles an Ident in place when it's a real (non-blank) name.
func (r *rewriter) defineIdent(e ast.Expr) ast.Expr {
	ident, ok := e.(*ast.Ident)
	if !ok || ident.Name == "_" {
		return e
	}

	return &ast.Ident{Name: r.define(ident.Name, typeOf(r.info, ident)), NamePos: ident.Pos()}
}

func (r *rewriter) stmt(stmt ast.Stmt) ast.Stmt {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		wantTwo := false
		if len(s.Lhs) == 2 && len(s.Rhs) == 1 {
			if _, ok := s.Rhs[0].(*ast.IndexExpr); ok {
				wantTwo = true
			} else if call, ok := s.Rhs[0].(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok && (id.Name == "get" || id.Name == "getIn") && !isShadowed(r.env, id.Name) {
					wantTwo = true
				}
			}
		}

		rhsTypes := make([]types.Type, len(s.Rhs))
		if wantTwo {
			var t types.Type
			s.Rhs[0], t = r.expr(s.Rhs[0], true)
			rhsTypes[0] = t
		} else {
			for j, expr := range s.Rhs {
				var t types.Type
				s.Rhs[j], t = r.expr(expr, false)
				rhsTypes[j] = t
			}
		}

		if s.Tok == token.DEFINE {
			for j, lhs := range s.Lhs {
				var t types.Type
				if wantTwo {
					if j == 0 {
						t = rhsTypes[0]
					}
				} else if len(rhsTypes) == len(s.Lhs) {
					t = rhsTypes[j]
				}
				s.Lhs[j] = r.defineIdentWithType(lhs, t)
			}
		} else {
			for j, lhs := range s.Lhs {
				s.Lhs[j], _ = r.expr(lhs, false)
			}
		}

		return s

	case *ast.ExprStmt:
		s.X, _ = r.expr(s.X, false)

		return s
	case *ast.ReturnStmt:
		for i, expr := range s.Results {
			s.Results[i], _ = r.expr(expr, false)
		}

		return s
	case *ast.BlockStmt:
		r.scoped(func() { r.block(s) })

		return s
	case *ast.IfStmt:
		r.scoped(func() {
			if s.Init != nil {
				s.Init = r.stmt(s.Init)
			}
			s.Cond, _ = r.expr(s.Cond, false)
			r.block(s.Body)
			if s.Else != nil {
				if els, ok := s.Else.(*ast.BlockStmt); ok {
					r.scoped(func() { r.block(els) })
				} else {
					s.Else = r.stmt(s.Else)
				}
			}
		})

		return s
	case *ast.ForStmt:
		r.scoped(func() {
			if s.Init != nil {
				s.Init = r.stmt(s.Init)
			}
			if s.Cond != nil {
				s.Cond, _ = r.expr(s.Cond, false)
			}
			if s.Post != nil {
				s.Post = r.stmt(s.Post)
			}
			r.block(s.Body)
		})

		return s
	case *ast.RangeStmt:
		s.X, _ = r.expr(s.X, false)
		// Desugar 'range x' to 'range x.All()'
		s.X = methodCall(s.X, "All")
		r.scoped(func() {
			if s.Tok == token.DEFINE {
				s.Key = r.defineIdent(s.Key)
				s.Value = r.defineIdent(s.Value)
			} else {
				if s.Key != nil {
					s.Key, _ = r.expr(s.Key, false)
				}
				if s.Value != nil {
					s.Value, _ = r.expr(s.Value, false)
				}
			}
			r.block(s.Body)
		})

		return s
	case *ast.SwitchStmt:
		r.scoped(func() {
			if s.Init != nil {
				s.Init = r.stmt(s.Init)
			}
			if s.Tag != nil {
				s.Tag, _ = r.expr(s.Tag, false)
			}
			r.block(s.Body)
		})

		return s
	case *ast.TypeSwitchStmt:
		r.scoped(func() {
			if s.Init != nil {
				s.Init = r.stmt(s.Init)
			}
			s.Assign = r.stmt(s.Assign)
			r.block(s.Body)
		})

		return s
	case *ast.CaseClause:
		for i, expr := range s.List {
			s.List[i], _ = r.expr(expr, false)
		}
		for i, st := range s.Body {
			s.Body[i] = r.stmt(st)
		}

		return s
	case *ast.DeferStmt:
		res, _ := r.expr(s.Call, false)
		if rewritten, ok := res.(*ast.CallExpr); ok {
			s.Call = rewritten
		}

		return s
	case *ast.DeclStmt:
		if d, ok := s.Decl.(*ast.GenDecl); ok {
			if d.Tok == token.VAR {
				for _, spec := range d.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						if vs.Type != nil {
							vs.Type = r.typ(vs.Type)
						}
						for i, name := range vs.Names {
							vs.Names[i] = &ast.Ident{Name: r.define(name.Name, typeOf(r.info, name)), NamePos: name.Pos()}
						}
						for i, val := range vs.Values {
							vs.Values[i], _ = r.expr(val, false)
						}
					}
				}
			}
			if d.Tok == token.TYPE {
				for _, spec := range d.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						if st, ok := ts.Type.(*ast.StructType); ok {
							for _, field := range st.Fields.List {
								field.Type = r.typ(field.Type)
							}
						}
					}
				}
			}
		}

		return s
	}

	return stmt
}

func (r *rewriter) defineIdentWithType(e ast.Expr, typ types.Type) ast.Expr {
	ident, ok := e.(*ast.Ident)
	if !ok || ident.Name == "_" {
		return e
	}
	t := typ
	if t == nil {
		t = typeOf(r.info, ident)
	}

	return &ast.Ident{Name: r.define(ident.Name, t), NamePos: ident.Pos()}
}

func (r *rewriter) expr(expr ast.Expr, wantTwoValues bool) (ast.Expr, types.Type) {
	if expr == nil {
		return nil, nil
	}

	switch e := expr.(type) {
	case *ast.Ident:
		if r.env != nil {
			if mangled, ok := r.lookup(e.Name); ok {
				return &ast.Ident{Name: mangled, NamePos: e.Pos()}, r.types[mangled]
			}
		}

		return e, typeOf(r.info, e)
	case *ast.BinaryExpr:
		e.X, _ = r.expr(e.X, false)
		e.Y, _ = r.expr(e.Y, false)

		return e, typeOf(r.info, e)
	case *ast.UnaryExpr:
		e.X, _ = r.expr(e.X, false)

		return e, typeOf(r.info, e)
	case *ast.StarExpr:
		e.X, _ = r.expr(e.X, false)

		return e, typeOf(r.info, e)
	case *ast.MapType, *ast.ArrayType:
		return r.typ(e), typeOf(r.info, e)
	case *ast.CallExpr:
		return r.callExpr(e, wantTwoValues)
	case *ast.SelectorExpr:
		e.X, _ = r.expr(e.X, false)

		return e, typeOf(r.info, e)
	case *ast.IndexExpr:
		x, _ := r.expr(e.X, false)
		if r.isArrayLike(x) {
			e.X = x
			e.Index, _ = r.expr(e.Index, false)

			return e, r.typeOf(e)
		}
		method := "Get"
		if wantTwoValues {
			method = "Lookup"
		}
		res := setPos(methodCall(x, method, fst(r.expr(e.Index, false))), e.Pos())

		return res, typeOf(r.info, e)
	case *ast.CompositeLit:
		if mt, ok := e.Type.(*ast.MapType); ok {
			r.hasPersistent = true
			res := setPos(&ast.CallExpr{
				Fun: persistentGeneric("NewMap", []ast.Expr{r.typ(mt.Key), r.typ(mt.Value)}, e.Pos()),
			}, e.Pos())
			for _, el := range e.Elts {
				if kv, ok := el.(*ast.KeyValueExpr); ok {
					res = methodCall(res, "Set", fst(r.expr(kv.Key, false)), fst(r.expr(kv.Value, false)))
				}
			}

			return res, typeOf(r.info, e)
		}
		if st, ok := e.Type.(*ast.ArrayType); ok && st.Len == nil {
			r.hasPersistent = true
			res := setPos(&ast.CallExpr{
				Fun: persistentGeneric("NewList", []ast.Expr{r.typ(st.Elt)}, e.Pos()),
			}, e.Pos())
			for _, el := range e.Elts {
				res = methodCall(res, "Append", fst(r.expr(el, false)))
			}

			return res, typeOf(r.info, e)
		}
		// Implicit-type composite literal (e.Type == nil): the type is inferred
		// from context by go/types (e.g. the {1,2} in map[string][]int{"k":{1,2}}).
		// Dispatch to the same persistent rewrite as the explicit-type cases above.
		if e.Type == nil {
			if inferred := typeOf(r.info, e); inferred != nil {
				if u, ok := inferred.Underlying().(*types.Map); ok {
					if keyExpr, valExpr := typeExprFor(u.Key()), typeExprFor(u.Elem()); keyExpr != nil && valExpr != nil {
						r.hasPersistent = true
						res := setPos(&ast.CallExpr{
							Fun: persistentGeneric("NewMap", []ast.Expr{r.typ(keyExpr), r.typ(valExpr)}, e.Pos()),
						}, e.Pos())
						for _, el := range e.Elts {
							if kv, ok := el.(*ast.KeyValueExpr); ok {
								res = methodCall(res, "Set", fst(r.expr(kv.Key, false)), fst(r.expr(kv.Value, false)))
							}
						}

						return res, typeOf(r.info, e)
					}
				}
				if u, ok := inferred.Underlying().(*types.Slice); ok {
					if eltExpr := typeExprFor(u.Elem()); eltExpr != nil {
						r.hasPersistent = true
						res := setPos(&ast.CallExpr{
							Fun: persistentGeneric("NewList", []ast.Expr{r.typ(eltExpr)}, e.Pos()),
						}, e.Pos())
						for _, el := range e.Elts {
							res = methodCall(res, "Append", fst(r.expr(el, false)))
						}

						return res, typeOf(r.info, e)
					}
				}
				// Struct or other named type: add explicit type so the generated Go
				// is valid when this literal appears as a function argument (Go only
				// permits omitting the type inside another composite literal, not as args).
				if typeExpr := typeExprFor(inferred); typeExpr != nil {
					e.Type = typeExpr
				}
			}
		}
		// General case (structs, explicit-length arrays)
		for i, el := range e.Elts {
			if kv, ok := el.(*ast.KeyValueExpr); ok {
				kv.Value, _ = r.expr(kv.Value, false)
			} else {
				e.Elts[i], _ = r.expr(el, false)
			}
		}

		return e, typeOf(r.info, e)
	case *ast.FuncLit:
		r.scoped(func() {
			if e.Type != nil && e.Type.Params != nil {
				for _, field := range e.Type.Params.List {
					for i, name := range field.Names {
						if id, ok := r.defineIdent(name).(*ast.Ident); ok {
							field.Names[i] = id
						}
					}
					field.Type = r.typ(field.Type)
				}
			}
			r.block(e.Body)
		})

		return e, typeOf(r.info, e)
	case *ast.ParenExpr:
		e.X, _ = r.expr(e.X, false)

		return e, typeOf(r.info, e)
	case *ast.SliceExpr:
		r.hasPersistent = true
		x, _ := r.expr(e.X, false)
		var low, high ast.Expr
		if e.Low != nil {
			low, _ = r.expr(e.Low, false)
		} else {
			low = &ast.BasicLit{Kind: token.INT, Value: "0"}
		}
		if e.High != nil {
			high, _ = r.expr(e.High, false)
		} else {
			high = &ast.CallExpr{Fun: persistentSel("Len", token.NoPos), Args: []ast.Expr{x}}
		}
		res := setPos(&ast.CallExpr{
			Fun:  persistentSel("Slice", e.Pos()),
			Args: []ast.Expr{x, low, high},
		}, e.Pos())

		return res, typeOf(r.info, e)
	case *ast.TypeAssertExpr:
		e.X, _ = r.expr(e.X, false)
		e.Type = r.typ(e.Type)

		return e, typeOf(r.info, e)
	}

	return expr, typeOf(r.info, expr)
}

func fst[T any, U any](t T, _ U) T {
	return t
}

// callExpr handles the CallExpr branch of expr(). ImGo-specific lowerings
// fire here: value-update builtins (set/get/update/delete, *In forms),
// len/make specialisation, and method-call expansion for SetIn/UpdateIn/
// DeleteIn on persistent maps.
func (r *rewriter) callExpr(e *ast.CallExpr, wantTwoValues bool) (ast.Expr, types.Type) {
	// ImGo value-update builtins (set/get/update/delete and their *In
	// forms). Dispatch on the receiver's static type:
	//   - struct: lower get/getIn to selector chains; lower update/updateIn
	//     to an IIFE that copies the struct and overwrites the field.
	//   - list (slice): translate to method-call shape using persistent.List's
	//     .Set/.Get; In-forms reuse the existing map In expansion below.
	//   - map (or unknown): translate to method-call shape and reuse the
	//     existing map lowering.
	// Builtin recognition is suppressed when the name is shadowed by a local
	// binding (Go's own len/append shadowing rule).
	if ident, ok := e.Fun.(*ast.Ident); ok {
		name := ident.Name
		if imgoBuiltins[name] && len(e.Args) >= 2 && !isShadowed(r.env, name) {
			if r.isStructLike(e.Args[0]) {
				receiverType := r.typeOf(e.Args[0])
				typeExpr := typeExprFor(receiverType)
				if expanded := expandStructBuiltin(name, e.Args, typeExpr, e.Pos()); expanded != nil {
					return fst(r.expr(expanded, wantTwoValues)), receiverType
				}
			}
			if r.isArrayLike(e.Args[0]) {
				receiverType := r.typeOf(e.Args[0])
				typeExpr := typeExprFor(receiverType)
				expanded := expandArrayBuiltin(
					name, e.Args, typeExpr, e.Pos(),
					func(ex ast.Expr) ast.Expr { return fst(r.expr(ex, false)) },
				)
				if expanded != nil {
					return expanded, receiverType
				}
			}
			if r.isListLike(e.Args[0]) {
				receiverType := r.typeOf(e.Args[0])
				if expanded := expandListBuiltin(name, e.Args, e.Pos()); expanded != nil {
					return fst(r.expr(expanded, wantTwoValues)), receiverType
				}
			}
			if r.isMapLike(e.Args[0]) {
				receiverType := r.typeOf(e.Args[0])

				return fst(r.expr(expandMapBuiltin(name, e.Args, wantTwoValues, e.Pos()), wantTwoValues)), receiverType
			}
		}

		if ident.Name == "len" && len(e.Args) == 1 {
			arg, _ := r.expr(e.Args[0], false)
			if r.isArrayLike(arg) {
				// Standard Go len(array) is pure and works fine.
				res := setPos(&ast.CallExpr{
					Fun:  ast.NewIdent("len"),
					Args: []ast.Expr{arg},
				}, e.Pos())

				return res, types.Typ[types.Int]
			}
			r.hasPersistent = true
			res := setPos(&ast.CallExpr{
				Fun:  persistentSel("Len", e.Pos()),
				Args: []ast.Expr{arg},
			}, e.Pos())

			return res, types.Typ[types.Int]
		}
		if ident.Name == "make" && len(e.Args) >= 1 {
			switch typ := e.Args[0].(type) {
			case *ast.MapType:
				r.hasPersistent = true
				res := setPos(&ast.CallExpr{
					Fun: persistentGeneric("NewMap", []ast.Expr{r.typ(typ.Key), r.typ(typ.Value)}, e.Pos()),
				}, e.Pos())

				return res, typeOf(r.info, typ)
			case *ast.ArrayType:
				if typ.Len == nil {
					r.hasPersistent = true
					res := setPos(&ast.CallExpr{
						Fun: persistentGeneric("NewList", []ast.Expr{r.typ(typ.Elt)}, e.Pos()),
					}, e.Pos())

					return res, typeOf(r.info, typ)
				}
			}
		}
	}

	e.Fun, _ = r.expr(e.Fun, false)
	for i, arg := range e.Args {
		e.Args[i], _ = r.expr(arg, false)
	}

	sel, isSel := e.Fun.(*ast.SelectorExpr)
	if !isSel {
		return e, typeOf(r.info, e)
	}

	// Dispatch on rewritten receiver's type for methods like .Set, .Append, etc.
	receiver := sel.X
	methodName := sel.Sel.Name
	switch methodName {
	case "Set", "Append", "Update", "Delete":
		if r.isArrayLike(receiver) {
			receiverType := r.typeOf(receiver)
			typeExpr := typeExprFor(receiverType)
			builtinName := strings.ToLower(methodName)
			args := append([]ast.Expr{receiver}, e.Args...)
			expanded := expandArrayBuiltin(builtinName, args, typeExpr, e.Pos(), func(ex ast.Expr) ast.Expr { return ex })
			if expanded != nil {
				return expanded, receiverType
			}
		}
		if r.isListLike(receiver) {
			receiverType := r.typeOf(receiver)
			builtinName := strings.ToLower(methodName)
			args := append([]ast.Expr{receiver}, e.Args...)
			if expanded := expandListBuiltin(builtinName, args, e.Pos()); expanded != nil {
				return expanded, receiverType
			}
		}

		return e, typeOf(r.info, e)
	}

	// SetIn / UpdateIn / DeleteIn only apply to map-typed receivers.
	if !r.isMapLike(receiver) {
		return e, typeOf(r.info, e)
	}
	const minInArgs = 2
	switch methodName {
	case "SetIn":
		if len(e.Args) < minInArgs {
			return e, typeOf(r.info, e)
		}
		keys := e.Args[:len(e.Args)-1]
		value := e.Args[len(e.Args)-1]
		res := setPos(expandInChain(receiver, keys, func(x, k ast.Expr) ast.Expr {
			return methodCall(x, "Set", k, value)
		}), e.Pos())

		return res, typeOf(r.info, e)
	case "UpdateIn":
		if len(e.Args) < minInArgs {
			return e, typeOf(r.info, e)
		}
		keys := e.Args[:len(e.Args)-1]
		fn := e.Args[len(e.Args)-1]
		res := setPos(expandInChain(receiver, keys, func(x, k ast.Expr) ast.Expr {
			return methodCall(x, "Update", k, fn)
		}), e.Pos())

		return res, typeOf(r.info, e)
	case "DeleteIn":
		if len(e.Args) == 0 {
			return e, typeOf(r.info, e)
		}
		res := setPos(expandInChain(receiver, e.Args, func(x, k ast.Expr) ast.Expr {
			return methodCall(x, "Delete", k)
		}), e.Pos())

		return res, typeOf(r.info, e)
	}

	return e, typeOf(r.info, e)
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
