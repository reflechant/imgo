package transpiler

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
)

// rewriter threads the mutable state of an AST rewrite pass: the scoped
// environment stack, the monotonic version counter for SSA-style mangling,
// resolved type information, and a flag tracking whether any node in the
// tree ended up needing the persistent package.
type rewriter struct {
	env           []map[string]string
	versions      map[string]int
	info          *types.Info
	hasPersistent bool
}

func Rewrite(file *ast.File, info *types.Info) *ast.File {
	r := &rewriter{
		versions: make(map[string]int),
		info:     info,
	}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Type != nil {
				if d.Type.Params != nil {
					for _, field := range d.Type.Params.List {
						field.Type = r.typ(field.Type)
					}
				}
				if d.Type.Results != nil {
					for _, field := range d.Type.Results.List {
						field.Type = r.typ(field.Type)
					}
				}
			}
			r.env = []map[string]string{make(map[string]string)}
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
							vs.Values[i] = r.expr(val, false)
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
func (r *rewriter) define(name string) string {
	if name == "_" {
		return name
	}
	r.versions[name]++
	mangled := fmt.Sprintf("%s_%d", name, r.versions[name])
	r.env[len(r.env)-1][name] = mangled
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
	res := tail(targets[n-1], keys[n-1])
	for i := n - 2; i >= 0; i-- {
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

// defineIdent mangles an Ident in place when it's a real (non-blank) name.
func (r *rewriter) defineIdent(e ast.Expr) ast.Expr {
	ident, ok := e.(*ast.Ident)
	if !ok || ident.Name == "_" {
		return e
	}
	return &ast.Ident{Name: r.define(ident.Name), NamePos: ident.Pos()}
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
		if wantTwo {
			s.Rhs[0] = r.expr(s.Rhs[0], true)
		} else {
			for j, expr := range s.Rhs {
				s.Rhs[j] = r.expr(expr, false)
			}
		}

		if s.Tok == token.DEFINE {
			for j, lhs := range s.Lhs {
				s.Lhs[j] = r.defineIdent(lhs)
			}
		} else {
			for j, lhs := range s.Lhs {
				s.Lhs[j] = r.expr(lhs, false)
			}
		}
		return s

	case *ast.ExprStmt:
		s.X = r.expr(s.X, false)
		return s
	case *ast.ReturnStmt:
		for i, expr := range s.Results {
			s.Results[i] = r.expr(expr, false)
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
			s.Cond = r.expr(s.Cond, false)
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
				s.Cond = r.expr(s.Cond, false)
			}
			if s.Post != nil {
				s.Post = r.stmt(s.Post)
			}
			r.block(s.Body)
		})
		return s
	case *ast.RangeStmt:
		s.X = r.expr(s.X, false)
		// Desugar 'range x' to 'range x.All()'
		s.X = methodCall(s.X, "All")
		r.scoped(func() {
			if s.Tok == token.DEFINE {
				s.Key = r.defineIdent(s.Key)
				s.Value = r.defineIdent(s.Value)
			} else {
				if s.Key != nil {
					s.Key = r.expr(s.Key, false)
				}
				if s.Value != nil {
					s.Value = r.expr(s.Value, false)
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
				s.Tag = r.expr(s.Tag, false)
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
			s.List[i] = r.expr(expr, false)
		}
		for i, st := range s.Body {
			s.Body[i] = r.stmt(st)
		}
		return s
	case *ast.DeferStmt:
		if rewritten, ok := r.expr(s.Call, false).(*ast.CallExpr); ok {
			s.Call = rewritten
		}
		return s
	case *ast.DeclStmt:
		if d, ok := s.Decl.(*ast.GenDecl); ok && d.Tok == token.VAR {
			for _, spec := range d.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					for i, name := range vs.Names {
						vs.Names[i] = &ast.Ident{Name: r.define(name.Name), NamePos: name.Pos()}
					}
					if vs.Type != nil {
						vs.Type = r.typ(vs.Type)
					}
					for i, val := range vs.Values {
						vs.Values[i] = r.expr(val, false)
					}
				}
			}
		}
		return s
	}
	return stmt
}

func (r *rewriter) expr(expr ast.Expr, wantTwoValues bool) ast.Expr {
	if expr == nil {
		return nil
	}

	switch e := expr.(type) {
	case *ast.Ident:
		if r.env != nil {
			if mangled, ok := r.lookup(e.Name); ok {
				return &ast.Ident{Name: mangled, NamePos: e.Pos()}
			}
		}
		return e
	case *ast.BinaryExpr:
		e.X = r.expr(e.X, false)
		e.Y = r.expr(e.Y, false)
		return e
	case *ast.UnaryExpr:
		e.X = r.expr(e.X, false)
		return e
	case *ast.StarExpr:
		e.X = r.expr(e.X, false)
		return e
	case *ast.MapType, *ast.ArrayType:
		return r.typ(e)
	case *ast.CallExpr:
		return r.callExpr(e, wantTwoValues)
	case *ast.SelectorExpr:
		e.X = r.expr(e.X, false)
		return e
	case *ast.IndexExpr:
		method := "Get"
		if wantTwoValues {
			method = "Lookup"
		}
		return setPos(methodCall(r.expr(e.X, false), method, r.expr(e.Index, false)), e.Pos())
	case *ast.CompositeLit:
		if mt, ok := e.Type.(*ast.MapType); ok {
			r.hasPersistent = true
			res := setPos(&ast.CallExpr{
				Fun: persistentGeneric("NewMap", []ast.Expr{r.typ(mt.Key), r.typ(mt.Value)}, e.Pos()),
			}, e.Pos())
			for _, el := range e.Elts {
				if kv, ok := el.(*ast.KeyValueExpr); ok {
					res = methodCall(res, "Set", r.expr(kv.Key, false), r.expr(kv.Value, false))
				}
			}
			return res
		}
		if st, ok := e.Type.(*ast.ArrayType); ok && st.Len == nil {
			r.hasPersistent = true
			res := setPos(&ast.CallExpr{
				Fun: persistentGeneric("NewList", []ast.Expr{r.typ(st.Elt)}, e.Pos()),
			}, e.Pos())
			for _, el := range e.Elts {
				res = methodCall(res, "Append", r.expr(el, false))
			}
			return res
		}
		// General case (e.g. Structs)
		for i, el := range e.Elts {
			if kv, ok := el.(*ast.KeyValueExpr); ok {
				kv.Value = r.expr(kv.Value, false)
			} else {
				e.Elts[i] = r.expr(el, false)
			}
		}
		return e
	case *ast.FuncLit:
		r.scoped(func() { r.block(e.Body) })
		return e
	case *ast.ParenExpr:
		e.X = r.expr(e.X, false)
		return e
	case *ast.SliceExpr:
		r.hasPersistent = true
		x := r.expr(e.X, false)
		var low, high ast.Expr
		if e.Low != nil {
			low = r.expr(e.Low, false)
		} else {
			low = &ast.BasicLit{Kind: token.INT, Value: "0"}
		}
		if e.High != nil {
			high = r.expr(e.High, false)
		} else {
			high = &ast.CallExpr{Fun: persistentSel("Len", token.NoPos), Args: []ast.Expr{x}}
		}
		return setPos(&ast.CallExpr{
			Fun:  persistentSel("Slice", e.Pos()),
			Args: []ast.Expr{x, low, high},
		}, e.Pos())
	case *ast.TypeAssertExpr:
		e.X = r.expr(e.X, false)
		e.Type = r.typ(e.Type)
		return e
	}

	return expr
}

// callExpr handles the CallExpr branch of expr(). ImGo-specific lowerings
// fire here: value-update builtins (set/get/update/delete, *In forms),
// len/make specialisation, and method-call expansion for SetIn/UpdateIn/
// DeleteIn on persistent maps.
func (r *rewriter) callExpr(e *ast.CallExpr, wantTwoValues bool) ast.Expr {
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
		if imgoBuiltins[ident.Name] && len(e.Args) >= 2 && !isShadowed(r.env, ident.Name) {
			if isStructLike(r.info, e.Args[0]) {
				typeExpr := typeExprFor(typeOf(r.info, e.Args[0]))
				if expanded := expandStructBuiltin(ident.Name, e.Args, typeExpr, e.Pos()); expanded != nil {
					return r.expr(expanded, wantTwoValues)
				}
			}
			if isListLike(r.info, e.Args[0]) || isArrayLike(r.info, e.Args[0]) {
				if expanded := expandListBuiltin(ident.Name, e.Args, e.Pos()); expanded != nil {
					return r.expr(expanded, wantTwoValues)
				}
			}
			if isMapLike(r.info, e.Args[0]) {
				return r.expr(expandMapBuiltin(ident.Name, e.Args, wantTwoValues, e.Pos()), wantTwoValues)
			}
		}

		if ident.Name == "len" && len(e.Args) == 1 {
			r.hasPersistent = true
			return setPos(&ast.CallExpr{
				Fun:  persistentSel("Len", e.Pos()),
				Args: []ast.Expr{r.expr(e.Args[0], false)},
			}, e.Pos())
		}
		if ident.Name == "make" && len(e.Args) >= 1 {
			switch typ := e.Args[0].(type) {
			case *ast.MapType:
				r.hasPersistent = true
				return setPos(&ast.CallExpr{
					Fun: persistentGeneric("NewMap", []ast.Expr{r.typ(typ.Key), r.typ(typ.Value)}, e.Pos()),
				}, e.Pos())
			case *ast.ArrayType:
				if typ.Len == nil {
					r.hasPersistent = true
					return setPos(&ast.CallExpr{
						Fun: persistentGeneric("NewList", []ast.Expr{r.typ(typ.Elt)}, e.Pos()),
					}, e.Pos())
				}
			}
		}
	}

	// Capture the receiver before recursive rewriting, since the child
	// rewrite may replace sel.X with a freshly-mangled ident that has no
	// entry in info.
	var origReceiver ast.Expr
	if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
		origReceiver = sel.X
	}

	e.Fun = r.expr(e.Fun, false)
	for i, arg := range e.Args {
		e.Args[i] = r.expr(arg, false)
	}

	sel, isSel := e.Fun.(*ast.SelectorExpr)
	if !isSel {
		return e
	}
	switch sel.Sel.Name {
	case "Set", "Append", "Delete":
		return e
	}
	// SetIn / UpdateIn / DeleteIn only apply to map-typed receivers. When
	// type info shows the receiver is some other type (e.g. a user struct
	// that defines a method by the same name), leave the call alone.
	if !isMapLike(r.info, origReceiver) {
		return e
	}
	switch sel.Sel.Name {
	case "SetIn":
		keys := e.Args[:len(e.Args)-1]
		value := e.Args[len(e.Args)-1]
		return setPos(expandInChain(sel.X, keys, func(x, k ast.Expr) ast.Expr {
			return methodCall(x, "Set", k, value)
		}), e.Pos())
	case "UpdateIn":
		keys := e.Args[:len(e.Args)-1]
		fn := e.Args[len(e.Args)-1]
		return setPos(expandInChain(sel.X, keys, func(x, k ast.Expr) ast.Expr {
			return methodCall(x, "Update", k, fn)
		}), e.Pos())
	case "DeleteIn":
		return setPos(expandInChain(sel.X, e.Args, func(x, k ast.Expr) ast.Expr {
			return methodCall(x, "Delete", k)
		}), e.Pos())
	}
	return e
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
