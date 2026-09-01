package catalog

import (
	"path/filepath"
	"testing"

	"github.com/gabokatta/mes/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "mes.db"))
	if err != nil {
		t.Fatalf("store.Open() unexpected error: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateAndListCategories(t *testing.T) {
	db := openTestStore(t).DB()

	if _, err := CreateCategory(db, "Servicios", 1); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	if _, err := CreateCategory(db, "Hogar", 0); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}

	got, err := Categories(db)
	if err != nil {
		t.Fatalf("Categories() unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("Categories() returned %d rows, want 2", len(got))
	}
	if got[0].Name != "Hogar" || got[1].Name != "Servicios" {
		t.Errorf("Categories() = %+v, want Hogar (sort_order 0) before Servicios (sort_order 1)", got)
	}
	if got[0].ID == 0 {
		t.Error("CreateCategory() should assign a non-zero ID")
	}
}

func TestCreateCategoryDuplicateNameFails(t *testing.T) {
	db := openTestStore(t).DB()

	if _, err := CreateCategory(db, "Hogar", 0); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	if _, err := CreateCategory(db, "Hogar", 1); err == nil {
		t.Error("CreateCategory() with a duplicate name should fail the UNIQUE constraint")
	}
}

func TestUpdateCategory(t *testing.T) {
	db := openTestStore(t).DB()

	cat, err := CreateCategory(db, "Hogar", 0)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}

	cat.Name = "Casa"
	cat.SortOrder = 5
	if err := UpdateCategory(db, cat); err != nil {
		t.Fatalf("UpdateCategory() unexpected error: %v", err)
	}

	got, err := Categories(db)
	if err != nil {
		t.Fatalf("Categories() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Casa" || got[0].SortOrder != 5 {
		t.Errorf("Categories() = %+v, want a single Casa row with sort_order 5", got)
	}
}
