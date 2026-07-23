// Package parser define los nodos del Árbol de Sintaxis Abstracta (AST)
// que el parser produce a partir de los tokens del lexer.
// Cada nodo representa una construcción SQL: consulta completa, expresión, columna, etc.
package parser

import (
	"fmt"
	"strings"
)

// Node es la interfaz raíz de todos los nodos del AST.
// El método sql() retorna una representación textual del nodo (útil para debug y EXPLAIN).
type Node interface {
	sql() string
}

// --- Nodo raíz de consulta ---

// SelectStmt representa una consulta SELECT completa.
type SelectStmt struct {
	Distinct bool         // SELECT DISTINCT
	Cols     []SelectCol  // columnas seleccionadas
	From     *TableRef    // tabla principal
	Joins    []JoinClause // JOINs
	Where    Expr         // condición WHERE (nil si ausente)
	GroupBy  []Expr       // columnas de GROUP BY
	OrderBy  []OrderItem  // criterios de ORDER BY
	Limit    *int64       // valor de LIMIT (nil si ausente)
}

func (s *SelectStmt) sql() string {
	var sb strings.Builder
	sb.WriteString("SELECT ")
	if s.Distinct {
		sb.WriteString("DISTINCT ")
	}
	cols := make([]string, len(s.Cols))
	for i, c := range s.Cols {
		cols[i] = c.sql()
	}
	sb.WriteString(strings.Join(cols, ", "))
	sb.WriteString(" FROM ")
	sb.WriteString(s.From.sql())
	for _, j := range s.Joins {
		sb.WriteString(" ")
		sb.WriteString(j.sql())
	}
	if s.Where != nil {
		sb.WriteString(" WHERE ")
		sb.WriteString(s.Where.sql())
	}
	if len(s.GroupBy) > 0 {
		exprs := make([]string, len(s.GroupBy))
		for i, e := range s.GroupBy {
			exprs[i] = e.sql()
		}
		sb.WriteString(" GROUP BY ")
		sb.WriteString(strings.Join(exprs, ", "))
	}
	if len(s.OrderBy) > 0 {
		items := make([]string, len(s.OrderBy))
		for i, o := range s.OrderBy {
			items[i] = o.sql()
		}
		sb.WriteString(" ORDER BY ")
		sb.WriteString(strings.Join(items, ", "))
	}
	if s.Limit != nil {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", *s.Limit))
	}
	return sb.String()
}

// --- Referencias de tabla ---

// TableRef referencia a una tabla, con alias opcional.
type TableRef struct {
	Name  string // nombre de la tabla en el catálogo
	Alias string // alias opcional (vacío si no hay)
}

func (t *TableRef) sql() string {
	if t.Alias != "" {
		return t.Name + " AS " + t.Alias
	}
	return t.Name
}

// EffectiveName retorna el alias si existe, o el nombre de tabla.
func (t *TableRef) EffectiveName() string {
	if t.Alias != "" {
		return t.Alias
	}
	return t.Name
}

// JoinClause representa un INNER JOIN ... ON ...
type JoinClause struct {
	Table *TableRef
	On    Expr
}

func (j *JoinClause) sql() string {
	return "INNER JOIN " + j.Table.sql() + " ON " + j.On.sql()
}

// --- Columnas de SELECT ---

// SelectCol es una columna del SELECT: una expresión con alias opcional.
// Si IsWildcard=true, la expresión es NULL y el alias se ignora (representa `*`).
type SelectCol struct {
	Expr       Expr
	Alias      string
	IsWildcard bool
}

func (c *SelectCol) sql() string {
	if c.IsWildcard {
		return "*"
	}
	if c.Alias != "" {
		return c.Expr.sql() + " AS " + c.Alias
	}
	return c.Expr.sql()
}

// --- ORDER BY ---

// OrderItem es un elemento del ORDER BY: expresión + dirección.
type OrderItem struct {
	Expr Expr
	Asc  bool // true=ASC (default), false=DESC
}

func (o *OrderItem) sql() string {
	dir := "ASC"
	if !o.Asc {
		dir = "DESC"
	}
	return o.Expr.sql() + " " + dir
}

// --- Expresiones (Expr) ---

// Expr es la interfaz de todas las expresiones SQL.
type Expr interface {
	Node
	exprNode()
}

// BinaryExpr representa una operación binaria: left op right.
// Op puede ser: =, <>, <, >, <=, >=, AND, OR, +, -, *, /
type BinaryExpr struct {
	Left  Expr
	Op    string
	Right Expr
}

func (b *BinaryExpr) exprNode() {}
func (b *BinaryExpr) sql() string {
	return "(" + b.Left.sql() + " " + b.Op + " " + b.Right.sql() + ")"
}

// UnaryExpr representa una operación unaria: op expr.
// Op puede ser: -, NOT
type UnaryExpr struct {
	Op   string
	Expr Expr
}

func (u *UnaryExpr) exprNode() {}
func (u *UnaryExpr) sql() string {
	return u.Op + " " + u.Expr.sql()
}

// Identifier es una referencia a una columna, opcionalmente calificada: tabla.columna.
type Identifier struct {
	Table  string // vacío si no está calificado
	Column string
}

func (i *Identifier) exprNode() {}
func (i *Identifier) sql() string {
	if i.Table != "" {
		return i.Table + "." + i.Column
	}
	return i.Column
}

// IntLiteral es un literal entero.
type IntLiteral struct {
	Value int64
}

func (il *IntLiteral) exprNode()   {}
func (il *IntLiteral) sql() string { return fmt.Sprintf("%d", il.Value) }

// FloatLiteral es un literal flotante.
type FloatLiteral struct {
	Value float64
}

func (fl *FloatLiteral) exprNode()   {}
func (fl *FloatLiteral) sql() string { return fmt.Sprintf("%g", fl.Value) }

// StringLiteral es un literal de cadena.
type StringLiteral struct {
	Value string
}

func (sl *StringLiteral) exprNode()   {}
func (sl *StringLiteral) sql() string { return "'" + strings.ReplaceAll(sl.Value, "'", "''") + "'" }

// BoolLiteral es un literal booleano (TRUE/FALSE).
type BoolLiteral struct {
	Value bool
}

func (bl *BoolLiteral) exprNode() {}
func (bl *BoolLiteral) sql() string {
	if bl.Value {
		return "TRUE"
	}
	return "FALSE"
}

// NullLiteral es el literal NULL.
type NullLiteral struct{}

func (nl *NullLiteral) exprNode()   {}
func (nl *NullLiteral) sql() string { return "NULL" }

// AggFunc es una llamada a función de agregación: COUNT(*), SUM(col), etc.
type AggFunc struct {
	Name   string // COUNT, SUM, AVG, MIN, MAX
	Arg    Expr   // nil para COUNT(*)
	IsStar bool   // true para COUNT(*)
}

func (a *AggFunc) exprNode() {}
func (a *AggFunc) sql() string {
	if a.IsStar {
		return a.Name + "(*)"
	}
	return a.Name + "(" + a.Arg.sql() + ")"
}
