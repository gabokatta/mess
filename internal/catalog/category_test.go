package catalog

import (
	"path/filepath"
	"testing"

	"github.com/gabokatta/mess/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "mess.db"))
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

func TestFindOrCreateCategoryReturnsExistingByName(t *testing.T) {
	db := openTestStore(t).DB()
	want, err := CreateCategory(db, "Hogar", 0)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}

	got, err := FindOrCreateCategory(db, "Hogar")
	if err != nil {
		t.Fatalf("FindOrCreateCategory() unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("FindOrCreateCategory() = %+v, want the existing %+v", got, want)
	}

	all, err := Categories(db)
	if err != nil {
		t.Fatalf("Categories() unexpected error: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("Categories() returned %d rows, want 1 (no duplicate created)", len(all))
	}
}

func TestFindOrCreateCategoryCreatesWhenMissing(t *testing.T) {
	db := openTestStore(t).DB()
	if _, err := CreateCategory(db, "Hogar", 0); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}

	got, err := FindOrCreateCategory(db, "Servicios")
	if err != nil {
		t.Fatalf("FindOrCreateCategory() unexpected error: %v", err)
	}
	if got.Name != "Servicios" || got.SortOrder != 1 {
		t.Errorf("FindOrCreateCategory() = %+v, want Servicios appended at sort_order 1", got)
	}
	if got.ID == 0 {
		t.Error("FindOrCreateCategory() should assign a non-zero ID")
	}
}

func TestEnsureDefaultCategoriesSeedsAnEmptyTable(t *testing.T) {
	db := openTestStore(t).DB()

	if err := EnsureDefaultCategories(db); err != nil {
		t.Fatalf("EnsureDefaultCategories() unexpected error: %v", err)
	}

	got, err := Categories(db)
	if err != nil {
		t.Fatalf("Categories() unexpected error: %v", err)
	}
	if len(got) != len(DefaultCategoryNames) {
		t.Fatalf("Categories() returned %d rows, want %d", len(got), len(DefaultCategoryNames))
	}
	for i, name := range DefaultCategoryNames {
		if got[i].Name != name {
			t.Errorf("Categories()[%d].Name = %q, want %q", i, got[i].Name, name)
		}
	}
}

func TestEnsureDefaultCategoriesLeavesAnExistingTableAlone(t *testing.T) {
	db := openTestStore(t).DB()
	if _, err := CreateCategory(db, "Custom", 0); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}

	if err := EnsureDefaultCategories(db); err != nil {
		t.Fatalf("EnsureDefaultCategories() unexpected error: %v", err)
	}

	got, err := Categories(db)
	if err != nil {
		t.Fatalf("Categories() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Custom" {
		t.Errorf("Categories() = %+v, want only the pre-existing Custom row", got)
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
