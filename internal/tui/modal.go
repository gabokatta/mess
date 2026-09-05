package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

type modal interface {
	Init() tea.Cmd
	Update(tea.Msg) (modal, tea.Cmd)
	View() string
	Help() string
}

// Keep enough width for huh's confirmation buttons below wrapped titles.
const formCardWidth = 56

type form struct {
	huh       *huh.Form
	theme     Theme
	maxHeight int
	help      string
	done      func() tea.Cmd
}

func newForm(theme Theme, width, height int, groups []*huh.Group, done func() tea.Cmd) *form {
	// Keys live in the app's help row, the same as on every screen, so the
	// card carries fields and nothing else.
	f := huh.NewForm(groups...).
		WithTheme(themeFor(theme)).
		WithShowHelp(false).
		WithWidth(min(formCardWidth, max(width-12, 20)))
	return &form{
		huh:       f,
		theme:     theme,
		maxHeight: max(height-12, 6),
		help:      "enter next · esc cancel",
		done:      done,
	}
}

func (f *form) Update(msg tea.Msg) (modal, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "esc" {
		return nil, nil
	}

	updated, cmd := f.huh.Update(msg)
	if next, ok := updated.(*huh.Form); ok {
		f.huh = next
	}

	switch f.huh.State {
	case huh.StateCompleted:
		return nil, tea.Batch(cmd, f.done())
	case huh.StateAborted:
		return nil, nil
	}
	return f, cmd
}

func (f *form) Init() tea.Cmd {
	cmd := f.huh.Init()
	if lipgloss.Height(f.huh.View()) > f.maxHeight {
		f.huh = f.huh.WithHeight(f.maxHeight)
	}
	return cmd
}

func (f *form) View() string { return f.theme.card(f.huh.View()) }
func (f *form) Help() string { return f.help }
