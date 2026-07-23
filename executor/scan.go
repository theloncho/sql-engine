// scan.go implementa el operador TableScan.
// Es la hoja del árbol de operadores: itera las filas de una tabla en memoria
// una a una, sin filtrar ni transformar.
package executor

import (
	"github.com/theloncho/sql-engine/catalog"
	"github.com/theloncho/sql-engine/types"
)

// TableScan es el operador hoja del árbol Volcano.
// Itera sobre todas las filas de una tabla en memoria.
type TableScan struct {
	table  *catalog.Table
	alias  string // alias de la tabla (para calificar columnas en OutputSchema)
	cursor int    // índice de la siguiente fila a retornar
	schema OutputSchema
}

// NewTableScan crea un operador de escaneo sobre una tabla.
// alias es el nombre efectivo con el que se referencian las columnas (nombre o alias).
func NewTableScan(t *catalog.Table, alias string) *TableScan {
	if alias == "" {
		alias = t.Name
	}
	cols := make([]OutputCol, len(t.Schema.Columns))
	for i, c := range t.Schema.Columns {
		cols[i] = OutputCol{Table: alias, Name: c.Name, Kind: c.Kind}
	}
	return &TableScan{
		table:  t,
		alias:  alias,
		cursor: 0,
		schema: OutputSchema{Cols: cols},
	}
}

// Next retorna la siguiente fila de la tabla. O (nil, nil) al llegar al fin.
func (s *TableScan) Next() (Row, error) {
	if s.cursor >= len(s.table.Rows) {
		return nil, nil // EOF
	}
	row := s.table.Rows[s.cursor]
	s.cursor++
	// Retornamos una copia para que operadores superiores no modifiquen la tabla.
	result := make(Row, len(row))
	copy(result, row)
	return result, nil
}

// Close reinicia el cursor (permite reutilizar el operador, útil en NestedLoopJoin).
func (s *TableScan) Close() error {
	s.cursor = 0
	return nil
}

// Schema retorna el esquema de salida del TableScan.
func (s *TableScan) Schema() OutputSchema { return s.schema }

// Reset reinicia el cursor al principio (para reutilización en JOINs).
func (s *TableScan) Reset() {
	s.cursor = 0
}

// --- Operador de filas dadas (MaterializedScan) ---
// Útil para tests y para el operador Distinct.

// SliceScan itera sobre una slice de filas ya materializadas.
type SliceScan struct {
	rows   []Row
	schema OutputSchema
	cursor int
}

// NewSliceScan crea un operador que itera sobre filas ya materializadas.
func NewSliceScan(rows []Row, schema OutputSchema) *SliceScan {
	return &SliceScan{rows: rows, schema: schema}
}

func (s *SliceScan) Next() (Row, error) {
	if s.cursor >= len(s.rows) {
		return nil, nil
	}
	row := s.rows[s.cursor]
	s.cursor++
	return row, nil
}

func (s *SliceScan) Close() error {
	s.cursor = 0
	return nil
}

func (s *SliceScan) Schema() OutputSchema { return s.schema }

// rowKey genera una clave string para una fila (para deduplicación en Distinct).
func rowKey(row Row) string {
	parts := make([]string, len(row))
	for i, v := range row {
		parts[i] = v.String()
		if v.Kind == types.KindNull {
			parts[i] = "\x00NULL"
		}
	}
	result := ""
	for _, p := range parts {
		result += "\x01" + p
	}
	return result
}
