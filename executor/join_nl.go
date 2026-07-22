// join_nl.go implementa el operador NestedLoopJoin (INNER JOIN).
// Complejidad: O(n × m) donde n = filas de la tabla externa, m = filas de la interna.
// Es el join más simple y correcto; sirve como referencia para verificar HashJoin.
//
// Funcionamiento:
//   - Para cada fila de la tabla externa (outer), reinicia la tabla interna (inner)
//     y recorre todas sus filas buscando las que cumplen la condición ON.
//   - La condición ON se evalúa sobre la fila concatenada (outer || inner).
package executor

import (
	"github.com/theloncho/sql-engine/parser"
)

// NestedLoopJoin implementa INNER JOIN por fuerza bruta.
type NestedLoopJoin struct {
	outer    Operator
	inner    ResettableOperator // la tabla interna debe poderse reiniciar
	cond     parser.Expr
	schema   OutputSchema
	outerRow Row  // fila actual de outer
	done     bool // outer agotado
}

// ResettableOperator es un Operator que puede reiniciarse al inicio.
// TableScan lo implementa con Reset().
type ResettableOperator interface {
	Operator
	Reset()
}

// NewNestedLoopJoin crea un NestedLoopJoin.
func NewNestedLoopJoin(outer Operator, inner ResettableOperator, cond parser.Expr) *NestedLoopJoin {
	// El esquema de salida es la concatenación de outer + inner.
	oc := outer.Schema().Cols
	ic := inner.Schema().Cols
	merged := make([]OutputCol, len(oc)+len(ic))
	copy(merged, oc)
	copy(merged[len(oc):], ic)

	return &NestedLoopJoin{
		outer:  outer,
		inner:  inner,
		cond:   cond,
		schema: OutputSchema{Cols: merged},
	}
}

// Next itera el producto cartesiano y retorna la siguiente fila que satisface la condición ON.
func (j *NestedLoopJoin) Next() (Row, error) {
	for {
		// Si no tenemos fila outer activa, obtener la siguiente.
		if j.outerRow == nil {
			if j.done {
				return nil, nil // EOF: outer agotado
			}
			row, err := j.outer.Next()
			if err != nil {
				return nil, err
			}
			if row == nil {
				j.done = true
				return nil, nil
			}
			j.outerRow = row
			j.inner.Reset() // reiniciar la tabla interna para esta fila outer
		}

		// Obtener la siguiente fila de inner.
		innerRow, err := j.inner.Next()
		if err != nil {
			return nil, err
		}
		if innerRow == nil {
			// Inner agotado: avanzar a la siguiente fila outer.
			j.outerRow = nil
			continue
		}

		// Construir la fila combinada y evaluar la condición ON.
		combined := concatRows(j.outerRow, innerRow)
		ctx := &EvalContext{Row: combined, Schema: j.schema}
		pass, err := EvalBool(j.cond, ctx)
		if err != nil {
			return nil, err
		}
		if pass {
			return combined, nil
		}
	}
}

// Close libera recursos de ambos hijos.
func (j *NestedLoopJoin) Close() error {
	j.outerRow = nil
	j.done = false
	if err := j.outer.Close(); err != nil {
		return err
	}
	return j.inner.Close()
}

// Schema retorna el esquema combinado outer+inner.
func (j *NestedLoopJoin) Schema() OutputSchema { return j.schema }

// concatRows concatena dos filas en una nueva.
func concatRows(a, b Row) Row {
	result := make(Row, len(a)+len(b))
	copy(result, a)
	copy(result[len(a):], b)
	return result
}
