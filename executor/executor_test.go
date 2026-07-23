package executor

import (
	"strings"
	"testing"

	"github.com/theloncho/sql-engine/catalog"
	"github.com/theloncho/sql-engine/loader"
	"github.com/theloncho/sql-engine/parser"
	"github.com/theloncho/sql-engine/types"
)

// helpers de test

func makeTestCatalog(t *testing.T) *catalog.Catalog {
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
	if _, err := loader.LoadReader(cat, strings.NewReader(empCSV), "employees", opts); err != nil {
		t.Fatalf("load employees: %v", err)
	}
	if _, err := loader.LoadReader(cat, strings.NewReader(deptCSV), "departments", opts); err != nil {
		t.Fatalf("load departments: %v", err)
	}
	return cat
}

func collectAll(t *testing.T, op Operator) []Row {
	t.Helper()
	var rows []Row
	for {
		row, err := op.Next()
		if err != nil {
			t.Fatalf("Next(): %v", err)
		}
		if row == nil {
			break
		}
		rows = append(rows, row)
	}
	return rows
}

// --- Tests de TableScan ---

func TestTableScan_Basic(t *testing.T) {
	cat := makeTestCatalog(t)
	tbl, _ := cat.Get("employees")
	scan := NewTableScan(tbl, "")
	rows := collectAll(t, scan)
	if len(rows) != 5 {
		t.Errorf("scan rows = %d, want 5", len(rows))
	}
}

func TestTableScan_Empty(t *testing.T) {
	tbl := &catalog.Table{
		Name:   "empty",
		Schema: catalog.Schema{Columns: []catalog.Column{{Name: "id", Kind: types.KindInt}}},
		Rows:   nil,
	}
	scan := NewTableScan(tbl, "")
	rows := collectAll(t, scan)
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}
}

// --- Tests de Filter ---

func TestFilter_WhereGt(t *testing.T) {
	cat := makeTestCatalog(t)
	tbl, _ := cat.Get("employees")
	scan := NewTableScan(tbl, "employees")

	pred, err := parser.Parse("SELECT * FROM employees WHERE salary > 80000")
	if err != nil {
		t.Fatal(err)
	}
	f := NewFilter(scan, pred.Where)
	rows := collectAll(t, f)
	// Alice 75000 NO, Bob 82000 SÍ, Carol 91000 SÍ, Dave NULL NO, Eve 60000 NO
	if len(rows) != 2 {
		t.Errorf("filter rows = %d, want 2", len(rows))
	}
}

func TestFilter_NullNotPassing(t *testing.T) {
	// Dave tiene salary NULL → no debe pasar ningún filtro de salary.
	cat := makeTestCatalog(t)
	tbl, _ := cat.Get("employees")
	scan := NewTableScan(tbl, "employees")
	pred, _ := parser.Parse("SELECT * FROM employees WHERE salary > 0")
	f := NewFilter(scan, pred.Where)
	rows := collectAll(t, f)
	for _, row := range rows {
		// El índice de salary es 3. Si es NULL, hay un bug.
		if row[3].IsNull() {
			t.Error("NULL salary should not pass salary > 0 filter")
		}
	}
}

// --- Tests de Project ---

func TestProject_Star(t *testing.T) {
	cat := makeTestCatalog(t)
	tbl, _ := cat.Get("employees")
	scan := NewTableScan(tbl, "employees")
	stmt, _ := parser.Parse("SELECT * FROM employees")
	proj, err := NewProject(scan, stmt.Cols)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectAll(t, proj)
	if len(rows) != 5 {
		t.Errorf("project * rows = %d, want 5", len(rows))
	}
}

func TestProject_SpecificCols(t *testing.T) {
	cat := makeTestCatalog(t)
	tbl, _ := cat.Get("employees")
	scan := NewTableScan(tbl, "employees")
	stmt, _ := parser.Parse("SELECT name, salary FROM employees")
	proj, err := NewProject(scan, stmt.Cols)
	if err != nil {
		t.Fatal(err)
	}
	schema := proj.Schema()
	if len(schema.Cols) != 2 {
		t.Errorf("schema cols = %d, want 2", len(schema.Cols))
	}
	if schema.Cols[0].Name != "name" {
		t.Errorf("col[0] = %q, want name", schema.Cols[0].Name)
	}
}

func TestProject_UnknownColumn(t *testing.T) {
	cat := makeTestCatalog(t)
	tbl, _ := cat.Get("employees")
	scan := NewTableScan(tbl, "employees")
	stmt, _ := parser.Parse("SELECT nonexistent FROM employees")
	_, err := NewProject(scan, stmt.Cols)
	if err == nil {
		t.Error("expected error for unknown column")
	}
}

// --- Tests de Sort ---

func TestSort_Asc(t *testing.T) {
	cat := makeTestCatalog(t)
	tbl, _ := cat.Get("employees")
	scan := NewTableScan(tbl, "employees")
	stmt, _ := parser.Parse("SELECT * FROM employees ORDER BY salary ASC")
	keys := []SortKey{{Expr: stmt.OrderBy[0].Expr, Asc: true}}
	sort := NewSort(scan, keys)
	rows := collectAll(t, sort)
	// NULL va al final; los demás en orden ascendente.
	// Eve(60000) < Alice(75000) < Bob(82000) < Carol(91000) < Dave(NULL)
	if len(rows) != 5 {
		t.Fatalf("sort rows = %d, want 5", len(rows))
	}
	// La última fila debe tener salary NULL (Dave)
	if !rows[4][3].IsNull() {
		t.Error("expected NULL salary last in ASC order")
	}
}

func TestSort_Desc(t *testing.T) {
	cat := makeTestCatalog(t)
	tbl, _ := cat.Get("employees")
	scan := NewTableScan(tbl, "employees")
	stmt, _ := parser.Parse("SELECT * FROM employees ORDER BY salary DESC")
	keys := []SortKey{{Expr: stmt.OrderBy[0].Expr, Asc: false}}
	sortOp := NewSort(scan, keys)
	rows := collectAll(t, sortOp)
	// Carol(91000) primero. NULL va al final en DESC también.
	if len(rows) != 5 {
		t.Fatalf("sort rows = %d, want 5", len(rows))
	}
	// Primera fila: Carol con salary 91000
	if rows[0][3].FVal != 91000.00 {
		t.Errorf("first row salary = %v, want 91000.00", rows[0][3])
	}
}

// --- Tests de Limit ---

func TestLimit_Basic(t *testing.T) {
	cat := makeTestCatalog(t)
	tbl, _ := cat.Get("employees")
	scan := NewTableScan(tbl, "employees")
	lim, _ := NewLimit(scan, 3)
	rows := collectAll(t, lim)
	if len(rows) != 3 {
		t.Errorf("limit rows = %d, want 3", len(rows))
	}
}

func TestLimit_MoreThanAvailable(t *testing.T) {
	cat := makeTestCatalog(t)
	tbl, _ := cat.Get("employees")
	scan := NewTableScan(tbl, "employees")
	lim, _ := NewLimit(scan, 100)
	rows := collectAll(t, lim)
	if len(rows) != 5 {
		t.Errorf("limit > rows: got %d, want 5", len(rows))
	}
}

func TestLimit_Zero(t *testing.T) {
	cat := makeTestCatalog(t)
	tbl, _ := cat.Get("employees")
	scan := NewTableScan(tbl, "employees")
	lim, _ := NewLimit(scan, 0)
	rows := collectAll(t, lim)
	if len(rows) != 0 {
		t.Errorf("limit 0: got %d, want 0", len(rows))
	}
}

// --- Tests de Aggregate ---

func TestAggregate_Count(t *testing.T) {
	cat := makeTestCatalog(t)
	tbl, _ := cat.Get("employees")
	scan := NewTableScan(tbl, "employees")
	agg := NewAggregate(scan, nil, []AggSpec{
		{Func: "COUNT", IsStar: true, Alias: "COUNT(*)"},
	})
	rows := collectAll(t, agg)
	if len(rows) != 1 {
		t.Fatalf("aggregate rows = %d, want 1", len(rows))
	}
	if rows[0][0].IVal != 5 {
		t.Errorf("COUNT(*) = %d, want 5", rows[0][0].IVal)
	}
}

func TestAggregate_GroupBy(t *testing.T) {
	cat := makeTestCatalog(t)
	tbl, _ := cat.Get("employees")
	scan := NewTableScan(tbl, "employees")
	stmt, _ := parser.Parse("SELECT dept_id, COUNT(*) FROM employees GROUP BY dept_id")
	agg := NewAggregate(scan, stmt.GroupBy, []AggSpec{
		{Func: "COUNT", IsStar: true, Alias: "COUNT(*)"},
	})
	rows := collectAll(t, agg)
	// dept 10 → 2 (Alice, Carol), dept 20 → 2 (Bob, Eve), dept 30 → 1 (Dave)
	if len(rows) != 3 {
		t.Errorf("group by dept rows = %d, want 3", len(rows))
	}
}

func TestAggregate_SumNullIgnored(t *testing.T) {
	cat := makeTestCatalog(t)
	tbl, _ := cat.Get("employees")
	scan := NewTableScan(tbl, "employees")
	stmt, _ := parser.Parse("SELECT SUM(salary) FROM employees")
	// Dave tiene NULL salary → debe ignorarse en SUM.
	agg := NewAggregate(scan, nil, []AggSpec{
		{Func: "SUM", Arg: stmt.Cols[0].Expr.(*parser.AggFunc).Arg, Alias: "SUM(salary)"},
	})
	rows := collectAll(t, agg)
	if len(rows) != 1 {
		t.Fatal("expected 1 row from SUM")
	}
	// 75000 + 82000.50 + 91000 + 60000 = 308000.50
	if rows[0][0].FVal != 308000.50 {
		t.Errorf("SUM(salary) = %v, want 308000.50", rows[0][0].FVal)
	}
}

func TestAggregate_EmptyTable(t *testing.T) {
	tbl := &catalog.Table{
		Name:   "empty",
		Schema: catalog.Schema{Columns: []catalog.Column{{Name: "v", Kind: types.KindInt}}},
	}
	scan := NewTableScan(tbl, "empty")
	agg := NewAggregate(scan, nil, []AggSpec{
		{Func: "COUNT", IsStar: true, Alias: "n"},
	})
	rows := collectAll(t, agg)
	if len(rows) != 1 || rows[0][0].IVal != 0 {
		t.Errorf("COUNT(*) on empty = %v, want 0", rows[0][0])
	}
}

// --- Tests de JOINs ---

func TestNestedLoopJoin(t *testing.T) {
	cat := makeTestCatalog(t)
	empTbl, _ := cat.Get("employees")
	deptTbl, _ := cat.Get("departments")

	outer := NewTableScan(empTbl, "e")
	inner := NewTableScan(deptTbl, "d")
	stmt, _ := parser.Parse("SELECT * FROM e INNER JOIN d ON e.dept_id = d.id")
	cond := stmt.Joins[0].On

	join := NewNestedLoopJoin(outer, inner, cond)
	rows := collectAll(t, join)
	// Dave tiene dept 30 que existe → todos los 5 empleados tienen dept válido.
	if len(rows) != 5 {
		t.Errorf("nested loop join rows = %d, want 5", len(rows))
	}
}

func TestHashJoin(t *testing.T) {
	cat := makeTestCatalog(t)
	empTbl, _ := cat.Get("employees")
	deptTbl, _ := cat.Get("departments")

	outer := NewTableScan(empTbl, "e")
	inner := NewTableScan(deptTbl, "d")

	// Claves: e.dept_id (idx=2) y d.id (idx=0)
	outerKey, _ := parser.Parse("SELECT e.dept_id FROM e")
	innerKey, _ := parser.Parse("SELECT d.id FROM d")

	join := NewHashJoin(outer, inner,
		outerKey.Cols[0].Expr,
		innerKey.Cols[0].Expr)
	rows := collectAll(t, join)
	if len(rows) != 5 {
		t.Errorf("hash join rows = %d, want 5", len(rows))
	}
}

// --- Tests de Distinct ---

func TestDistinct(t *testing.T) {
	cat := makeTestCatalog(t)
	tbl, _ := cat.Get("employees")
	scan := NewTableScan(tbl, "employees")
	stmt, _ := parser.Parse("SELECT dept_id FROM employees")
	proj, _ := NewProject(scan, stmt.Cols)
	dist := NewDistinct(proj)
	rows := collectAll(t, dist)
	// Hay 3 departamentos distintos: 10, 20, 30
	if len(rows) != 3 {
		t.Errorf("distinct dept_id = %d rows, want 3", len(rows))
	}
}

// --- Test de cadena completa (pipeline) ---

func TestFullPipeline_ScanFilterProjectSortLimit(t *testing.T) {
	cat := makeTestCatalog(t)
	tbl, _ := cat.Get("employees")

	// SELECT name FROM employees WHERE active = TRUE ORDER BY salary DESC LIMIT 2
	// Árbol correcto: Limit → Sort → Filter → Scan (sort antes de project para que salary esté disponible)
	scan := NewTableScan(tbl, "employees")

	pred, _ := parser.Parse("SELECT * FROM employees WHERE active = TRUE")
	filtered := NewFilter(scan, pred.Where)

	// Sort antes de Project (salary aún está en el schema de Filter).
	sortStmt, _ := parser.Parse("SELECT * FROM employees ORDER BY salary DESC")
	keys := []SortKey{{Expr: sortStmt.OrderBy[0].Expr, Asc: false}}
	sorted := NewSort(filtered, keys)

	lim, _ := NewLimit(sorted, 2)

	// Project al final.
	projStmt, _ := parser.Parse("SELECT name FROM employees")
	proj, err := NewProject(lim, projStmt.Cols)
	if err != nil {
		t.Fatal(err)
	}

	rows := collectAll(t, proj)
	if len(rows) != 2 {
		t.Errorf("pipeline rows = %d, want 2", len(rows))
	}
	// La primera fila debería ser el empleado activo con más salary.
	// Bob(82000.50 activo) y Dave(NULL activo): order DESC NULLS LAST → Bob primero.
	if rows[0][0].SVal != "Bob" {
		t.Errorf("first row = %q, want Bob (highest salary among active)", rows[0][0].SVal)
	}
}
