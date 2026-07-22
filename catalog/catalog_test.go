package catalog

import (
	"testing"

	"github.com/theloncho/sql-engine/types"
)

func makeTable(name string, cols []Column, rows [][]types.Value) *Table {
	return &Table{Name: name, Schema: Schema{Columns: cols}, Rows: rows}
}

func TestCatalogRegisterAndGet(t *testing.T) {
	cat := New()
	tbl := makeTable("users", []Column{
		{Name: "id", Kind: types.KindInt},
		{Name: "name", Kind: types.KindString},
	}, nil)

	if err := cat.Register(tbl); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, err := cat.Get("users")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "users" {
		t.Errorf("got table name %q, want %q", got.Name, "users")
	}
}

func TestCatalogDuplicateRegister(t *testing.T) {
	cat := New()
	tbl := makeTable("x", []Column{{Name: "a", Kind: types.KindInt}}, nil)
	if err := cat.Register(tbl); err != nil {
		t.Fatal(err)
	}
	if err := cat.Register(tbl); err == nil {
		t.Error("expected error on duplicate register, got nil")
	}
}

func TestCatalogGetMissing(t *testing.T) {
	cat := New()
	if _, err := cat.Get("nonexistent"); err == nil {
		t.Error("expected error for missing table, got nil")
	}
}

func TestSchemaIndexOf(t *testing.T) {
	s := Schema{Columns: []Column{
		{Name: "id", Kind: types.KindInt},
		{Name: "name", Kind: types.KindString},
		{Name: "active", Kind: types.KindBool},
	}}
	if idx := s.IndexOf("name"); idx != 1 {
		t.Errorf("IndexOf(name) = %d, want 1", idx)
	}
	if idx := s.IndexOf("missing"); idx != -1 {
		t.Errorf("IndexOf(missing) = %d, want -1", idx)
	}
}
