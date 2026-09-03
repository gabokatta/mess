package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/gabokatta/mess/internal/catalog"
)

// palette is Okabe-Ito, which stays distinguishable under the common forms
// of color blindness and reads on both light and dark grounds.
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

// categoryColor indexes by position, so categories must arrive sorted the
// way catalog.Categories returns them. Past eight, colors repeat.
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

// groupStyle keeps the accent free for the one thing per screen that earns
// it.
func groupStyle(index int) lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(palette[index%len(palette)])
}
