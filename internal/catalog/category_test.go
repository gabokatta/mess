package catalog_test

import (
	"testing"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/fixture"
)

func TestCreateAndListCategories(t *testing.T) {
	db := fixture.DB(t)

	if _, err := catalog.CreateCategory(db, "Utilities", 1); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	if _, err := catalog.CreateCategory(db, "Home", 0); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}

	got, err := catalog.Categories(db)
	if err != nil {
		t.Fatalf("Categories() unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("Categories() returned %d rows, want 2", len(got))
	}
	if got[0].Name != "Home" {
		t.Errorf("Categories()[0].Name = %q, want Home (sort_order 0)", got[0].Name)
	}
	if got[1].Name != "Utilities" {
		t.Errorf("Categories()[1].Name = %q, want Utilities (sort_order 1)", got[1].Name)
	}
	if got[0].ID == 0 {
		t.Error("CreateCategory() should assign a non-zero ID")
	}
}

func TestCreateCategoryDuplicateNameFails(t *testing.T) {
	db := fixture.DB(t)

	if _, err := catalog.CreateCategory(db, "Home", 0); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	if _, err := catalog.CreateCategory(db, "Home", 1); err == nil {
		t.Error("CreateCategory() with a duplicate name should fail the UNIQUE constraint")
	}
}

func TestFindOrCreateCategoryReturnsExistingByName(t *testing.T) {
	db := fixture.DB(t)
	want, err := catalog.CreateCategory(db, "Home", 0)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}

	got, err := catalog.FindOrCreateCategory(db, "Home")
	if err != nil {
		t.Fatalf("FindOrCreateCategory() unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("FindOrCreateCategory() = %+v, want the existing %+v", got, want)
	}

	all, err := catalog.Categories(db)
	if err != nil {
		t.Fatalf("Categories() unexpected error: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("Categories() returned %d rows, want 1 (no duplicate created)", len(all))
	}
}

func TestFindOrCreateCategoryCreatesWhenMissing(t *testing.T) {
	db := fixture.DB(t)
	if _, err := catalog.CreateCategory(db, "Home", 0); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}

	got, err := catalog.FindOrCreateCategory(db, "Utilities")
	if err != nil {
		t.Fatalf("FindOrCreateCategory() unexpected error: %v", err)
	}
	if got.Name != "Utilities" {
		t.Errorf("FindOrCreateCategory().Name = %q, want Utilities", got.Name)
	}
	if got.SortOrder != 1 {
		t.Errorf("FindOrCreateCategory().SortOrder = %d, want 1 (appended)", got.SortOrder)
	}
	if got.ID == 0 {
		t.Error("FindOrCreateCategory() should assign a non-zero ID")
	}
}

func TestEnsureDefaultCategoriesSeedsAnEmptyTable(t *testing.T) {
	db := fixture.DB(t)

	if err := catalog.EnsureDefaultCategories(db); err != nil {
		t.Fatalf("EnsureDefaultCategories() unexpected error: %v", err)
	}

	got, err := catalog.Categories(db)
	if err != nil {
		t.Fatalf("Categories() unexpected error: %v", err)
	}
	if len(got) != len(catalog.DefaultCategoryNames) {
		t.Fatalf("Categories() returned %d rows, want %d", len(got), len(catalog.DefaultCategoryNames))
	}
	for i, name := range catalog.DefaultCategoryNames {
		if got[i].Name != name {
			t.Errorf("Categories()[%d].Name = %q, want %q", i, got[i].Name, name)
		}
	}
}

func TestEnsureDefaultCategoriesLeavesAnExistingTableAlone(t *testing.T) {
	db := fixture.DB(t)
	if _, err := catalog.CreateCategory(db, "Custom", 0); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}

	if err := catalog.EnsureDefaultCategories(db); err != nil {
		t.Fatalf("EnsureDefaultCategories() unexpected error: %v", err)
	}

	got, err := catalog.Categories(db)
	if err != nil {
		t.Fatalf("Categories() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Custom" {
		t.Errorf("Categories() = %+v, want only the pre-existing Custom row", got)
	}
}
