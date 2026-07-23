package explain_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/theloncho/sql-engine/catalog"
	"github.com/theloncho/sql-engine/executor"
	"github.com/theloncho/sql-engine/explain"
	"github.com/theloncho/sql-engine/types"
)

func TestExplainPrint_Nil(t *testing.T) {
	var buf bytes.Buffer
	explain.Print(nil, &buf)
	if buf.String() != "" {
		t.Errorf("expected empty output for nil operator, got %q", buf.String())
	}
}

func TestExplainPrint_TableScan(t *testing.T) {
	tbl := &catalog.Table{
		Name: "users",
		Schema: catalog.Schema{
			Columns: []catalog.Column{
				{Name: "id", Kind: types.KindInt},
				{Name: "name", Kind: types.KindString},
			},
		},
	}
	scan := executor.NewTableScan(tbl, "users")

	var buf bytes.Buffer
	explain.Print(scan, &buf)

	output := buf.String()
	if !strings.Contains(output, "TableScan") {
		t.Errorf("expected output to contain TableScan, got %q", output)
	}
	if !strings.Contains(output, "Output: [users.id, users.name]") {
		t.Errorf("expected output to contain column schema, got %q", output)
	}
}
