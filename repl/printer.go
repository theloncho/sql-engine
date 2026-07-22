// printer.go implementa la impresión de resultados en formato de tabla ASCII.
// La anchura de cada columna se adapta al contenido para una salida legible.
package repl

import (
	"fmt"
	"io"
	"strings"

	"github.com/theloncho/sql-engine/executor"
)

// PrintTable imprime filas en formato de tabla ASCII con encabezados.
// Ejemplo de salida:
//
//	+----+---------+---------+
//	| id | name    | salary  |
//	+----+---------+---------+
//	| 1  | Alice   | 75000   |
//	+----+---------+---------+
func PrintTable(w io.Writer, headers []string, rows []executor.Row) {
	if len(rows) == 0 && len(headers) == 0 {
		fmt.Fprintln(w, "(sin resultados)")
		return
	}

	// Calcular el ancho máximo de cada columna (encabezado vs datos).
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, val := range row {
			if i < len(widths) {
				s := formatValue(val)
				if len(s) > widths[i] {
					widths[i] = len(s)
				}
			}
		}
	}

	// Función para imprimir una línea divisora.
	printSep := func() {
		fmt.Fprint(w, "+")
		for _, colW := range widths {
			fmt.Fprint(w, strings.Repeat("-", colW+2)+"+")
		}
		fmt.Fprintln(w)
	}

	// Función para imprimir una fila de datos.
	printRow := func(cells []string) {
		fmt.Fprint(w, "|")
		for i, cell := range cells {
			if i < len(widths) {
				fmt.Fprintf(w, " %-*s |", widths[i], cell)
			}
		}
		fmt.Fprintln(w)
	}

	// Imprimir tabla.
	printSep()
	printRow(headers)
	printSep()

	if len(rows) == 0 {
		// Tabla vacía: mostrar una fila de mensaje.
		fmt.Fprintln(w, "| (0 filas)"+strings.Repeat(" ", max(0, sumWidths(widths)-8))+" |")
	} else {
		for _, row := range rows {
			cells := make([]string, len(headers))
			for i := range cells {
				if i < len(row) {
					cells[i] = formatValue(row[i])
				}
			}
			printRow(cells)
		}
	}
	printSep()
}

// formatValue convierte un Value a su representación para impresión.
// NULL se muestra como "NULL" para distinguirlo de un string vacío.
func formatValue(v interface{ String() string }) string {
	return v.String()
}

// sumWidths suma todos los anchos de columna.
func sumWidths(ws []int) int {
	total := 0
	for _, w := range ws {
		total += w + 3 // +3 por " | "
	}
	return total
}

// max retorna el mayor de dos enteros.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
