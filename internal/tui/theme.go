package tui

import "charm.land/lipgloss/v2"

type Theme struct {
	App       lipgloss.Style
	Title     lipgloss.Style
	Tab       lipgloss.Style
	TabActive lipgloss.Style
	Muted     lipgloss.Style
	Help      lipgloss.Style
	Logo      lipgloss.Style
}

func NewTheme(dark bool) Theme {
	accent := lipgloss.Color("#7D56F4")
	fg, muted := lipgloss.Color("#1c1917"), lipgloss.Color("#78716c")
	if dark {
		fg, muted = lipgloss.Color("#fafaf9"), lipgloss.Color("#a8a29e")
	}

	return Theme{
		// Uncolored: overlayLogo slices these rows by rune count, and a
		// color code would throw that count off.
		App:       lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()),
		Title:     lipgloss.NewStyle().Bold(true).Foreground(accent),
		Tab:       lipgloss.NewStyle().Foreground(muted).Padding(0, 1),
		TabActive: lipgloss.NewStyle().Foreground(fg).Bold(true).Underline(true).Padding(0, 1),
		Muted:     lipgloss.NewStyle().Foreground(muted),
		Help:      lipgloss.NewStyle().Foreground(muted),
		Logo:      lipgloss.NewStyle().Foreground(muted),
	}
}
