// Package executor implementa el modelo de ejecución por iteradores (Volcano/Iterator model).
//
// # Modelo Volcano
//
// Cada operador implementa la interfaz Operator con un único método Next() que retorna
// la siguiente fila disponible. Los operadores se encadenan en un árbol:
//
//	Project → Sort → Filter → Scan
//
// El operador raíz se llama repetidamente hasta que Next() retorna (nil, nil),
// señal de que no hay más filas.
//
// Esta arquitectura permite evaluación perezosa (lazy evaluation): las filas
// se producen y consumen una a una, sin materializar conjuntos intermedios completos
// (excepto Sort y Aggregate, que por su naturaleza deben materializar).
package executor

import "github.com/theloncho/sql-engine/types"

// Row es una fila de datos: una slice de Values alineada con el esquema del operador.
type Row = []types.Value

// Operator es la interfaz central del modelo Volcano.
// Todo operador — Scan, Filter, Project, Sort, Limit, Aggregate, Join, Distinct —
// implementa esta interfaz. Agregar un operador nuevo no requiere modificar los existentes.
type Operator interface {
	// Next retorna la siguiente fila disponible.
	//   - (row, nil): hay una fila válida.
	//   - (nil, nil): no hay más filas (EOF).
	//   - (nil, err): ocurrió un error.
	Next() (Row, error)

	// Close libera los recursos de este operador y propaga Close() a sus hijos.
	// Debe llamarse siempre, incluso si se interrumpe la iteración antes de EOF.
	Close() error

	// Schema retorna los nombres y tipos de las columnas que produce este operador.
	// Permite a los operadores superiores resolver referencias de columnas.
	Schema() OutputSchema
}

// OutputSchema describe las columnas que produce un operador.
type OutputSchema struct {
	Cols []OutputCol
}

// OutputCol describe una columna de salida: nombre, tipo, y calificador de tabla (para JOINs).
type OutputCol struct {
	Table string     // nombre o alias de tabla; vacío para columnas calculadas
	Name  string     // nombre de la columna
	Kind  types.Kind // tipo del valor
}

// IndexOf retorna el índice de la columna con el nombre dado (y tabla opcional).
// Si table="", busca solo por nombre.
// Retorna -1 si no encuentra la columna.
func (s OutputSchema) IndexOf(table, col string) int {
	// Búsqueda calificada: tabla.columna
	if table != "" {
		for i, c := range s.Cols {
			if c.Table == table && c.Name == col {
				return i
			}
		}
		return -1
	}
	// Búsqueda no calificada: puede haber ambigüedad, pero retornamos el primero.
	for i, c := range s.Cols {
		if c.Name == col {
			return i
		}
	}
	return -1
}

// ColNames retorna solo los nombres de columna (para el printer del REPL).
func (s OutputSchema) ColNames() []string {
	names := make([]string, len(s.Cols))
	for i, c := range s.Cols {
		if c.Table != "" {
			names[i] = c.Table + "." + c.Name
		} else {
			names[i] = c.Name
		}
	}
	return names
}
