// aggregate.go implementa el operador Aggregate (GROUP BY + COUNT/SUM/AVG/MIN/MAX).
// Es un operador de bloqueo: consume todas las filas del hijo, las agrupa,
// y luego retorna los resultados de agregación de grupo en grupo.
//
// Reglas de NULL en agregados (SQL estándar):
//   - COUNT(*) cuenta todas las filas incluyendo NULLs.
//   - COUNT(col), SUM, AVG, MIN, MAX ignoran valores NULL.
//   - Si todos los valores son NULL, SUM/AVG/MIN/MAX retornan NULL.
package executor

import (
	"fmt"
	"math"

	"github.com/theloncho/sql-engine/parser"
	"github.com/theloncho/sql-engine/types"
)

// Aggregate is a blocking operator that performs GROUP BY operations.
// It consumes all input rows before producing any output rows.
// This is typical in SQL engines for aggregation queries.
// AggSpec describe una función de agregación a calcular.
type AggSpec struct {
	Func   string      // COUNT, SUM, AVG, MIN, MAX
	Arg    parser.Expr // nil para COUNT(*)
	IsStar bool        // true para COUNT(*)
	Alias  string      // nombre de columna en la salida
}

// accumulator mantiene el estado de acumulación para un grupo.
type accumulator struct {
	groupVals []types.Value // valores de las columnas GROUP BY
	counts    []int64       // para COUNT
	sums      []float64     // para SUM y AVG
	sumCounts []int64       // filas no-NULL para AVG
	mins      []types.Value // para MIN
	maxs      []types.Value // para MAX
	hasValue  []bool        // ¿hay al menos un valor no-NULL?
	totalRows int64         // para COUNT(*)
}

// Aggregate implementa GROUP BY con funciones de agregación.
type Aggregate struct {
	child    Operator
	groupBy  []parser.Expr // expresiones de agrupación
	aggSpecs []AggSpec     // funciones de agregación
	results  []Row         // filas de resultado ya calculadas
	cursor   int
	ready    bool
	schema   OutputSchema
}

// NewAggregate crea un operador de agregación.
// groupBy puede ser vacío (para SELECT COUNT(*) FROM t sin GROUP BY).
func NewAggregate(child Operator, groupBy []parser.Expr, aggs []AggSpec) *Aggregate {
	// El esquema de salida tiene las columnas de GROUP BY + las columnas de agregación.
	childSchema := child.Schema()
	outCols := make([]OutputCol, 0)

	for _, gExpr := range groupBy {
		name := exprName(gExpr)
		kind := types.KindNull
		if ident, ok := gExpr.(*parser.Identifier); ok {
			if idx := childSchema.IndexOf(ident.Table, ident.Column); idx >= 0 {
				kind = childSchema.Cols[idx].Kind
			}
		}
		outCols = append(outCols, OutputCol{Name: name, Kind: kind})
	}
	for _, agg := range aggs {
		kind := types.KindNull
		switch agg.Func {
		case "COUNT":
			kind = types.KindInt
		case "SUM", "AVG":
			kind = types.KindFloat
		case "MIN", "MAX":
			// El tipo depende de la columna; usamos KindNull (dinámico en runtime).
			kind = types.KindNull
		}
		outCols = append(outCols, OutputCol{Name: agg.Alias, Kind: kind})
	}

	return &Aggregate{
		child:    child,
		groupBy:  groupBy,
		aggSpecs: aggs,
		schema:   OutputSchema{Cols: outCols},
	}
}

// Next retorna la siguiente fila de resultado de agregación.
func (a *Aggregate) Next() (Row, error) {
	if !a.ready {
		if err := a.compute(); err != nil {
			return nil, err
		}
		a.ready = true
	}
	if a.cursor >= len(a.results) {
		return nil, nil
	}
	row := a.results[a.cursor]
	a.cursor++
	return row, nil
}

// compute is the core of the aggregation process.
// It iterates over all rows, groups them using a key,
// and updates accumulators for each aggregation function.
// compute consume todas las filas del hijo, las agrupa y calcula los agregados.
func (a *Aggregate) compute() error {
	childSchema := a.child.Schema()

	// Mapa: clave string → acumulador. Slice para orden determinista.
	keyOrder := []string{}
	groups := map[string]*accumulator{}

	for {
		row, err := a.child.Next()
		if err != nil {
			return err
		}
		if row == nil {
			break
		}

		ctx := &EvalContext{Row: row, Schema: childSchema}

		// Calcular clave de grupo.
		groupVals := make([]types.Value, len(a.groupBy))
		for i, gExpr := range a.groupBy {
			v, err := Eval(gExpr, ctx)
			if err != nil {
				return fmt.Errorf("GROUP BY eval: %w", err)
			}
			groupVals[i] = v
		}
		key := rowKey(groupVals)

		// Crear acumulador si es el primer elemento del grupo.
		acc, exists := groups[key]
		if !exists {
			acc = &accumulator{
				groupVals: groupVals,
				counts:    make([]int64, len(a.aggSpecs)),
				sums:      make([]float64, len(a.aggSpecs)),
				sumCounts: make([]int64, len(a.aggSpecs)),
				mins:      make([]types.Value, len(a.aggSpecs)),
				maxs:      make([]types.Value, len(a.aggSpecs)),
				hasValue:  make([]bool, len(a.aggSpecs)),
			}
			for i := range a.aggSpecs {
				acc.mins[i] = types.Null()
				acc.maxs[i] = types.Null()
			}
			groups[key] = acc
			keyOrder = append(keyOrder, key)
		}
		acc.totalRows++

		// Actualizar acumuladores de cada función de agregación.
		for i, spec := range a.aggSpecs {
			if spec.IsStar {
				acc.counts[i]++ // COUNT(*) siempre incrementa
				continue
			}
			v, err := Eval(spec.Arg, ctx)
			if err != nil {
				return fmt.Errorf("agg %s eval: %w", spec.Func, err)
			}
			if v.IsNull() {
				continue // ignorar NULL (excepto COUNT(*))
			}
			switch spec.Func {
			case "COUNT":
				acc.counts[i]++
			case "SUM", "AVG":
				f, ok := toFloat(v)
				if !ok {
					return fmt.Errorf("SUM/AVG requires numeric column, got %s", v.TypeName())
				}
				acc.sums[i] += f
				acc.sumCounts[i]++
				acc.hasValue[i] = true
			case "MIN":
				if !acc.hasValue[i] {
					acc.mins[i] = v
					acc.hasValue[i] = true
				} else {
					cmp, _ := v.Cmp(acc.mins[i])
					if cmp < 0 {
						acc.mins[i] = v
					}
				}
			case "MAX":
				if !acc.hasValue[i] {
					acc.maxs[i] = v
					acc.hasValue[i] = true
				} else {
					cmp, _ := v.Cmp(acc.maxs[i])
					if cmp > 0 {
						acc.maxs[i] = v
					}
				}
			}
		}
	}

	// Si no hay filas y no hay GROUP BY, retornar una fila con ceros/nulls.
	if len(groups) == 0 && len(a.groupBy) == 0 {
		row := a.buildResultRow(&accumulator{
			groupVals: nil,
			counts:    make([]int64, len(a.aggSpecs)),
			sums:      make([]float64, len(a.aggSpecs)),
			sumCounts: make([]int64, len(a.aggSpecs)),
			mins:      make([]types.Value, len(a.aggSpecs)),
			maxs:      make([]types.Value, len(a.aggSpecs)),
			hasValue:  make([]bool, len(a.aggSpecs)),
		})
		a.results = []Row{row}
		return nil
	}

	// Construir filas de resultado.
	a.results = make([]Row, 0, len(groups))
	for _, key := range keyOrder {
		acc := groups[key]
		a.results = append(a.results, a.buildResultRow(acc))
	}
	return nil
}

// buildResultRow construye la fila de resultado para un grupo.
// buildResultRow transforms accumulated group data into a final output row.
// It applies SQL aggregation rules like NULL handling and division for AVG.
func (a *Aggregate) buildResultRow(acc *accumulator) Row {
	row := make(Row, 0, len(a.groupBy)+len(a.aggSpecs))
	row = append(row, acc.groupVals...)
	for i, spec := range a.aggSpecs {
		var v types.Value
		switch spec.Func {
		case "COUNT":
			v = types.IntVal(acc.counts[i])
		case "SUM":
			if !acc.hasValue[i] {
				v = types.Null()
			} else {
				v = types.FloatVal(acc.sums[i])
			}
		case "AVG":
			if acc.sumCounts[i] == 0 {
				v = types.Null()
			} else {
				v = types.FloatVal(acc.sums[i] / float64(acc.sumCounts[i]))
			}
		case "MIN":
			v = acc.mins[i]
		case "MAX":
			v = acc.maxs[i]
		default:
			v = types.Null()
		}
		row = append(row, v)
	}
	return row
}

// toFloat convierte un Value numérico a float64. Retorna (0, false) si no es numérico.
func toFloat(v types.Value) (float64, bool) {
	switch v.Kind {
	case types.KindInt:
		return float64(v.IVal), true
	case types.KindFloat:
		return v.FVal, true
	}
	return math.NaN(), false
}

// Close limpia el estado del operador.
func (a *Aggregate) Close() error {
	a.results = nil
	a.cursor = 0
	a.ready = false
	return a.child.Close()
}

// Schema retorna el esquema de salida del Aggregate.
func (a *Aggregate) Schema() OutputSchema { return a.schema }

func (a *Aggregate) Children() []Operator { return []Operator{a.child} }
