package main_test

import (
	"testing"

	"github.com/theloncho/sql-engine/catalog"
	"github.com/theloncho/sql-engine/executor"
	"github.com/theloncho/sql-engine/loader"
	"github.com/theloncho/sql-engine/parser"
	"github.com/theloncho/sql-engine/planner"
)

func TestIntegration_FullPipeline(t *testing.T) {
	cat := catalog.New()
	opts := loader.DefaultOptions()

	_, err := loader.LoadFile(cat, "data/employees.csv", opts)
	if err != nil {
		t.Fatalf("failed to load employees.csv: %v", err)
	}

	_, err = loader.LoadFile(cat, "data/departments.csv", opts)
	if err != nil {
		t.Fatalf("failed to load departments.csv: %v", err)
	}

	query := "SELECT e.name, d.name FROM employees AS e INNER JOIN departments AS d ON e.dept_id = d.id WHERE e.salary > 70000 ORDER BY e.salary DESC"
	stmt, err := parser.Parse(query)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	plan, err := planner.Build(stmt, cat)
	if err != nil {
		t.Fatalf("planner failed: %v", err)
	}
	defer plan.Root.Close()

	var rows []executor.Row
	for {
		row, err := plan.Root.Next()
		if err != nil {
			t.Fatalf("execution error: %v", err)
		}
		if row == nil {
			break
		}
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		t.Errorf("expected non-empty result set, got 0 rows")
	}
}
