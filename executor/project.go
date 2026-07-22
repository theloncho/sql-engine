// project.go implementa el operador Project (SELECT cols).
// Transforma cada fila del hijo: selecciona, renombra y evalúa expresiones.
// Soporta SELECT *, listas de columnas con alias, y expresiones calculadas.
package executor

import (
	"fmt"

	"github.com/theloncho/sql-engine/parser"
	"github.com/theloncho/sql-engine/types"
)

// Project aplica una lista de expresiones sobre cada fila del hijo,
// produciendo filas con las columnas especificadas en el SELECT.
type Project struct {
	child  Operator
	cols   []parser.SelectCol
	schema OutputSchema // esquema de salida calculado en construcción
}

// NewProject crea un operador Project.
// Retorna error si alguna columna referenciada no existe en el esquema del hijo.
func NewProject(child Operator, cols []parser.SelectCol) (*Project, error) {
	childSchema := child.Schema()

	// Si hay un wildcard SELECT *, expandir todas las columnas del hijo.
	hasWildcard := false
	for _, c := range cols {
		if c.IsWildcard {
			hasWildcard = true
			break
		}
	}

	var outCols []OutputCol
	if hasWildcard {
		// SELECT * → todas las columnas del hijo
		outCols = make([]OutputCol, len(childSchema.Cols))
		copy(outCols, childSchema.Cols)
	} else {
		outCols = make([]OutputCol, 0, len(cols))
		for _, c := range cols {
			outCol, err := resolveOutputCol(c, &childSchema)
			if err != nil {
				return nil, err
			}
			outCols = append(outCols, outCol)
		}
	}

	return &Project{
		child:  child,
		cols:   cols,
		schema: OutputSchema{Cols: outCols},
	}, nil
}

// resolveOutputCol determina el nombre y tipo de una columna de salida.
func resolveOutputCol(c parser.SelectCol, childSchema *OutputSchema) (OutputCol, error) {
	// Determinar el nombre de salida.
	outName := ""
	if c.Alias != "" {
		outName = c.Alias
	} else {
		outName = exprName(c.Expr)
	}

	// Determinar el tipo de salida intentando resolverlo desde el esquema del hijo.
	kind := types.KindNull
	if ident, ok := c.Expr.(*parser.Identifier); ok {
		idx := childSchema.IndexOf(ident.Table, ident.Column)
		if idx == -1 {
			if ident.Table != "" {
				return OutputCol{}, fmt.Errorf("column %q.%q not found", ident.Table, ident.Column)
			}
			return OutputCol{}, fmt.Errorf("column %q not found", ident.Column)
		}
		kind = childSchema.Cols[idx].Kind
	} else {
		// Para expresiones calculadas, el tipo se determina en runtime.
		// Lo marcamos como KindNull aquí y el printer usa el tipo del Value real.
		kind = types.KindNull
	}

	table := ""
	if c.Alias == "" {
		if ident, ok := c.Expr.(*parser.Identifier); ok {
			table = ident.Table
		}
	}

	return OutputCol{Table: table, Name: outName, Kind: kind}, nil
}

// exprName genera un nombre descriptivo para una expresión (sin alias).
func exprName(expr parser.Expr) string {
	switch e := expr.(type) {
	case *parser.Identifier:
		return e.Column
	case *parser.AggFunc:
		if e.IsStar {
			return e.Name + "(*)"
		}
		return e.Name + "(" + exprName(e.Arg) + ")"
	default:
		return "expr"
	}
}

// Next evalúa las expresiones del SELECT sobre la siguiente fila del hijo.
func (p *Project) Next() (Row, error) {
	childRow, err := p.child.Next()
	if err != nil {
		return nil, err
	}
	if childRow == nil {
		return nil, nil // EOF
	}

	childSchema := p.child.Schema()

	// SELECT *: pasar la fila completa
	if len(p.cols) == 1 && p.cols[0].IsWildcard {
		return childRow, nil
	}

	ctx := &EvalContext{Row: childRow, Schema: childSchema}
	outRow := make(Row, len(p.cols))
	for i, col := range p.cols {
		if col.IsWildcard {
			// Wildcard en medio de una lista no está soportado en esta versión.
			return nil, fmt.Errorf("wildcard (*) must be the only column in SELECT")
		}
		v, err := Eval(col.Expr, ctx)
		if err != nil {
			return nil, fmt.Errorf("projecting column %d: %w", i, err)
		}
		outRow[i] = v
	}
	return outRow, nil
}

// Close propaga el cierre al hijo.
func (p *Project) Close() error { return p.child.Close() }

// Schema retorna el esquema de salida del Project.
func (p *Project) Schema() OutputSchema { return p.schema }
