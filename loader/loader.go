// Package loader carga archivos CSV a tablas en memoria.
// Las celdas vacías se convierten a NULL. Los tipos se infieren automáticamente
// a partir de la primera fila de datos, o pueden declararse en el encabezado
// con la sintaxis "columna:tipo" (p. ej. "salary:float").
package loader

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/theloncho/sql-engine/catalog"
	"github.com/theloncho/sql-engine/types"
)

// Options controla el comportamiento del cargador.
type Options struct {
	// TableName sobreescribe el nombre de la tabla (por defecto: nombre del archivo sin extensión).
	TableName string
	// InferTypes=true (default) infiere tipos desde la primera fila de datos.
	// InferTypes=false trata todas las columnas como string.
	InferTypes bool
}

// DefaultOptions retorna las opciones por defecto.
func DefaultOptions() Options {
	return Options{InferTypes: true}
}

// LoadFile carga un CSV desde la ruta dada y lo registra en el catálogo.
// El nombre de la tabla se deduce del nombre del archivo (sin extensión) salvo que
// Options.TableName esté definido.
func LoadFile(cat *catalog.Catalog, path string, opts Options) (*catalog.Table, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("loader: cannot open %q: %w", path, err)
	}
	defer f.Close()

	name := opts.TableName
	if name == "" {
		base := filepath.Base(path)
		name = strings.TrimSuffix(base, filepath.Ext(base))
	}
	return LoadReader(cat, f, name, opts)
}

// LoadReader carga un CSV desde un io.Reader. Útil en tests (sin tocar disco).
func LoadReader(cat *catalog.Catalog, r io.Reader, tableName string, opts Options) (*catalog.Table, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	// --- Leer encabezado ---
	header, err := reader.Read()
	if err == io.EOF {
		return nil, fmt.Errorf("loader: %q is empty (no header row)", tableName)
	}
	if err != nil {
		return nil, fmt.Errorf("loader: reading header of %q: %w", tableName, err)
	}

	// Parsear nombres y tipos opcionales del encabezado ("col:tipo").
	colNames, declaredKinds := parseHeader(header)

	// --- Leer todas las filas ---
	var rawRows [][]string
	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("loader: reading row in %q: %w", tableName, err)
		}
		if len(rec) != len(colNames) {
			return nil, fmt.Errorf("loader: row has %d fields, expected %d in %q", len(rec), len(colNames), tableName)
		}
		rawRows = append(rawRows, rec)
	}

	// --- Inferir o usar tipos declarados ---
	schema, err := buildSchema(colNames, declaredKinds, rawRows, opts.InferTypes)
	if err != nil {
		return nil, fmt.Errorf("loader: building schema for %q: %w", tableName, err)
	}

	// --- Parsear filas con tipos finales ---
	rows := make([][]types.Value, 0, len(rawRows))
	for i, raw := range rawRows {
		row := make([]types.Value, len(schema.Columns))
		for j, cell := range raw {
			v, err := types.ParseValueAs(cell, schema.Columns[j].Kind)
			if err != nil {
				return nil, fmt.Errorf("loader: row %d, col %q: %w", i+1, schema.Columns[j].Name, err)
			}
			row[j] = v
		}
		rows = append(rows, row)
	}

	tbl := &catalog.Table{
		Name:   tableName,
		Schema: *schema,
		Rows:   rows,
	}
	cat.Replace(tbl)
	return tbl, nil
}

// parseHeader descompone los nombres de columna, separando el tipo opcional ("col:tipo").
func parseHeader(header []string) (names []string, kinds []types.Kind) {
	names = make([]string, len(header))
	kinds = make([]types.Kind, len(header))
	for i, h := range header {
		h = strings.TrimSpace(h)
		if idx := strings.LastIndex(h, ":"); idx > 0 {
			col := strings.TrimSpace(h[:idx])
			typePart := strings.TrimSpace(h[idx+1:])
			if k, err := types.KindFromString(typePart); err == nil {
				names[i] = col
				kinds[i] = k
				continue
			}
		}
		names[i] = h
		kinds[i] = types.KindNull // KindNull = "no declarado, inferir"
	}
	return
}

// buildSchema determina el tipo final de cada columna.
// Si el tipo fue declarado en el encabezado, se usa tal cual.
// Si InferTypes=true, se infiere del primer valor no-nulo de la columna.
// Si no hay datos o todos son nulos, se usa KindString.
func buildSchema(colNames []string, declaredKinds []types.Kind, rawRows [][]string, inferTypes bool) (*catalog.Schema, error) {
	cols := make([]catalog.Column, len(colNames))
	for i, name := range colNames {
		k := declaredKinds[i]
		if k == types.KindNull && inferTypes {
			// Inferir desde la primera fila no-vacía.
			k = inferKind(i, rawRows)
		}
		if k == types.KindNull {
			k = types.KindString // fallback
		}
		cols[i] = catalog.Column{Name: name, Kind: k}
	}
	return &catalog.Schema{Columns: cols}, nil
}

// inferKind recorre la columna col en rawRows y retorna el Kind del primer valor no-vacío.
func inferKind(col int, rawRows [][]string) types.Kind {
	for _, row := range rawRows {
		if col < len(row) && strings.TrimSpace(row[col]) != "" {
			v := types.ParseValue(row[col])
			if v.Kind != types.KindNull {
				return v.Kind
			}
		}
	}
	return types.KindNull // todos nulos → fallback a string en el llamador
}
