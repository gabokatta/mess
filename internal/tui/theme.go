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
	Alert  lipgloss.Style
	Help   lipgloss.Style
	Tab    lipgloss.Style
	Active lipgloss.Style
	Logo   lipgloss.Style
}

// buttonText is the ink on a focused confirm button. The accent behind it is
// the same saturated purple on either ground, so the text does not follow the
// theme the way the rest of the card does.
var buttonText = lipgloss.Color("#fafaf9")

func NewTheme(dark bool) Theme {
	accent := lipgloss.Color("#7D56F4")
	fg, muted := lipgloss.Color("#1c1917"), lipgloss.Color("#78716c")
	if dark {
		fg, muted = lipgloss.Color("#fafaf9"), lipgloss.Color("#a8a29e")
	}

	return Theme{
		Dark:   dark,
		App:    lipgloss.NewStyle().Padding(1, 2, 0, 2).Border(lipgloss.RoundedBorder()),
		Title:  lipgloss.NewStyle().Bold(true),
		Accent: lipgloss.NewStyle().Foreground(accent).Bold(true),
		Bright: lipgloss.NewStyle().Foreground(fg),
		Muted:  lipgloss.NewStyle().Foreground(muted),
		Alert:  lipgloss.NewStyle().Foreground(palette[5]),
		Help:   lipgloss.NewStyle().Foreground(muted),
		Tab:    lipgloss.NewStyle().Foreground(muted).Padding(0, 1),
		Active: lipgloss.NewStyle().Foreground(fg).Bold(true).Underline(true).Padding(0, 1),
		Logo:   lipgloss.NewStyle().Foreground(muted),
	}
}

// card is the frame every modal that sizes itself to its own content draws
// inside, so a form and the category list read as the same kind of object.
func (t Theme) card(content string) string {
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).Render(content)
}

// themeFor maps the app's four colours onto huh, so a form obeys the same
// rules as every screen: Accent is the cursor and the choice it sits on,
// foreground is content, Muted is the text about the content, and Alert is the
// thing worth acting on.
//
// It starts from ThemeBase rather than ThemeCharm because Charm paints an
// unselected option in ANSI 235, which is near-black and vanishes against any
// background that is not close to white.
//
// Every field is recoloured rather than replaced: huh carries a style's glyph
// inside it, so "> " on a selector and the padding around a button are lost
// with the style they came in.
func themeFor(t Theme) huh.Theme {
	return huh.ThemeFunc(func(bool) *huh.Styles {
		s := huh.ThemeBase(t.Dark)
		fg, muted := t.Bright.GetForeground(), t.Muted.GetForeground()
		accent, alert := t.Accent.GetForeground(), t.Alert.GetForeground()

		for _, f := range []*huh.FieldStyles{&s.Focused, &s.Blurred} {
			f.Description = f.Description.Foreground(muted)
			f.Option = f.Option.Foreground(fg)
			f.UnselectedOption = f.UnselectedOption.Foreground(fg)
			f.UnselectedPrefix = f.UnselectedPrefix.Foreground(muted)
			f.ErrorIndicator = f.ErrorIndicator.Foreground(alert)
			f.ErrorMessage = f.ErrorMessage.Foreground(alert)
			f.TextInput.Text = f.TextInput.Text.Foreground(fg)
			f.TextInput.Placeholder = f.TextInput.Placeholder.Foreground(muted)
		}

		// The focused field carries the accent, which is what the cursor
		// gutter carries on every list.
		s.Focused.Title = s.Focused.Title.Foreground(fg).Bold(true)
		s.Focused.Base = s.Focused.Base.BorderForeground(accent)
		s.Focused.SelectSelector = s.Focused.SelectSelector.Foreground(accent)
		s.Focused.MultiSelectSelector = s.Focused.MultiSelectSelector.Foreground(accent)
		s.Focused.SelectedOption = s.Focused.SelectedOption.Foreground(accent)
		s.Focused.SelectedPrefix = s.Focused.SelectedPrefix.Foreground(accent)
		s.Focused.TextInput.Prompt = s.Focused.TextInput.Prompt.Foreground(accent)
		s.Focused.TextInput.Cursor = s.Focused.TextInput.Cursor.Foreground(accent)
		s.Focused.FocusedButton = s.Focused.FocusedButton.Foreground(buttonText).Background(accent)
		s.Focused.BlurredButton = s.Focused.BlurredButton.Foreground(muted).Background(nil)

		// A field nobody is on says so by dropping to muted everywhere, so
		// there is one accent on the card at a time.
		s.Blurred.Title = s.Blurred.Title.Foreground(muted).Bold(true)
		s.Blurred.Base = s.Blurred.Base.BorderForeground(muted)
		s.Blurred.SelectSelector = s.Blurred.SelectSelector.Foreground(muted)
		s.Blurred.MultiSelectSelector = s.Blurred.MultiSelectSelector.Foreground(muted)
		s.Blurred.SelectedOption = s.Blurred.SelectedOption.Foreground(muted)
		s.Blurred.SelectedPrefix = s.Blurred.SelectedPrefix.Foreground(muted)
		s.Blurred.TextInput.Prompt = s.Blurred.TextInput.Prompt.Foreground(muted)
		s.Blurred.FocusedButton = s.Blurred.FocusedButton.Foreground(muted).Background(nil)
		s.Blurred.BlurredButton = s.Blurred.BlurredButton.Foreground(muted).Background(nil)
		return s
	})
}
