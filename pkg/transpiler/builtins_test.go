package transpiler

import (
	"go/ast"
	"go/token"
	"testing"
)

func TestBuiltinEdgeCases(t *testing.T) {
	t.Run("expandListBuiltin unsupported", func(t *testing.T) {
		res := expandListBuiltin("delete", []ast.Expr{ast.NewIdent("l")}, token.NoPos)
		if res != nil {
			t.Errorf("Expected nil for delete on list, got %T", res)
		}
	})

	t.Run("expandArrayBuiltin unsupported", func(t *testing.T) {
		res := expandArrayBuiltin("delete", []ast.Expr{ast.NewIdent("a")}, nil,
			token.NoPos, func(e ast.Expr) ast.Expr { return e })
		if res != nil {
			t.Errorf("Expected nil for delete on array, got %T", res)
		}

		// update with nil typeExpr
		res = expandArrayBuiltin("update", []ast.Expr{
			ast.NewIdent("a"), ast.NewIdent("i"), ast.NewIdent("f"),
		}, nil, token.NoPos, func(e ast.Expr) ast.Expr { return e })
		if res != nil {
			t.Errorf("Expected nil for update on array with nil typeExpr, got %T", res)
		}
	})

	t.Run("expandStructBuiltin edge cases", func(t *testing.T) {
		// get with non-string lit
		res := expandStructBuiltin("get", []ast.Expr{ast.NewIdent("s"), ast.NewIdent("notalit")}, nil, token.NoPos)
		if res != nil {
			t.Errorf("Expected nil for get with non-string lit, got %T", res)
		}

		// getIn with non-string lit
		res = expandStructBuiltin("getIn", []ast.Expr{
			ast.NewIdent("s"), &ast.BasicLit{Kind: token.STRING, Value: `"A"`}, ast.NewIdent("notalit"),
		}, nil, token.NoPos)
		if res != nil {
			t.Errorf("Expected nil for getIn with non-string lit, got %T", res)
		}

		// update with non-string lit
		res = expandStructBuiltin("update", []ast.Expr{
			ast.NewIdent("s"), ast.NewIdent("notalit"), ast.NewIdent("f"),
		}, ast.NewIdent("T"), token.NoPos)
		if res != nil {
			t.Errorf("Expected nil for update with non-string lit, got %T", res)
		}

		// updateIn with non-string lit
		res = expandStructBuiltin("updateIn", []ast.Expr{
			ast.NewIdent("s"), &ast.BasicLit{Kind: token.STRING, Value: `"A"`}, ast.NewIdent("notalit"), ast.NewIdent("f"),
		}, ast.NewIdent("T"), token.NoPos)
		if res != nil {
			t.Errorf("Expected nil for updateIn with non-string lit, got %T", res)
		}

		// update with nil typeExpr
		res = expandStructBuiltin("update", []ast.Expr{
			ast.NewIdent("s"), &ast.BasicLit{Kind: token.STRING, Value: `"A"`}, ast.NewIdent("f"),
		}, nil, token.NoPos)
		if res != nil {
			t.Errorf("Expected nil for update with nil typeExpr, got %T", res)
		}

		// updateIn with nil typeExpr
		res = expandStructBuiltin("updateIn", []ast.Expr{
			ast.NewIdent("s"), &ast.BasicLit{Kind: token.STRING, Value: `"A"`}, ast.NewIdent("f"),
		}, nil, token.NoPos)
		if res != nil {
			t.Errorf("Expected nil for updateIn with nil typeExpr, got %T", res)
		}

		// unsupported builtin
		res = expandStructBuiltin("delete", []ast.Expr{ast.NewIdent("s")}, nil, token.NoPos)
		if res != nil {
			t.Errorf("Expected nil for unsupported builtin on struct, got %T", res)
		}
	})

	t.Run("stringLitField invalid cases", func(t *testing.T) {
		// Not a BasicLit
		_, ok := stringLitField(ast.NewIdent("x"))
		if ok {
			t.Errorf("Expected false for non-BasicLit")
		}

		// BasicLit but not STRING
		lit := &ast.BasicLit{Kind: token.INT, Value: "123"}
		_, ok = stringLitField(lit)
		if ok {
			t.Errorf("Expected false for non-STRING BasicLit")
		}

		// Invalid string literal (missing closing quote)
		lit = &ast.BasicLit{Kind: token.STRING, Value: `"`}
		_, ok = stringLitField(lit)
		if ok {
			t.Errorf("Expected false for invalid string literal")
		}
	})
}
