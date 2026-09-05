package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/gabokatta/mess/internal/catalog"
)

// Okabe-Ito: distinguishable under common color blindness, on either ground.
// Its size is the catalog's PaletteSize, which is what a category's stored
// color_index is pinned to, so a category always names a real slot here.
var _ [catalog.PaletteSize]color.Color = palette

var palette = [8]color.Color{
	lipgloss.Color("#E69F00"),
	lipgloss.Color("#56B4E9"),
	lipgloss.Color("#009E73"),
	lipgloss.Color("#F0E442"),
	lipgloss.Color("#0072B2"),
	lipgloss.Color("#D55E00"),
	lipgloss.Color("#CC79A7"),
	lipgloss.Color("#999999"),
}

// Reads the category's own colour, so the order categories are listed in has
// nothing to do with the hue any of them renders in.
func categoryColor(categories []catalog.Category, categoryID int64) color.Color {
	for _, c := range categories {
		if c.ID == categoryID {
			return palette[c.ColorIndex]
		}
	}
	return palette[0]
}

func categoryStyle(categories []catalog.Category, categoryID int64) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(categoryColor(categories, categoryID))
}
