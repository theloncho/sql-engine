// sort.go implementa el operador Sort (ORDER BY).
// Sort es un operador de bloqueo: debe materializar todas las filas del hijo
// antes de poder retornar la primera fila ordenada. Esto es inevitable dado
// que ORDER BY necesita comparar todos los elementos.
package executor

import (
	"fmt"
	"sort"

	"github.com/theloncho/sql-engine/parser"
	"github.com/theloncho/sql-engine/types"
)

// SortKey describe un criterio de ordenación: expresión + dirección.
type SortKey struct {
	Expr parser.Expr
	Asc  bool
}

// Sort materializa todas las filas del hijo y las retorna ordenadas.
type Sort struct {
	child  Operator
	keys   []SortKey
	buffer []Row  // filas materializadas
	cursor int    // siguiente fila a retornar
	ready  bool   // true si ya materializamos y ordenamos
	sortErr error // error capturado durante la comparación (sort.Slice no retorna error)
}

// NewSort crea un operador Sort con los criterios de ordenación dados.
func NewSort(child Operator, keys []SortKey) *Sort {
	return &Sort{child: child, keys: keys}
}

// Next retorna la siguiente fila en orden. En la primera llamada, materializa y ordena.
func (s *Sort) Next() (Row, error) {
	if !s.ready {
		if err := s.materializeAndSort(); err != nil {
			return nil, err
		}
		s.ready = true
	}
	if s.sortErr != nil {
		return nil, s.sortErr
	}
	if s.cursor >= len(s.buffer) {
		return nil, nil // EOF
	}
	row := s.buffer[s.cursor]
	s.cursor++
	return row, nil
}

// materializeAndSort consume todas las filas del hijo y las ordena en memoria.
func (s *Sort) materializeAndSort() error {
	schema := s.child.Schema()
	for {
		row, err := s.child.Next()
		if err != nil {
			return err
		}
		if row == nil {
			break
		}
		s.buffer = append(s.buffer, row)
	}

	// Ordenar usando sort.SliceStable (estable para múltiples claves).
	sort.SliceStable(s.buffer, func(i, j int) bool {
		if s.sortErr != nil {
			return false
		}
		for _, key := range s.keys {
			ctxI := &EvalContext{Row: s.buffer[i], Schema: schema}
			ctxJ := &EvalContext{Row: s.buffer[j], Schema: schema}
			vi, errI := Eval(key.Expr, ctxI)
			vj, errJ := Eval(key.Expr, ctxJ)
			if errI != nil {
				s.sortErr = errI
				return false
			}
			if errJ != nil {
				s.sortErr = errJ
				return false
			}
			// NULL siempre va al final (NULLS LAST, comportamiento estándar SQL).
			// Si vi es NULL, la fila i va detrás de j → retornar false (i no precede a j).
			// Si vj es NULL, la fila j va detrás de i → retornar true (i precede a j).
			if vi.IsNull() && vj.IsNull() {
				continue
			}
			if vi.IsNull() {
				return false // i (NULL) va al final
			}
			if vj.IsNull() {
				return true // j (NULL) va al final
			}
			cmp, err := vi.Cmp(vj)
			if err != nil {
				// Tipos incomparables: los tratamos como iguales.
				continue
			}
			if cmp != 0 {
				if key.Asc {
					return cmp < 0
				}
				return cmp > 0
			}
			// cmp == 0: continuar con la siguiente clave.
		}
		return false
	})
	return nil
}

// Close propaga el cierre al hijo y limpia el buffer.
func (s *Sort) Close() error {
	s.buffer = nil
	s.cursor = 0
	s.ready = false
	s.sortErr = nil
	return s.child.Close()
}

// Schema retorna el mismo esquema que el hijo.
func (s *Sort) Schema() OutputSchema { return s.child.Schema() }

// --- Limit ---

// Limit retorna a lo más N filas del hijo.
type Limit struct {
	child   Operator
	maxRows int64
	count   int64
}

// NewLimit crea un operador Limit.
func NewLimit(child Operator, n int64) (*Limit, error) {
	if n < 0 {
		return nil, fmt.Errorf("LIMIT must be non-negative, got %d", n)
	}
	return &Limit{child: child, maxRows: n}, nil
}

// Next retorna la siguiente fila hasta alcanzar el límite.
func (l *Limit) Next() (Row, error) {
	if l.count >= l.maxRows {
		return nil, nil // límite alcanzado
	}
	row, err := l.child.Next()
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	l.count++
	return row, nil
}

// Close propaga el cierre y resetea el contador.
func (l *Limit) Close() error {
	l.count = 0
	return l.child.Close()
}

// Schema retorna el mismo esquema que el hijo.
func (l *Limit) Schema() OutputSchema { return l.child.Schema() }

// --- Helpers de tipo para compatibilidad ---

// kindFromValue extrae el Kind de un Value (para esquemas dinámicos).
func kindFromValue(v types.Value) types.Kind {
	return v.Kind
}

func (s *Sort) Children() []Operator { return []Operator{s.child} }
func (l *Limit) Children() []Operator { return []Operator{l.child} }
