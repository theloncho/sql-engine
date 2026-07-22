// distinct.go implementa el operador Distinct (SELECT DISTINCT).
// Elimina filas duplicadas usando un mapa de hashes de filas ya vistas.
// Es un operador pipeline: no necesita materializar todas las filas,
// pero sí mantiene el estado de las filas vistas (overhead de memoria O(n)).
package executor

// Distinct filtra filas duplicadas.
// La deduplicación usa la función rowKey (definida en scan.go) que serializa
// la fila completa como string para usarla como clave de mapa.
type Distinct struct {
	child Operator
	seen  map[string]struct{} // conjunto de claves de filas ya emitidas
}

// NewDistinct crea un operador Distinct.
func NewDistinct(child Operator) *Distinct {
	return &Distinct{
		child: child,
		seen:  make(map[string]struct{}),
	}
}

// Next retorna la siguiente fila no duplicada.
func (d *Distinct) Next() (Row, error) {
	for {
		row, err := d.child.Next()
		if err != nil {
			return nil, err
		}
		if row == nil {
			return nil, nil // EOF
		}
		key := rowKey(row)
		if _, exists := d.seen[key]; exists {
			continue // ya vista: descartar
		}
		d.seen[key] = struct{}{}
		return row, nil
	}
}

// Close propaga el cierre y limpia el estado de deduplicación.
func (d *Distinct) Close() error {
	d.seen = make(map[string]struct{})
	return d.child.Close()
}

// Schema retorna el mismo esquema que el hijo.
func (d *Distinct) Schema() OutputSchema { return d.child.Schema() }
