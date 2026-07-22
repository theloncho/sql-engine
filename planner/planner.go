// Package planner construye el árbol de operadores a partir del AST.
// El planner es el puente entre el parser y el executor:
// recibe un *SelectStmt del parser y retorna el operador raíz listo para ejecutar.
//
// Responsabilidades del planner:
//  - Resolver nombres de tabla y columna contra el catálogo.
//  - Construir el árbol de operadores en el orden correcto (Scan → Filter → Aggregate → Sort → Limit → Project).
//  - Elegir entre NestedLoopJoin y HashJoin según la condición ON.
//  - Extraer las especificaciones de agregación del AST y separar las columnas GROUP BY.
package planner

import (
	"fmt"
	"strings"

	"github.com/theloncho/sql-engine/catalog"
	"github.com/theloncho/sql-engine/executor"
	"github.com/theloncho/sql-engine/parser"
)

// Plan es el resultado del planner: el operador raíz del árbol de ejecución.
type Plan struct {
	Root    executor.Operator
	Explain bool // true si se solicitó EXPLAIN
}

// Build construye el plan de ejecución a partir del AST y el catálogo.
func Build(stmt *parser.SelectStmt, cat *catalog.Catalog) (*Plan, error) {
	return build(stmt, cat, false)
}

// BuildExplain construye el plan marcado para EXPLAIN.
func BuildExplain(stmt *parser.SelectStmt, cat *catalog.Catalog) (*Plan, error) {
	return build(stmt, cat, true)
}

func build(stmt *parser.SelectStmt, cat *catalog.Catalog, explain bool) (*Plan, error) {
	// 1. Construir el operador base (tabla principal + JOINs).
	base, err := buildSource(stmt, cat)
	if err != nil {
		return nil, err
	}

	// 2. Aplicar WHERE (Filter).
	var current executor.Operator = base
	if stmt.Where != nil {
		current = executor.NewFilter(current, stmt.Where)
	}

	// 3. Determinar si la consulta tiene funciones de agregación.
	hasAgg := hasAggregates(stmt.Cols) || len(stmt.GroupBy) > 0
	if hasAgg {
		aggOp, err := buildAggregate(current, stmt)
		if err != nil {
			return nil, err
		}
		current = aggOp
	}

	// 4. ORDER BY (Sort).
	if len(stmt.OrderBy) > 0 {
		keys := make([]executor.SortKey, len(stmt.OrderBy))
		for i, o := range stmt.OrderBy {
			keys[i] = executor.SortKey{Expr: o.Expr, Asc: o.Asc}
		}
		current = executor.NewSort(current, keys)
	}

	// 5. LIMIT.
	if stmt.Limit != nil {
		lim, err := executor.NewLimit(current, *stmt.Limit)
		if err != nil {
			return nil, err
		}
		current = lim
	}

	// 6. Project (SELECT cols).
	// Si hay agregación ya aplicamos la proyección en buildAggregate.
	if !hasAgg {
		proj, err := executor.NewProject(current, stmt.Cols)
		if err != nil {
			return nil, fmt.Errorf("planner: project: %w", err)
		}
		current = proj
	}

	// 7. DISTINCT.
	if stmt.Distinct {
		current = executor.NewDistinct(current)
	}

	return &Plan{Root: current, Explain: explain}, nil
}

// buildSource construye la fuente de datos: TableScan + JOINs.
func buildSource(stmt *parser.SelectStmt, cat *catalog.Catalog) (executor.Operator, error) {
	// Tabla principal.
	mainTbl, err := cat.Get(stmt.From.Name)
	if err != nil {
		return nil, fmt.Errorf("planner: %w", err)
	}
	var current executor.Operator = executor.NewTableScan(mainTbl, stmt.From.EffectiveName())

	// Aplicar JOINs encadenados.
	for _, join := range stmt.Joins {
		joinTbl, err := cat.Get(join.Table.Name)
		if err != nil {
			return nil, fmt.Errorf("planner: JOIN table: %w", err)
		}
		inner := executor.NewTableScan(joinTbl, join.Table.EffectiveName())

		// ¿Podemos usar HashJoin? La condición ON debe ser "col = col".
		if outerKey, innerKey, ok := extractEquiJoinKeys(join.On, current.Schema(), inner.Schema()); ok {
			current = executor.NewHashJoin(current, inner, outerKey, innerKey)
		} else {
			current = executor.NewNestedLoopJoin(current, inner, join.On)
		}
	}
	return current, nil
}

// buildAggregate construye el operador Aggregate y el Project final para consultas con agregación.
func buildAggregate(child executor.Operator, stmt *parser.SelectStmt) (executor.Operator, error) {
	// Separar las columnas del SELECT en: columnas de GROUP BY y funciones de agregación.
	var aggSpecs []executor.AggSpec
	var projCols []parser.SelectCol

	for _, col := range stmt.Cols {
		if col.IsWildcard {
			return nil, fmt.Errorf("planner: SELECT * not allowed with GROUP BY")
		}
		if agg, ok := col.Expr.(*parser.AggFunc); ok {
			alias := col.Alias
			if alias == "" {
				if agg.IsStar {
					alias = agg.Name + "(*)"
				} else {
					alias = agg.Name + "(" + exprName(agg.Arg) + ")"
				}
			}
			aggSpecs = append(aggSpecs, executor.AggSpec{
				Func:   strings.ToUpper(agg.Name),
				Arg:    agg.Arg,
				IsStar: agg.IsStar,
				Alias:  alias,
			})
		}
		// Las columnas no-agregadas van al projCols (se proyectan después del Aggregate).
		projCols = append(projCols, col)
	}

	aggOp := executor.NewAggregate(child, stmt.GroupBy, aggSpecs)

	// Proyectar las columnas finales desde el resultado del Aggregate.
	// El esquema del Aggregate ya tiene los nombres correctos, así que hacemos
	// un Project que simplemente selecciona en el orden pedido.
	// Si no hay columnas no-agregadas aparte de GROUP BY, podemos omitir el Project.
	proj, err := buildAggProject(aggOp, stmt)
	if err != nil {
		return nil, err
	}
	return proj, nil
}

// buildAggProject construye la proyección final sobre el resultado del Aggregate.
func buildAggProject(aggOp executor.Operator, stmt *parser.SelectStmt) (executor.Operator, error) {
	aggSchema := aggOp.Schema()
	var outCols []parser.SelectCol

	for _, col := range stmt.Cols {
		if col.IsWildcard {
			return nil, fmt.Errorf("SELECT * not allowed with aggregation")
		}
		// Buscar el nombre de la columna en el esquema del Aggregate.
		name := ""
		if agg, ok := col.Expr.(*parser.AggFunc); ok {
			alias := col.Alias
			if alias == "" {
				if agg.IsStar {
					alias = agg.Name + "(*)"
				} else {
					alias = agg.Name + "(" + exprName(agg.Arg) + ")"
				}
			}
			name = alias
		} else {
			name = exprName(col.Expr)
			if col.Alias != "" {
				name = col.Alias
			}
		}

		// Crear un Identifier que apunta a la columna del Aggregate por nombre.
		idx := aggSchema.IndexOf("", name)
		if idx < 0 {
			// Intentar con el nombre sin alias.
			name = exprName(col.Expr)
			idx = aggSchema.IndexOf("", name)
		}
		if idx < 0 {
			return nil, fmt.Errorf("column %q not found in aggregate result", name)
		}

		outCols = append(outCols, parser.SelectCol{
			Expr:  &parser.Identifier{Column: aggSchema.Cols[idx].Name},
			Alias: col.Alias,
		})
	}
	return executor.NewProject(aggOp, outCols)
}

// --- Detección de equi-join para HashJoin ---

// extractEquiJoinKeys intenta extraer un par (outerKey, innerKey) de una condición ON simple
// de la forma "col_outer = col_inner". Retorna ok=false si la condición no es un equi-join simple.
func extractEquiJoinKeys(cond parser.Expr, outerSchema, innerSchema executor.OutputSchema) (parser.Expr, parser.Expr, bool) {
	bin, ok := cond.(*parser.BinaryExpr)
	if !ok || bin.Op != "=" {
		return nil, nil, false
	}
	// Verificar que los dos lados son identificadores.
	leftIdent, lok := bin.Left.(*parser.Identifier)
	rightIdent, rok := bin.Right.(*parser.Identifier)
	if !lok || !rok {
		return nil, nil, false
	}

	// ¿El lado izquierdo pertenece a outer y el derecho a inner, o viceversa?
	leftInOuter := outerSchema.IndexOf(leftIdent.Table, leftIdent.Column) >= 0
	rightInInner := innerSchema.IndexOf(rightIdent.Table, rightIdent.Column) >= 0
	if leftInOuter && rightInInner {
		return bin.Left, bin.Right, true
	}

	rightInOuter := outerSchema.IndexOf(rightIdent.Table, rightIdent.Column) >= 0
	leftInInner := innerSchema.IndexOf(leftIdent.Table, leftIdent.Column) >= 0
	if rightInOuter && leftInInner {
		return bin.Right, bin.Left, true
	}

	return nil, nil, false
}

// --- Helpers ---

// hasAggregates reporta si alguna columna del SELECT contiene una función de agregación.
func hasAggregates(cols []parser.SelectCol) bool {
	for _, c := range cols {
		if containsAgg(c.Expr) {
			return true
		}
	}
	return false
}

// containsAgg busca recursivamente funciones de agregación en una expresión.
func containsAgg(expr parser.Expr) bool {
	if expr == nil {
		return false
	}
	switch e := expr.(type) {
	case *parser.AggFunc:
		return true
	case *parser.BinaryExpr:
		return containsAgg(e.Left) || containsAgg(e.Right)
	case *parser.UnaryExpr:
		return containsAgg(e.Expr)
	}
	return false
}

// exprName replica la función del executor para nombrar expresiones.
func exprName(expr parser.Expr) string {
	if expr == nil {
		return ""
	}
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
