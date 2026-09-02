package tui

import (
	"testing"

	"github.com/gabokatta/mess/internal/catalog"
)

func TestCategoryColorFollowsSortedPosition(t *testing.T) {
	categories := []catalog.Category{
		{ID: 10, SortOrder: 0},
		{ID: 20, SortOrder: 1},
		{ID: 30, SortOrder: 2},
	}

	if got, want := categoryColor(categories, 10), categoryPalette[0]; got != want {
		t.Errorf("categoryColor(10) = %v, want palette[0] %v", got, want)
	}
	if got, want := categoryColor(categories, 20), categoryPalette[1]; got != want {
		t.Errorf("categoryColor(20) = %v, want palette[1] %v", got, want)
	}
	if got, want := categoryColor(categories, 30), categoryPalette[2]; got != want {
		t.Errorf("categoryColor(30) = %v, want palette[2] %v", got, want)
	}
}

func TestCategoryColorWrapsPastEightCategories(t *testing.T) {
	categories := make([]catalog.Category, 9)
	for i := range categories {
		categories[i] = catalog.Category{ID: int64(i + 1), SortOrder: i}
	}

	ninth := categoryColor(categories, 9)
	first := categoryColor(categories, 1)
	if ninth != first {
		t.Errorf("categoryColor(9th) = %v, want it to repeat palette[0] = %v", ninth, first)
	}
}

func TestCategoryColorUnknownIDFallsBackToFirst(t *testing.T) {
	categories := []catalog.Category{{ID: 10, SortOrder: 0}, {ID: 20, SortOrder: 1}}
	if got, want := categoryColor(categories, 999), categoryPalette[0]; got != want {
		t.Errorf("categoryColor(unknown) = %v, want fallback palette[0] %v", got, want)
	}
}
