package tui

import "charm.land/lipgloss/v2"

type Theme struct {
	App       lipgloss.Style
	Title     lipgloss.Style
	Tab       lipgloss.Style
	TabActive lipgloss.Style
	Muted     lipgloss.Style
	Help      lipgloss.Style
	Rule      lipgloss.Style
}

func NewTheme(dark bool) Theme {
	accent := lipgloss.Color("#7D56F4")
	fg, muted, rule := lipgloss.Color("#1c1917"), lipgloss.Color("#78716c"), lipgloss.Color("#e7e5e4")
	if dark {
		fg, muted, rule = lipgloss.Color("#fafaf9"), lipgloss.Color("#a8a29e"), lipgloss.Color("#292524")
	}

	return Theme{
		App:       lipgloss.NewStyle().Padding(1, 2),
		Title:     lipgloss.NewStyle().Bold(true).Foreground(accent),
		Tab:       lipgloss.NewStyle().Foreground(muted).Padding(0, 1),
		TabActive: lipgloss.NewStyle().Foreground(fg).Bold(true).Underline(true).Padding(0, 1),
		Muted:     lipgloss.NewStyle().Foreground(muted),
		Help:      lipgloss.NewStyle().Foreground(muted),
		Rule:      lipgloss.NewStyle().Foreground(rule),
	}
}
