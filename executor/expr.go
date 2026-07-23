// expr.go implementa la evaluación de expresiones SQL sobre una fila.
// Las expresiones se resuelven en runtime: dado un nodo del AST y una fila actual,
// se retorna el Value resultante.
//
// Manejo de NULL: sigue la lógica de tres valores de SQL (TRUE, FALSE, UNKNOWN).
// NULL comparado con cualquier cosa produce UNKNOWN (representado como NULL en el Value).
package executor

import (
	"fmt"
	"strings"

	"github.com/theloncho/sql-engine/parser"
	"github.com/theloncho/sql-engine/types"
)

// EvalContext proporciona el contexto de evaluación:
// la fila actual y el esquema del operador desde el que se evalúa.
type EvalContext struct {
	Row    Row
	Schema OutputSchema
}

// Eval evalúa una expresión del AST sobre el contexto dado.
// Retorna el Value resultante o un error semántico (columna no encontrada, tipo incorrecto).
func Eval(expr parser.Expr, ctx *EvalContext) (types.Value, error) {
	switch e := expr.(type) {

	case *parser.IntLiteral:
		return types.IntVal(e.Value), nil

	case *parser.FloatLiteral:
		return types.FloatVal(e.Value), nil

	case *parser.StringLiteral:
		return types.StringVal(e.Value), nil

	case *parser.BoolLiteral:
		return types.BoolVal(e.Value), nil

	case *parser.NullLiteral:
		return types.Null(), nil

	case *parser.Identifier:
		return evalIdentifier(e, ctx)

	case *parser.UnaryExpr:
		return evalUnary(e, ctx)

	case *parser.BinaryExpr:
		return evalBinary(e, ctx)

	case *parser.AggFunc:
		// Las funciones de agregación no se evalúan en este nivel —
		// el operador Aggregate las procesa directamente. Si llegamos aquí
		// es porque la consulta tiene una función de agregación fuera de un contexto
		// de agregación, lo cual es un error semántico.
		return types.Null(), fmt.Errorf("aggregate function %s not allowed here", e.Name)
	}

	return types.Null(), fmt.Errorf("unsupported expression type %T", expr)
}

// evalIdentifier resuelve una referencia de columna (tabla.col o col) en el contexto.
func evalIdentifier(e *parser.Identifier, ctx *EvalContext) (types.Value, error) {
	idx := ctx.Schema.IndexOf(e.Table, e.Column)
	if idx == -1 {
		if e.Table != "" {
			return types.Null(), fmt.Errorf("column %q.%q not found", e.Table, e.Column)
		}
		return types.Null(), fmt.Errorf("column %q not found", e.Column)
	}
	if idx >= len(ctx.Row) {
		return types.Null(), fmt.Errorf("row index out of bounds: idx=%d, len=%d", idx, len(ctx.Row))
	}
	return ctx.Row[idx], nil
}

// evalUnary evalúa operaciones unarias: negación numérica (-) y NOT lógico.
func evalUnary(e *parser.UnaryExpr, ctx *EvalContext) (types.Value, error) {
	val, err := Eval(e.Expr, ctx)
	if err != nil {
		return types.Null(), err
	}
	switch e.Op {
	case "-":
		if val.IsNull() {
			return types.Null(), nil
		}
		switch val.Kind {
		case types.KindInt:
			return types.IntVal(-val.IVal), nil
		case types.KindFloat:
			return types.FloatVal(-val.FVal), nil
		}
		return types.Null(), fmt.Errorf("unary minus not supported on type %s", val.TypeName())
	case "NOT":
		if val.IsNull() {
			return types.Null(), nil // NOT NULL = NULL (UNKNOWN)
		}
		if val.Kind != types.KindBool {
			return types.Null(), fmt.Errorf("NOT requires boolean, got %s", val.TypeName())
		}
		return types.BoolVal(!val.BVal), nil
	}
	return types.Null(), fmt.Errorf("unknown unary operator %q", e.Op)
}

// evalBinary evalúa operaciones binarias: comparaciones, lógica, aritmética.
func evalBinary(e *parser.BinaryExpr, ctx *EvalContext) (types.Value, error) {
	switch strings.ToUpper(e.Op) {
	case "AND":
		return evalAnd(e, ctx)
	case "OR":
		return evalOr(e, ctx)
	}

	left, err := Eval(e.Left, ctx)
	if err != nil {
		return types.Null(), err
	}
	right, err := Eval(e.Right, ctx)
	if err != nil {
		return types.Null(), err
	}

	switch e.Op {
	case "+":
		return types.Add(left, right)
	case "-":
		return types.Sub(left, right)
	case "*":
		return types.Mul(left, right)
	case "/":
		return types.Div(left, right)
	}

	return evalCmp(e.Op, left, right)
}

// evalAnd implementa la lógica de tres valores para AND:
//   TRUE  AND TRUE  = TRUE
//   TRUE  AND FALSE = FALSE
//   TRUE  AND NULL  = NULL (UNKNOWN)
//   FALSE AND *     = FALSE
//   NULL  AND FALSE = FALSE
//   NULL  AND *     = NULL
func evalAnd(e *parser.BinaryExpr, ctx *EvalContext) (types.Value, error) {
	left, err := Eval(e.Left, ctx)
	if err != nil {
		return types.Null(), err
	}
	// Cortocircuito: si left es FALSE, retornar FALSE sin evaluar right.
	if !left.IsNull() && left.Kind == types.KindBool && !left.BVal {
		return types.BoolVal(false), nil
	}
	right, err := Eval(e.Right, ctx)
	if err != nil {
		return types.Null(), err
	}
	// Si right es FALSE, retornar FALSE.
	if !right.IsNull() && right.Kind == types.KindBool && !right.BVal {
		return types.BoolVal(false), nil
	}
	// Si alguno es NULL, retornar NULL (UNKNOWN).
	if left.IsNull() || right.IsNull() {
		return types.Null(), nil
	}
	if left.Kind != types.KindBool || right.Kind != types.KindBool {
		return types.Null(), fmt.Errorf("AND requires boolean operands")
	}
	return types.BoolVal(left.BVal && right.BVal), nil
}

// evalOr implementa la lógica de tres valores para OR:
//   FALSE OR FALSE = FALSE
//   FALSE OR TRUE  = TRUE
//   FALSE OR NULL  = NULL (UNKNOWN)
//   TRUE  OR *     = TRUE
//   NULL  OR TRUE  = TRUE
//   NULL  OR *     = NULL
func evalOr(e *parser.BinaryExpr, ctx *EvalContext) (types.Value, error) {
	left, err := Eval(e.Left, ctx)
	if err != nil {
		return types.Null(), err
	}
	// Cortocircuito: si left es TRUE, retornar TRUE.
	if !left.IsNull() && left.Kind == types.KindBool && left.BVal {
		return types.BoolVal(true), nil
	}
	right, err := Eval(e.Right, ctx)
	if err != nil {
		return types.Null(), err
	}
	// Si right es TRUE, retornar TRUE.
	if !right.IsNull() && right.Kind == types.KindBool && right.BVal {
		return types.BoolVal(true), nil
	}
	// Si alguno es NULL, retornar NULL (UNKNOWN).
	if left.IsNull() || right.IsNull() {
		return types.Null(), nil
	}
	if left.Kind != types.KindBool || right.Kind != types.KindBool {
		return types.Null(), fmt.Errorf("OR requires boolean operands")
	}
	return types.BoolVal(left.BVal || right.BVal), nil
}

// evalCmp evalúa operadores de comparación: =, <>, <, >, <=, >=.
// NULL en cualquier lado produce NULL (UNKNOWN).
func evalCmp(op string, left, right types.Value) (types.Value, error) {
	if left.IsNull() || right.IsNull() {
		return types.Null(), nil // NULL comparado con cualquier cosa = UNKNOWN
	}
	cmp, err := left.Cmp(right)
	if err != nil {
		return types.Null(), fmt.Errorf("comparison error: %w", err)
	}
	var result bool
	switch op {
	case "=":
		result = cmp == 0
	case "<>":
		result = cmp != 0
	case "<":
		result = cmp < 0
	case ">":
		result = cmp > 0
	case "<=":
		result = cmp <= 0
	case ">=":
		result = cmp >= 0
	default:
		return types.Null(), fmt.Errorf("unknown comparison operator %q", op)
	}
	return types.BoolVal(result), nil
}

// EvalBool evalúa una expresión esperando un resultado booleano.
// Retorna false para NULL (UNKNOWN → no pasa el filtro, semántica SQL).
func EvalBool(expr parser.Expr, ctx *EvalContext) (bool, error) {
	v, err := Eval(expr, ctx)
	if err != nil {
		return false, err
	}
	if v.IsNull() {
		return false, nil // UNKNOWN → false en contexto de filtro
	}
	if v.Kind != types.KindBool {
		return false, fmt.Errorf("expression must be boolean, got %s", v.TypeName())
	}
	return v.BVal, nil
}
