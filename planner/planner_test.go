package planner

import (
	"strings"
	"testing"

	"github.com/theloncho/sql-engine/catalog"
	"github.com/theloncho/sql-engine/executor"
	"github.com/theloncho/sql-engine/loader"
	"github.com/theloncho/sql-engine/parser"
)

func makeTestCat(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat := catalog.New()
	empCSV := `id,name,dept_id,salary,active
1,Alice,10,75000.00,true
2,Bob,20,82000.50,true
3,Carol,10,91000.00,false
4,Dave,30,,true
5,Eve,20,60000.00,true`
	deptCSV := `id,name,budget
10,Engineering,500000
20,Marketing,200000
30,Sales,150000`
	opts := loader.DefaultOptions()
	loader.LoadReader(cat, strings.NewReader(empCSV), "employees", opts)
	loader.LoadReader(cat, strings.NewReader(deptCSV), "departments", opts)
	return cat
}

func planAndRun(t *testing.T, sql string, cat *catalog.Catalog) []executor.Row {
	t.Helper()
	stmt, err := parser.Parse(sql)
	if err != nil {
		t.Fatalf("parse(%q): %v", sql, err)
	}
	plan, err := Build(stmt, cat)
	if err != nil {
		t.Fatalf("build(%q): %v", sql, err)
	}
	defer plan.Root.Close()
	var rows []executor.Row
	for {
		row, err := plan.Root.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if row == nil {
			break
		}
		rows = append(rows, row)
	}
	return rows
}

func TestPlanner_SelectStar(t *testing.T) {
	cat := makeTestCat(t)
	rows := planAndRun(t, "SELECT * FROM employees", cat)
	if len(rows) != 5 {
		t.Errorf("rows = %d, want 5", len(rows))
	}
}

func TestPlanner_WhereFilter(t *testing.T) {
	cat := makeTestCat(t)
	rows := planAndRun(t, "SELECT name FROM employees WHERE salary > 80000", cat)
	// Bob (82000.50) y Carol (91000)
	if len(rows) != 2 {
		t.Errorf("rows = %d, want 2", len(rows))
	}
}

func TestPlanner_OrderByLimit(t *testing.T) {
	cat := makeTestCat(t)
	rows := planAndRun(t, "SELECT name FROM employees ORDER BY salary DESC LIMIT 2", cat)
	if len(rows) != 2 {
		t.Errorf("rows = %d, want 2", len(rows))
	}
}

func TestPlanner_CountStar(t *testing.T) {
	cat := makeTestCat(t)
	rows := planAndRun(t, "SELECT COUNT(*) FROM employees", cat)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0][0].IVal != 5 {
		t.Errorf("COUNT(*) = %d, want 5", rows[0][0].IVal)
	}
}

func TestPlanner_GroupBy(t *testing.T) {
	cat := makeTestCat(t)
	rows := planAndRun(t, "SELECT dept_id, COUNT(*) FROM employees GROUP BY dept_id", cat)
	if len(rows) != 3 {
		t.Errorf("groups = %d, want 3", len(rows))
	}
}

func TestPlanner_Join(t *testing.T) {
	cat := makeTestCat(t)
	rows := planAndRun(t, "SELECT e.name, d.name FROM employees AS e INNER JOIN departments AS d ON e.dept_id = d.id", cat)
	if len(rows) != 5 {
		t.Errorf("join rows = %d, want 5", len(rows))
	}
}

func TestPlanner_Distinct(t *testing.T) {
	cat := makeTestCat(t)
	rows := planAndRun(t, "SELECT DISTINCT dept_id FROM employees", cat)
	if len(rows) != 3 {
		t.Errorf("distinct dept = %d, want 3", len(rows))
	}
}

func TestPlanner_UnknownTable(t *testing.T) {
	cat := makeTestCat(t)
	stmt, _ := parser.Parse("SELECT * FROM nonexistent")
	_, err := Build(stmt, cat)
	if err == nil {
		t.Error("expected error for unknown table")
	}
}
