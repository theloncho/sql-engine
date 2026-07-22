package loader

import (
	"strings"
	"testing"

	"github.com/theloncho/sql-engine/catalog"
	"github.com/theloncho/sql-engine/types"
)

func TestLoadReader_Basic(t *testing.T) {
	csv := `id,name,salary
1,Alice,75000.00
2,Bob,82000.50
`
	cat := catalog.New()
	tbl, err := LoadReader(cat, strings.NewReader(csv), "employees", DefaultOptions())
	if err != nil {
		t.Fatalf("LoadReader: %v", err)
	}
	if tbl.NumRows() != 2 {
		t.Errorf("rows = %d, want 2", tbl.NumRows())
	}
	if tbl.NumCols() != 3 {
		t.Errorf("cols = %d, want 3", tbl.NumCols())
	}
	// El id debe haberse inferido como int.
	if tbl.Schema.Columns[0].Kind != types.KindInt {
		t.Errorf("col 'id' kind = %d, want KindInt", tbl.Schema.Columns[0].Kind)
	}
	// salary debe ser float.
	if tbl.Schema.Columns[2].Kind != types.KindFloat {
		t.Errorf("col 'salary' kind = %d, want KindFloat", tbl.Schema.Columns[2].Kind)
	}
}

func TestLoadReader_NullCell(t *testing.T) {
	csv := `id,salary
1,
2,50000
`
	cat := catalog.New()
	tbl, err := LoadReader(cat, strings.NewReader(csv), "t", DefaultOptions())
	if err != nil {
		t.Fatalf("LoadReader: %v", err)
	}
	// Primera fila, salary es NULL.
	if !tbl.Rows[0][1].IsNull() {
		t.Error("expected NULL for empty salary cell")
	}
	// Segunda fila, salary es 50000.
	if tbl.Rows[1][1].IsNull() {
		t.Error("expected non-NULL for salary=50000")
	}
}

func TestLoadReader_DeclaredTypes(t *testing.T) {
	csv := `id:int,name:string,active:bool
1,Alice,true
2,Bob,false
`
	cat := catalog.New()
	tbl, err := LoadReader(cat, strings.NewReader(csv), "t", DefaultOptions())
	if err != nil {
		t.Fatalf("LoadReader: %v", err)
	}
	if tbl.Schema.Columns[0].Kind != types.KindInt {
		t.Error("expected KindInt for id:int")
	}
	if tbl.Schema.Columns[2].Kind != types.KindBool {
		t.Error("expected KindBool for active:bool")
	}
}

func TestLoadReader_EmptyFile(t *testing.T) {
	cat := catalog.New()
	_, err := LoadReader(cat, strings.NewReader(""), "empty", DefaultOptions())
	if err == nil {
		t.Error("expected error for empty CSV, got nil")
	}
}

func TestLoadReader_MismatchedColumns(t *testing.T) {
	csv := `a,b,c
1,2
`
	cat := catalog.New()
	_, err := LoadReader(cat, strings.NewReader(csv), "bad", DefaultOptions())
	if err == nil {
		t.Error("expected error for mismatched column count, got nil")
	}
}

func TestLoadReader_BoolInference(t *testing.T) {
	csv := `id,active
1,true
2,false
`
	cat := catalog.New()
	tbl, err := LoadReader(cat, strings.NewReader(csv), "t", DefaultOptions())
	if err != nil {
		t.Fatalf("LoadReader: %v", err)
	}
	if tbl.Schema.Columns[1].Kind != types.KindBool {
		t.Errorf("expected KindBool for 'active', got %d", tbl.Schema.Columns[1].Kind)
	}
}
