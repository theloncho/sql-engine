// main.go es el punto de entrada del motor SQL en memoria.
// Puede usarse como REPL interactivo o para ejecutar una consulta directa.
//
// Uso:
//
//	./sql-engine                          # Inicia el REPL interactivo
//	./sql-engine -load data/employees.csv # Carga CSV y abre el REPL
//	./sql-engine -q "SELECT * FROM t"    # Ejecuta una consulta y sale
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/theloncho/sql-engine/catalog"
	"github.com/theloncho/sql-engine/executor"
	"github.com/theloncho/sql-engine/explain"
	"github.com/theloncho/sql-engine/loader"
	"github.com/theloncho/sql-engine/parser"
	"github.com/theloncho/sql-engine/planner"
	"github.com/theloncho/sql-engine/repl"
)

func main() {
	// Flags de línea de comandos.
	var (
		loadFiles = flag.String("load", "", "archivos CSV a cargar (separados por coma)")
		query     = flag.String("q", "", "consulta SQL a ejecutar (modo no-interactivo)")
		explainQ  = flag.Bool("explain", false, "mostrar árbol de operadores sin ejecutar")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Motor de Consultas SQL en Memoria — Go
Uso: sql-engine [opciones]

Opciones:
  -load <archivos>   Cargar uno o más archivos CSV (separados por coma)
  -q <sql>           Ejecutar una consulta SQL y salir
  -explain           Combinar con -q para mostrar el plan de ejecución

Ejemplos:
  sql-engine -load data/employees.csv,data/departments.csv
  sql-engine -load data/employees.csv -q "SELECT * FROM employees LIMIT 5"
  sql-engine -load data/employees.csv -q "SELECT * FROM employees" -explain

`)
		flag.PrintDefaults()
	}
	flag.Parse()

	// Crear catálogo.
	cat := catalog.New()

	// Cargar archivos CSV si se especificaron.
	if *loadFiles != "" {
		files := strings.Split(*loadFiles, ",")
		for _, f := range files {
			f = strings.TrimSpace(f)
			if f == "" {
				continue
			}
			opts := loader.DefaultOptions()
			tbl, err := loader.LoadFile(cat, f, opts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error cargando %q: %v\n", f, err)
				os.Exit(1)
			}
			fmt.Printf("Tabla '%s' cargada: %d filas, %d columnas.\n", tbl.Name, tbl.NumRows(), tbl.NumCols())
		}
		fmt.Println()
	}

	// Modo no-interactivo: ejecutar una consulta y salir.
	if *query != "" {
		if err := runQuery(cat, *query, *explainQ); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Modo interactivo: iniciar el REPL.
	r := repl.New(cat)
	r.Run()
}

// runQuery ejecuta una consulta SQL en modo no-interactivo.
func runQuery(cat *catalog.Catalog, sql string, doExplain bool) error {
	stmt, err := parser.Parse(sql)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	plan, err := planner.Build(stmt, cat)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}
	defer plan.Root.Close()

	if doExplain {
		fmt.Println("== EXPLAIN ==")
		explain.Print(plan.Root, os.Stdout)
		fmt.Println()
		return nil
	}

	// Ejecutar y recolectar filas.
	schema := plan.Root.Schema()
	headers := schema.ColNames()

	var rows []executor.Row
	for {
		row, err := plan.Root.Next()
		if err != nil {
			return fmt.Errorf("execute: %w", err)
		}
		if row == nil {
			break
		}
		rows = append(rows, row)
	}

	repl.PrintTable(os.Stdout, headers, rows)
	fmt.Printf("(%d fila(s))\n", len(rows))
	return nil
}
