package tui

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

type Theme struct {
	Dark   bool
	App    lipgloss.Style
	Title  lipgloss.Style
	Accent lipgloss.Style
	Bright lipgloss.Style
	Muted  lipgloss.Style
	Help   lipgloss.Style
	Tab    lipgloss.Style
	Active lipgloss.Style
	Logo   lipgloss.Style
}

func NewTheme(dark bool) Theme {
	accent := lipgloss.Color("#7D56F4")
	fg, muted := lipgloss.Color("#1c1917"), lipgloss.Color("#78716c")
	if dark {
		fg, muted = lipgloss.Color("#fafaf9"), lipgloss.Color("#a8a29e")
	}

	return Theme{
		Dark: dark,
		// Uncolored: overlayLogo slices these rows by rune count, and a
		// color code would throw that count off.
		App:    lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()),
		Title:  lipgloss.NewStyle().Bold(true),
		Accent: lipgloss.NewStyle().Foreground(accent).Bold(true),
		Bright: lipgloss.NewStyle().Foreground(fg),
		Muted:  lipgloss.NewStyle().Foreground(muted),
		Help:   lipgloss.NewStyle().Foreground(muted),
		Tab:    lipgloss.NewStyle().Foreground(muted).Padding(0, 1),
		Active: lipgloss.NewStyle().Foreground(fg).Bold(true).Underline(true).Padding(0, 1),
		Logo:   lipgloss.NewStyle().Foreground(muted),
	}
}

// themeFor resolves Huh's theme at form-construction time rather than
// forwarding tea.BackgroundColorMsg into every open form.
func themeFor(t Theme) huh.Theme {
	return huh.ThemeFunc(func(bool) *huh.Styles { return huh.ThemeCharm(t.Dark) })
}
