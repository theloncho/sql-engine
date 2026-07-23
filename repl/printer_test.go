package repl_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/theloncho/sql-engine/executor"
	"github.com/theloncho/sql-engine/repl"
	"github.com/theloncho/sql-engine/types"
)

func TestPrintTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	repl.PrintTable(&buf, nil, nil)
	if !strings.Contains(buf.String(), "(sin resultados)") {
		t.Errorf("expected '(sin resultados)', got %q", buf.String())
	}
}

func TestPrintTable_WithRows(t *testing.T) {
	var buf bytes.Buffer
	headers := []string{"id", "name", "salary"}
	rows := []executor.Row{
		{types.IntVal(1), types.StringVal("Alice"), types.FloatVal(75000.00)},
		{types.IntVal(2), types.StringVal("Bob"), types.Null()},
	}

	repl.PrintTable(&buf, headers, rows)
	output := buf.String()

	if !strings.Contains(output, "| id | name  | salary |") {
		t.Errorf("expected output to contain headers, got:\n%s", output)
	}
	if !strings.Contains(output, "| 1  | Alice | 75000  |") {
		t.Errorf("expected output to contain Alice's row, got:\n%s", output)
	}
	if !strings.Contains(output, "| 2  | Bob   | NULL   |") {
		t.Errorf("expected output to contain Bob's row with NULL, got:\n%s", output)
	}
}
