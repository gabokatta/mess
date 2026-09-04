package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/gabokatta/mess/internal/catalog"
)

// Okabe-Ito: distinguishable under common color blindness, on either ground.
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

// Indexes by position, so categories must arrive in catalog.Categories order.
func categoryColor(categories []catalog.Category, categoryID int64) color.Color {
	for i, c := range categories {
		if c.ID == categoryID {
			return palette[i%len(palette)]
		}
	}
	return palette[0]
}

func categoryStyle(categories []catalog.Category, categoryID int64) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(categoryColor(categories, categoryID))
}
