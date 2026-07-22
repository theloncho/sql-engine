// join_hash.go implementa el operador HashJoin (INNER JOIN más eficiente).
// Complejidad: O(n + m) promedio, donde n = tabla de construcción (build), m = sonda (probe).
//
// Algoritmo:
//  1. Build phase: materializar la tabla interna en un mapa hash indexado por la clave de join.
//  2. Probe phase: para cada fila de la tabla externa, buscar en el mapa las filas coincidentes.
//
// Restricción: HashJoin solo funciona para condiciones de igualdad de la forma
// col_externa = col_interna (o viceversa). El planner es responsable de detectar
// si la condición ON es apta para HashJoin; si no lo es, usa NestedLoopJoin.
package executor

import (
	"fmt"

	"github.com/theloncho/sql-engine/parser"
	"github.com/theloncho/sql-engine/types"
)

// HashJoin implementa INNER JOIN con tabla hash.
type HashJoin struct {
	outer     Operator
	inner     Operator
	outerKey  parser.Expr // clave de join del lado outer
	innerKey  parser.Expr // clave de join del lado inner
	schema    OutputSchema

	// Estado de la fase de probe.
	hashTable map[string][]Row // hash table construida sobre inner
	matches   []Row            // filas inner que hacen match con outerRow actual
	matchIdx  int
	outerRow  Row
	built     bool
}

// NewHashJoin crea un HashJoin.
// outerKeyExpr e innerKeyExpr son las expresiones de clave del join (típicamente columnas).
// Ejemplo: "e.dept_id = d.id" → outerKeyExpr=e.dept_id, innerKeyExpr=d.id
func NewHashJoin(outer, inner Operator, outerKeyExpr, innerKeyExpr parser.Expr) *HashJoin {
	oc := outer.Schema().Cols
	ic := inner.Schema().Cols
	merged := make([]OutputCol, len(oc)+len(ic))
	copy(merged, oc)
	copy(merged[len(oc):], ic)

	return &HashJoin{
		outer:    outer,
		inner:    inner,
		outerKey: outerKeyExpr,
		innerKey: innerKeyExpr,
		schema:   OutputSchema{Cols: merged},
	}
}

// Next implementa la fase de probe. La fase de build ocurre en la primera llamada.
func (h *HashJoin) Next() (Row, error) {
	// Build phase: construir la tabla hash sobre inner.
	if !h.built {
		if err := h.build(); err != nil {
			return nil, err
		}
		h.built = true
	}

	for {
		// Si hay matches pendientes del outerRow actual, emitirlos.
		if h.outerRow != nil && h.matchIdx < len(h.matches) {
			innerRow := h.matches[h.matchIdx]
			h.matchIdx++
			return concatRows(h.outerRow, innerRow), nil
		}

		// Obtener siguiente fila outer (probe).
		outerRow, err := h.outer.Next()
		if err != nil {
			return nil, err
		}
		if outerRow == nil {
			return nil, nil // EOF
		}
		h.outerRow = outerRow

		// Calcular clave de la fila outer.
		outerCtx := &EvalContext{Row: outerRow, Schema: h.outer.Schema()}
		outerVal, err := Eval(h.outerKey, outerCtx)
		if err != nil {
			return nil, fmt.Errorf("hashJoin outer key eval: %w", err)
		}
		if outerVal.IsNull() {
			// NULL no hace match con nada en un INNER JOIN.
			h.matches = nil
			h.matchIdx = 0
			continue
		}

		// Buscar en la tabla hash.
		key := hashKey(outerVal)
		h.matches = h.hashTable[key]
		h.matchIdx = 0
	}
}

// build materializa la tabla inner en un mapa hash.
func (h *HashJoin) build() error {
	h.hashTable = make(map[string][]Row)
	innerSchema := h.inner.Schema()

	for {
		row, err := h.inner.Next()
		if err != nil {
			return err
		}
		if row == nil {
			break
		}
		ctx := &EvalContext{Row: row, Schema: innerSchema}
		keyVal, err := Eval(h.innerKey, ctx)
		if err != nil {
			return fmt.Errorf("hashJoin inner key eval: %w", err)
		}
		if keyVal.IsNull() {
			continue // NULL no participa en el hash
		}
		key := hashKey(keyVal)
		h.hashTable[key] = append(h.hashTable[key], row)
	}
	return nil
}

// hashKey genera la clave string para el mapa hash.
// Incluye el tipo para evitar colisiones entre int(1) y string("1").
func hashKey(v types.Value) string {
	return fmt.Sprintf("%d:%s", v.Kind, v.String())
}

// Close libera recursos.
func (h *HashJoin) Close() error {
	h.hashTable = nil
	h.matches = nil
	h.outerRow = nil
	h.built = false
	if err := h.outer.Close(); err != nil {
		return err
	}
	return h.inner.Close()
}

// Schema retorna el esquema combinado outer+inner.
func (h *HashJoin) Schema() OutputSchema { return h.schema }
