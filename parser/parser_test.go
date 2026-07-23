package parser

import (
	"testing"
)

func mustParse(t *testing.T, sql string) *SelectStmt {
	t.Helper()
	stmt, err := Parse(sql)
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", sql, err)
	}
	return stmt
}

func expectParseError(t *testing.T, sql string) {
	t.Helper()
	_, err := Parse(sql)
	if err == nil {
		t.Errorf("Parse(%q): expected error, got nil", sql)
	}
}

// --- Tests de consultas válidas ---

func TestParser_SimpleSelect(t *testing.T) {
	stmt := mustParse(t, "SELECT * FROM employees")
	if len(stmt.Cols) != 1 || !stmt.Cols[0].IsWildcard {
		t.Error("expected wildcard column *")
	}
	if stmt.From.Name != "employees" {
		t.Errorf("from = %q, want %q", stmt.From.Name, "employees")
	}
}

func TestParser_SelectCols(t *testing.T) {
	stmt := mustParse(t, "SELECT id, name, salary FROM employees")
	if len(stmt.Cols) != 3 {
		t.Fatalf("cols count = %d, want 3", len(stmt.Cols))
	}
	names := []string{"id", "name", "salary"}
	for i, want := range names {
		ident, ok := stmt.Cols[i].Expr.(*Identifier)
		if !ok {
			t.Errorf("col[%d] not an Identifier", i)
			continue
		}
		if ident.Column != want {
			t.Errorf("col[%d] = %q, want %q", i, ident.Column, want)
		}
	}
}

func TestParser_Where(t *testing.T) {
	stmt := mustParse(t, "SELECT * FROM t WHERE salary > 50000")
	if stmt.Where == nil {
		t.Fatal("expected WHERE clause")
	}
	bin, ok := stmt.Where.(*BinaryExpr)
	if !ok {
		t.Fatalf("WHERE not a BinaryExpr, got %T", stmt.Where)
	}
	if bin.Op != ">" {
		t.Errorf("op = %q, want >", bin.Op)
	}
}

func TestParser_WhereAndOr(t *testing.T) {
	stmt := mustParse(t, "SELECT * FROM t WHERE a = 1 AND b = 2 OR c = 3")
	if stmt.Where == nil {
		t.Fatal("expected WHERE clause")
	}
	// Top-level debería ser OR (menor precedencia).
	bin, ok := stmt.Where.(*BinaryExpr)
	if !ok || bin.Op != "OR" {
		t.Errorf("expected top-level OR, got %T %v", stmt.Where, stmt.Where)
	}
}

func TestParser_OrderBy(t *testing.T) {
	stmt := mustParse(t, "SELECT * FROM t ORDER BY salary DESC, name ASC")
	if len(stmt.OrderBy) != 2 {
		t.Fatalf("orderby count = %d, want 2", len(stmt.OrderBy))
	}
	if stmt.OrderBy[0].Asc {
		t.Error("first ORDER BY should be DESC")
	}
	if !stmt.OrderBy[1].Asc {
		t.Error("second ORDER BY should be ASC")
	}
}

func TestParser_Limit(t *testing.T) {
	stmt := mustParse(t, "SELECT * FROM t LIMIT 10")
	if stmt.Limit == nil || *stmt.Limit != 10 {
		t.Errorf("limit = %v, want 10", stmt.Limit)
	}
}

func TestParser_GroupBy(t *testing.T) {
	stmt := mustParse(t, "SELECT dept, COUNT(*) FROM employees GROUP BY dept")
	if len(stmt.GroupBy) != 1 {
		t.Fatalf("groupby count = %d, want 1", len(stmt.GroupBy))
	}
	if len(stmt.Cols) != 2 {
		t.Fatalf("cols count = %d, want 2", len(stmt.Cols))
	}
	// Segunda columna debe ser COUNT(*)
	agg, ok := stmt.Cols[1].Expr.(*AggFunc)
	if !ok || !agg.IsStar {
		t.Error("expected COUNT(*)")
	}
}

func TestParser_InnerJoin(t *testing.T) {
	stmt := mustParse(t, "SELECT * FROM e INNER JOIN d ON e.dept_id = d.id")
	if len(stmt.Joins) != 1 {
		t.Fatalf("joins count = %d, want 1", len(stmt.Joins))
	}
	if stmt.Joins[0].Table.Name != "d" {
		t.Errorf("join table = %q, want d", stmt.Joins[0].Table.Name)
	}
}

func TestParser_Distinct(t *testing.T) {
	stmt := mustParse(t, "SELECT DISTINCT dept FROM employees")
	if !stmt.Distinct {
		t.Error("expected DISTINCT flag")
	}
}

func TestParser_TableAlias(t *testing.T) {
	stmt := mustParse(t, "SELECT e.name FROM employees AS e")
	if stmt.From.Alias != "e" {
		t.Errorf("alias = %q, want e", stmt.From.Alias)
	}
}

func TestParser_NullLiteral(t *testing.T) {
	stmt := mustParse(t, "SELECT * FROM t WHERE x = NULL")
	bin := stmt.Where.(*BinaryExpr)
	if _, ok := bin.Right.(*NullLiteral); !ok {
		t.Error("expected NullLiteral on right side")
	}
}

func TestParser_Arithmetic(t *testing.T) {
	stmt := mustParse(t, "SELECT salary * 1.1 AS bonus FROM t")
	col := stmt.Cols[0]
	if col.Alias != "bonus" {
		t.Errorf("alias = %q, want bonus", col.Alias)
	}
	bin, ok := col.Expr.(*BinaryExpr)
	if !ok || bin.Op != "*" {
		t.Errorf("expected multiplication, got %T %v", col.Expr, col.Expr)
	}
}

// --- Tests de errores de sintaxis ---

func TestParser_Errors(t *testing.T) {
	badQueries := []string{
		"",
		"SELECT",
		"SELECT * FROM",
		"SELECT * FROM t WHERE",
		"SELECT * FROM t ORDER",
		"SELECT * FROM t ORDER BY",
		"SELECT * FROM t LIMIT abc",
		"SELECT * FROM t INNER JOIN",
		"SELECT * FROM t INNER JOIN d",
		"SELECT * FROM t INNER JOIN d ON",
		"HELLO WORLD",
		"SELECT * FROM t WHERE (a = 1",
		"SELECT * FROM t EXTRA_TOKEN",
	}
	for _, q := range badQueries {
		expectParseError(t, q)
	}
}

func TestParser_Semicolon(t *testing.T) {
	// El punto y coma final es opcional.
	stmt := mustParse(t, "SELECT * FROM t;")
	if stmt.From.Name != "t" {
		t.Error("expected table t")
	}
}

func TestParser_LowercaseKeywords(t *testing.T) {
	// Verificar que el parser acepte palabras clave en minúsculas.
	stmt := mustParse(t, "select id, name from users where age > 21 limit 10")
	if stmt.From.Name != "users" {
		t.Errorf("from = %q, want users", stmt.From.Name)
	}
	if stmt.Limit == nil || *stmt.Limit != 10 {
		t.Errorf("limit = %v, want 10", stmt.Limit)
	}
}
