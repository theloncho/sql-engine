// Package explain implementa la función EXPLAIN: imprime el árbol de operadores
// del plan de ejecución sin ejecutar la consulta.
// Esto permite visualizar cómo el motor descompone una consulta.
package explain

import (
	"fmt"
	"io"
	"strings"

	"github.com/theloncho/sql-engine/executor"
)

// Print imprime el árbol de operadores al writer dado.
// La representación usa indentación para mostrar la jerarquía padre-hijo.
func Print(op executor.Operator, w io.Writer) {
	printOp(op, w, 0, true)
}

// printOp imprime recursivamente un operador con su nivel de indentación.
func printOp(op executor.Operator, w io.Writer, depth int, isLast bool) {
	indent := strings.Repeat("  ", depth)
	connector := "└─ "
	if !isLast {
		connector = "├─ "
	}
	if depth == 0 {
		connector = ""
	}

	name, children := describeOp(op)
	schema := op.Schema()
	cols := schema.ColNames()

	fmt.Fprintf(w, "%s%s%s\n", indent, connector, name)
	fmt.Fprintf(w, "%s   Output: [%s]\n", indent, strings.Join(cols, ", "))

	for i, child := range children {
		printOp(child, w, depth+1, i == len(children)-1)
	}
}

// describeOp retorna el nombre del operador y sus hijos para EXPLAIN.
// Usa type assertion para identificar cada tipo de operador.
func describeOp(op executor.Operator) (name string, children []executor.Operator) {
	switch o := op.(type) {
	case *executor.TableScan:
		return fmt.Sprintf("TableScan(schema: %d cols)", len(o.Schema().Cols)), nil

	case *executor.SliceScan:
		return "SliceScan", nil

	case *executor.Filter:
		return "Filter (WHERE ...)", []executor.Operator{getChild(o)}

	case *executor.Project:
		cols := o.Schema().ColNames()
		return fmt.Sprintf("Project [%s]", strings.Join(cols, ", ")), []executor.Operator{getProjectChild(o)}

	case *executor.Sort:
		return "Sort (ORDER BY ...)", []executor.Operator{getSortChild(o)}

	case *executor.Limit:
		return "Limit", []executor.Operator{getLimitChild(o)}

	case *executor.Aggregate:
		return "Aggregate (GROUP BY + agg funcs)", []executor.Operator{getAggChild(o)}

	case *executor.NestedLoopJoin:
		return "NestedLoopJoin (ON ...)", getJoinNLChildren(o)

	case *executor.HashJoin:
		return "HashJoin (ON ... [hash])", getJoinHashChildren(o)

	case *executor.Distinct:
		return "Distinct", []executor.Operator{getDistinctChild(o)}

	default:
		return fmt.Sprintf("Unknown(%T)", op), nil
	}
}

// Los siguientes helpers usan reflexión de tipos para extraer hijos de operadores.
// En Go, los campos privados no son accesibles desde fuera del paquete,
// por lo que los operadores necesitan exponer sus hijos para EXPLAIN.
// Usamos un approach de interfaz opcional: los operadores que tienen hijos
// implementan la interfaz ExplainNode.

// ExplainNode es una interfaz opcional que los operadores pueden implementar
// para exponer sus hijos al EXPLAIN. Los operadores sin hijos no la implementan.
type ExplainNode interface {
	Children() []executor.Operator
}

// getChild usa la interfaz ExplainNode si está disponible, o retorna nil.
func getChildren(op executor.Operator) []executor.Operator {
	if en, ok := op.(ExplainNode); ok {
		return en.Children()
	}
	return nil
}

// --- helpers específicos por tipo (fallback para tipos concretos) ---

func getChild(f *executor.Filter) executor.Operator {
	if en, ok := executor.Operator(f).(ExplainNode); ok {
		ch := en.Children()
		if len(ch) > 0 {
			return ch[0]
		}
	}
	return nil
}

func getProjectChild(p *executor.Project) executor.Operator {
	if en, ok := executor.Operator(p).(ExplainNode); ok {
		ch := en.Children()
		if len(ch) > 0 {
			return ch[0]
		}
	}
	return nil
}

func getSortChild(s *executor.Sort) executor.Operator {
	if en, ok := executor.Operator(s).(ExplainNode); ok {
		ch := en.Children()
		if len(ch) > 0 {
			return ch[0]
		}
	}
	return nil
}

func getLimitChild(l *executor.Limit) executor.Operator {
	if en, ok := executor.Operator(l).(ExplainNode); ok {
		ch := en.Children()
		if len(ch) > 0 {
			return ch[0]
		}
	}
	return nil
}

func getAggChild(a *executor.Aggregate) executor.Operator {
	if en, ok := executor.Operator(a).(ExplainNode); ok {
		ch := en.Children()
		if len(ch) > 0 {
			return ch[0]
		}
	}
	return nil
}

func getJoinNLChildren(j *executor.NestedLoopJoin) []executor.Operator {
	if en, ok := executor.Operator(j).(ExplainNode); ok {
		return en.Children()
	}
	return nil
}

func getJoinHashChildren(j *executor.HashJoin) []executor.Operator {
	if en, ok := executor.Operator(j).(ExplainNode); ok {
		return en.Children()
	}
	return nil
}

func getDistinctChild(d *executor.Distinct) executor.Operator {
	if en, ok := executor.Operator(d).(ExplainNode); ok {
		ch := en.Children()
		if len(ch) > 0 {
			return ch[0]
		}
	}
	return nil
}
