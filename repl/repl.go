// Package repl implementa el Read-Eval-Print Loop del motor SQL.
// El REPL permite al usuario ingresar consultas SQL de forma interactiva,
// ejecutarlas y ver los resultados en formato tabular.
package repl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/theloncho/sql-engine/catalog"
	"github.com/theloncho/sql-engine/executor"
	"github.com/theloncho/sql-engine/explain"
	"github.com/theloncho/sql-engine/loader"
	"github.com/theloncho/sql-engine/parser"
	"github.com/theloncho/sql-engine/planner"
)

// REPL mantiene el estado de la sesión interactiva.
type REPL struct {
	catalog *catalog.Catalog
	in      io.Reader
	out     io.Writer
}

// New crea un REPL con el catálogo dado.
func New(cat *catalog.Catalog) *REPL {
	return &REPL{catalog: cat, in: os.Stdin, out: os.Stdout}
}

// NewWithIO crea un REPL con readers/writers personalizados (útil para tests).
func NewWithIO(cat *catalog.Catalog, in io.Reader, out io.Writer) *REPL {
	return &REPL{catalog: cat, in: in, out: out}
}

// Run inicia el loop del REPL y no retorna hasta que el usuario escribe "quit" o "exit".
func (r *REPL) Run() {
	r.printf("SQL Engine REPL — escribe 'help' para ayuda, 'quit' para salir.\n\n")
	scanner := bufio.NewScanner(r.in)
	var queryBuf strings.Builder

	for {
		if queryBuf.Len() == 0 {
			r.printf("sql> ")
		} else {
			r.printf("  -> ")
		}

		if !scanner.Scan() {
			break
		}
		line := scanner.Text()

		// Comandos especiales (sin necesidad de ;)
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if lower == "quit" || lower == "exit" || lower == "\\q" {
			r.printf("¡Hasta luego!\n")
			break
		}
		if lower == "help" || lower == "\\h" {
			r.printHelp()
			continue
		}
		if lower == "\\d" || lower == "tables" {
			r.printTables()
			continue
		}
		if strings.HasPrefix(lower, "\\load ") || strings.HasPrefix(lower, "load ") {
			r.handleLoad(trimmed)
			continue
		}

		// Acumular líneas hasta encontrar ;
		queryBuf.WriteString(line)
		queryBuf.WriteString(" ")

		if strings.Contains(line, ";") || !strings.Contains(line, " ") && len(trimmed) > 0 {
			// Ejecutar la consulta acumulada.
			query := strings.TrimSpace(queryBuf.String())
			queryBuf.Reset()
			r.executeQuery(query)
		}
	}
}

// executeQuery parsea, planifica y ejecuta una consulta SQL, imprimiendo los resultados.
func (r *REPL) executeQuery(query string) {
	// ¿EXPLAIN?
	isExplain := false
	upperQ := strings.ToUpper(strings.TrimSpace(query))
	if strings.HasPrefix(upperQ, "EXPLAIN ") {
		isExplain = true
		query = strings.TrimSpace(query[7:])
	}

	// Parsear.
	stmt, err := parser.Parse(query)
	if err != nil {
		r.printf("Error de sintaxis: %v\n\n", err)
		return
	}

	// Planificar.
	plan, err := planner.Build(stmt, r.catalog)
	if err != nil {
		r.printf("Error de planificación: %v\n\n", err)
		return
	}
	defer plan.Root.Close()

	// EXPLAIN: solo imprimir el árbol, no ejecutar.
	if isExplain {
		r.printf("== EXPLAIN ==\n")
		explain.Print(plan.Root, r.out)
		r.printf("\n")
		return
	}

	// Ejecutar y recolectar filas.
	var rows []executor.Row
	schema := plan.Root.Schema()

	for {
		row, err := plan.Root.Next()
		if err != nil {
			r.printf("Error de ejecución: %v\n\n", err)
			return
		}
		if row == nil {
			break
		}
		rows = append(rows, row)
	}

	// Imprimir resultados.
	PrintTable(r.out, schema.ColNames(), rows)
	r.printf("(%d fila(s))\n\n", len(rows))
}

// handleLoad maneja el comando "\load archivo.csv" o "load archivo.csv".
func (r *REPL) handleLoad(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) < 2 {
		r.printf("Uso: load <archivo.csv> [nombre_tabla]\n")
		return
	}
	path := parts[1]
	opts := loader.DefaultOptions()
	if len(parts) >= 3 {
		opts.TableName = parts[2]
	}
	tbl, err := loader.LoadFile(r.catalog, path, opts)
	if err != nil {
		r.printf("Error al cargar: %v\n", err)
		return
	}
	r.printf("Tabla '%s' cargada: %d filas, %d columnas.\n\n", tbl.Name, tbl.NumRows(), tbl.NumCols())
}

// printTables muestra el catálogo de tablas y sus esquemas.
func (r *REPL) printTables() {
	r.printf("%s", r.catalog.Describe())
	r.printf("\n")
}

// printHelp muestra la ayuda del REPL.
func (r *REPL) printHelp() {
	r.printf(`Comandos disponibles:
  <consulta SQL>  Ejecutar una consulta (termina con ; o en una sola línea)
  EXPLAIN <sql>   Mostrar el árbol de operadores sin ejecutar
  load <csv>      Cargar un archivo CSV como tabla
  \d / tables     Listar tablas del catálogo
  help / \h       Mostrar esta ayuda
  quit / exit     Salir del REPL

Ejemplos:
  SELECT * FROM employees;
  SELECT name, salary FROM employees WHERE salary > 70000 ORDER BY salary DESC;
  SELECT dept_id, COUNT(*), AVG(salary) FROM employees GROUP BY dept_id;
  SELECT e.name, d.name FROM employees AS e INNER JOIN departments AS d ON e.dept_id = d.id;
  EXPLAIN SELECT * FROM employees WHERE salary > 50000;

`)
}

func (r *REPL) printf(format string, args ...interface{}) {
	fmt.Fprintf(r.out, format, args...)
}
