package tui

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

type Theme struct {
	Dark      bool
	App       lipgloss.Style
	Title     lipgloss.Style
	Tab       lipgloss.Style
	TabActive lipgloss.Style
	Muted     lipgloss.Style
	Help      lipgloss.Style
	Logo      lipgloss.Style
	Chart     lipgloss.Style
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
		App:       lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()),
		Title:     lipgloss.NewStyle().Bold(true).Foreground(accent),
		Tab:       lipgloss.NewStyle().Foreground(muted).Padding(0, 1),
		TabActive: lipgloss.NewStyle().Foreground(fg).Bold(true).Underline(true).Padding(0, 1),
		Muted:     lipgloss.NewStyle().Foreground(muted),
		Help:      lipgloss.NewStyle().Foreground(muted),
		Logo:      lipgloss.NewStyle().Foreground(muted),
		Chart:     lipgloss.NewStyle().Foreground(accent),
	}
}

// themeFor resolves Huh's own theme once, at form-construction time, rather
// than forwarding tea.BackgroundColorMsg into every open form — by the time
// a user can open one, t.Dark is already known.
func themeFor(t Theme) huh.Theme {
	return huh.ThemeFunc(func(bool) *huh.Styles { return huh.ThemeCharm(t.Dark) })
}

// formHeight is a form's vertical budget inside the app's box. Left unset,
// a Group sizes itself to fit every field at once, so a form taller than
// the terminal just runs off the bottom instead of scrolling.
func formHeight(appHeight int) int {
	if h := appHeight - 10; h > 6 {
		return h
	}
	return 6
}
