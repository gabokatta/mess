package catalog_test

import (
	"testing"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/fixture"
)

func TestCreateAndListCategories(t *testing.T) {
	db := fixture.DB(t)

	if _, err := catalog.CreateCategory(db, "Utilities", 1, 1); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	if _, err := catalog.CreateCategory(db, "Home", 0, 0); err != nil {
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

	if _, err := catalog.CreateCategory(db, "Home", 0, 0); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	if _, err := catalog.CreateCategory(db, "Home", 1, 1); err == nil {
		t.Error("CreateCategory() with a duplicate name should fail the UNIQUE constraint")
	}
}

// Appending puts a category at the end of the list with a colour nobody is
// using, which is everything the create form has to know.
func TestAppendCategoryGoesLastWithAFreeColour(t *testing.T) {
	db := fixture.DB(t)
	if _, err := catalog.CreateCategory(db, "Home", 0, 0); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}

	got, err := catalog.AppendCategory(db, "Utilities")
	if err != nil {
		t.Fatalf("AppendCategory() unexpected error: %v", err)
	}
	if got.Name != "Utilities" {
		t.Errorf("AppendCategory().Name = %q, want Utilities", got.Name)
	}
	if got.SortOrder != 1 {
		t.Errorf("AppendCategory().SortOrder = %d, want 1 (appended)", got.SortOrder)
	}
	if got.ColorIndex != 1 {
		t.Errorf("AppendCategory().ColorIndex = %d, want the lowest free slot 1", got.ColorIndex)
	}
	if got.ID == 0 {
		t.Error("AppendCategory() should assign a non-zero ID")
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
	if _, err := catalog.CreateCategory(db, "Custom", 0, 0); err != nil {
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

// The palette has eight slots and the schema says so, which is why paletteAt
// indexes rather than wrapping defensively.
func TestColorIndexIsPinnedToThePalette(t *testing.T) {
	db := fixture.DB(t)
	if _, err := db.Exec(`INSERT INTO category (name, sort_order, color_index) VALUES ('Bad', 0, 99)`); err == nil {
		t.Error("a colour index outside the palette should be refused by the schema")
	}
}
