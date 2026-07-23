// filter.go implementa el operador Filter (WHERE).
// Consume filas del operador hijo y solo pasa las que satisfacen el predicado.
// Es un operador pipeline puro: no materializa ninguna fila intermedia.
package executor

import "github.com/theloncho/sql-engine/parser"

// Filter evalúa un predicado booleano sobre cada fila del hijo.
// Las filas para las que el predicado es FALSE o NULL (UNKNOWN) son descartadas.
type Filter struct {
	child     Operator
	predicate parser.Expr
}

// NewFilter crea un operador Filter.
// predicate es el AST de la condición WHERE.
func NewFilter(child Operator, predicate parser.Expr) *Filter {
	return &Filter{child: child, predicate: predicate}
}

// Next itera el hijo hasta encontrar una fila que pase el predicado, o EOF.
func (f *Filter) Next() (Row, error) {
	for {
		row, err := f.child.Next()
		if err != nil {
			return nil, err
		}
		if row == nil {
			return nil, nil // EOF: no hay más filas
		}
		ctx := &EvalContext{Row: row, Schema: f.child.Schema()}
		pass, err := EvalBool(f.predicate, ctx)
		if err != nil {
			return nil, err
		}
		if pass {
			return row, nil
		}
		// Fila descartada: continúa con la siguiente.
	}
}

// Close propaga el cierre al hijo.
func (f *Filter) Close() error { return f.child.Close() }

// Schema retorna el mismo esquema que el hijo (Filter no modifica columnas).
func (f *Filter) Schema() OutputSchema { return f.child.Schema() }

func (f *Filter) Children() []Operator { return []Operator{f.child} }
