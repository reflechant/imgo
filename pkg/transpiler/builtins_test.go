package transpiler

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuiltinEdgeCases(t *testing.T) {
	t.Parallel()
	t.Run("expandListBuiltin unsupported", func(t *testing.T) {
		t.Parallel()
		res := expandListBuiltin("delete", []ast.Expr{ast.NewIdent("l")}, token.NoPos)
		assert.Nil(t, res)
	})

	t.Run("expandArrayBuiltin unsupported", func(t *testing.T) {
		t.Parallel()
		res := expandArrayBuiltin("delete", []ast.Expr{ast.NewIdent("a")}, nil,
			token.NoPos, func(e ast.Expr) ast.Expr { return e })
		assert.Nil(t, res)

		// update with nil typeExpr
		res = expandArrayBuiltin("update", []ast.Expr{
			ast.NewIdent("a"), ast.NewIdent("i"), ast.NewIdent("f"),
		}, nil, token.NoPos, func(e ast.Expr) ast.Expr { return e })
		assert.Nil(t, res)
	})

	t.Run("expandArrayBuiltin insufficient args", func(t *testing.T) {
		t.Parallel()
		id := func(e ast.Expr) ast.Expr { return e }

		// get with only receiver (no index)
		res := expandArrayBuiltin(builtinGet, []ast.Expr{ast.NewIdent("a")}, nil, token.NoPos, id)
		assert.Nil(t, res)

		// update with only receiver + index (missing fn)
		res = expandArrayBuiltin(builtinUpdate, []ast.Expr{
			ast.NewIdent("a"), ast.NewIdent("i"),
		}, ast.NewIdent("T"), token.NoPos, id)
		assert.Nil(t, res)
	})

	t.Run("expandStructBuiltin edge cases", func(t *testing.T) {
		t.Parallel()
		// get with non-string lit
		res := expandStructBuiltin("get", []ast.Expr{ast.NewIdent("s"), ast.NewIdent("notalit")}, nil, token.NoPos)
		assert.Nil(t, res)

		// getIn with non-string lit
		res = expandStructBuiltin("getIn", []ast.Expr{
			ast.NewIdent("s"), &ast.BasicLit{Kind: token.STRING, Value: `"A"`}, ast.NewIdent("notalit"),
		}, nil, token.NoPos)
		assert.Nil(t, res)

		// update with non-string lit
		res = expandStructBuiltin("update", []ast.Expr{
			ast.NewIdent("s"), ast.NewIdent("notalit"), ast.NewIdent("f"),
		}, ast.NewIdent("T"), token.NoPos)
		assert.Nil(t, res)

		// updateIn with non-string lit
		res = expandStructBuiltin("updateIn", []ast.Expr{
			ast.NewIdent("s"), &ast.BasicLit{Kind: token.STRING, Value: `"A"`}, ast.NewIdent("notalit"), ast.NewIdent("f"),
		}, ast.NewIdent("T"), token.NoPos)
		assert.Nil(t, res)

		// update with nil typeExpr
		res = expandStructBuiltin("update", []ast.Expr{
			ast.NewIdent("s"), &ast.BasicLit{Kind: token.STRING, Value: `"A"`}, ast.NewIdent("f"),
		}, nil, token.NoPos)
		assert.Nil(t, res)

		// updateIn with nil typeExpr
		res = expandStructBuiltin("updateIn", []ast.Expr{
			ast.NewIdent("s"), &ast.BasicLit{Kind: token.STRING, Value: `"A"`}, ast.NewIdent("f"),
		}, nil, token.NoPos)
		assert.Nil(t, res)

		// unsupported builtin
		res = expandStructBuiltin("delete", []ast.Expr{ast.NewIdent("s")}, nil, token.NoPos)
		assert.Nil(t, res)

		// get with only receiver (no field arg)
		res = expandStructBuiltin(builtinGet, []ast.Expr{ast.NewIdent("s")}, nil, token.NoPos)
		assert.Nil(t, res)

		// update with only receiver + field (missing fn)
		res = expandStructBuiltin(builtinUpdate, []ast.Expr{
			ast.NewIdent("s"), &ast.BasicLit{Kind: token.STRING, Value: `"F"`},
		}, ast.NewIdent("T"), token.NoPos)
		assert.Nil(t, res)

		// updateIn with only receiver (no keys or fn)
		res = expandStructBuiltin("updateIn", []ast.Expr{ast.NewIdent("s")}, ast.NewIdent("T"), token.NoPos)
		assert.Nil(t, res)
	})

	t.Run("stringLitField invalid cases", func(t *testing.T) {
		t.Parallel()
		// Not a BasicLit
		_, ok := stringLitField(ast.NewIdent("x"))
		assert.False(t, ok)

		// BasicLit but not STRING
		lit := &ast.BasicLit{Kind: token.INT, Value: "123"}
		_, ok = stringLitField(lit)
		assert.False(t, ok)

		// Invalid string literal (missing closing quote)
		lit = &ast.BasicLit{Kind: token.STRING, Value: `"`}
		_, ok = stringLitField(lit)
		assert.False(t, ok)
	})
}
