package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/gabokatta/mess/internal/catalog"
)

// categoryPalette is Okabe-Ito, chosen for staying distinguishable under the
// common forms of color blindness. One palette, not a light/dark pair — the
// hues read fine on both.
var categoryPalette = [8]color.Color{
	lipgloss.Color("#E69F00"),
	lipgloss.Color("#56B4E9"),
	lipgloss.Color("#009E73"),
	lipgloss.Color("#F0E442"),
	lipgloss.Color("#0072B2"),
	lipgloss.Color("#D55E00"),
	lipgloss.Color("#CC79A7"),
	lipgloss.Color("#000000"),
}

// categoryColor resolves categoryID's color from its position in categories,
// which must already be sorted the way catalog.Categories returns them —
// never stored, so the mapping can't drift out of sync with the category
// list itself. Past 8 categories, colors repeat. An ID not found in
// categories (data not loaded yet) falls back to the first color.
func categoryColor(categories []catalog.Category, categoryID int64) color.Color {
	for i, c := range categories {
		if c.ID == categoryID {
			return categoryPalette[i%len(categoryPalette)]
		}
	}
	return categoryPalette[0]
}

// categoryStyle is categoryColor as a Style, for callers that render text
// rather than a chart bar.
func categoryStyle(categories []catalog.Category, categoryID int64) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(categoryColor(categories, categoryID))
}
