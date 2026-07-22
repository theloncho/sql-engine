// Package catalog gestiona el catálogo de tablas del motor SQL en memoria.
// El catálogo almacena esquemas y las filas de cada tabla.
package catalog

import (
	"fmt"
	"strings"

	"github.com/theloncho/sql-engine/types"
)

// Column describe una columna: nombre y tipo.
type Column struct {
	Name string
	Kind types.Kind
}

// Schema describe el esquema de una tabla: lista ordenada de columnas.
type Schema struct {
	Columns []Column
}

// IndexOf retorna el índice de la columna con el nombre dado, o -1 si no existe.
func (s *Schema) IndexOf(name string) int {
	for i, c := range s.Columns {
		if c.Name == name {
			return i
		}
	}
	return -1
}

// Table es una tabla en memoria con su esquema y sus filas.
type Table struct {
	Name   string
	Schema Schema
	Rows   [][]types.Value // cada fila es un slice de Values alineado con Schema.Columns
}

// NumCols retorna el número de columnas.
func (t *Table) NumCols() int { return len(t.Schema.Columns) }

// NumRows retorna el número de filas.
func (t *Table) NumRows() int { return len(t.Rows) }

// Catalog es el registro global de todas las tablas cargadas en memoria.
// No es concurrente: el motor es single-threaded por diseño (sin goroutines en el planificador).
type Catalog struct {
	tables map[string]*Table // nombre de tabla (case-sensitive) → tabla
}

// New crea un catálogo vacío.
func New() *Catalog {
	return &Catalog{tables: make(map[string]*Table)}
}

// Register registra una tabla en el catálogo.
// Retorna error si ya existe una tabla con ese nombre.
func (c *Catalog) Register(t *Table) error {
	if _, exists := c.tables[t.Name]; exists {
		return fmt.Errorf("table %q already exists in catalog", t.Name)
	}
	c.tables[t.Name] = t
	return nil
}

// Replace registra o reemplaza una tabla (útil para tests y carga incremental).
func (c *Catalog) Replace(t *Table) {
	c.tables[t.Name] = t
}

// Get retorna la tabla con el nombre dado, o error si no existe.
func (c *Catalog) Get(name string) (*Table, error) {
	t, ok := c.tables[name]
	if !ok {
		return nil, fmt.Errorf("table %q not found", name)
	}
	return t, nil
}

// TableNames retorna los nombres de todas las tablas registradas, en orden.
func (c *Catalog) TableNames() []string {
	names := make([]string, 0, len(c.tables))
	for n := range c.tables {
		names = append(names, n)
	}
	// Ordenar para salida determinista.
	sortStrings(names)
	return names
}

// Describe retorna una descripción textual del esquema de todas las tablas.
func (c *Catalog) Describe() string {
	var sb strings.Builder
	for _, name := range c.TableNames() {
		t := c.tables[name]
		sb.WriteString(fmt.Sprintf("Table: %s (%d rows)\n", t.Name, t.NumRows()))
		for _, col := range t.Schema.Columns {
			sb.WriteString(fmt.Sprintf("  %-20s %s\n", col.Name, kindName(col.Kind)))
		}
	}
	return sb.String()
}

func kindName(k types.Kind) string {
	switch k {
	case types.KindInt:
		return "INT"
	case types.KindFloat:
		return "FLOAT"
	case types.KindString:
		return "STRING"
	case types.KindBool:
		return "BOOL"
	default:
		return "NULL"
	}
}

// sortStrings ordena in-place una slice de strings (insertion sort, suficiente para pocos elementos).
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		key := ss[i]
		j := i - 1
		for j >= 0 && ss[j] > key {
			ss[j+1] = ss[j]
			j--
		}
		ss[j+1] = key
	}
}
