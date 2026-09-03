package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
)

type modal interface {
	Init() tea.Cmd
	Update(tea.Msg) (modal, tea.Cmd)
	View() string
	Help() string
}

type form struct {
	huh  *huh.Form
	help string
	done func() tea.Cmd
}

func newForm(theme Theme, width, height int, groups []*huh.Group, done func() tea.Cmd) *form {
	f := huh.NewForm(groups...).
		WithTheme(themeFor(theme)).
		WithWidth(max(width-6, 20)).
		WithHeight(max(height-10, 6))
	return &form{huh: f, help: "enter next · esc cancel", done: done}
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

func (f *form) Init() tea.Cmd { return f.huh.Init() }
func (f *form) View() string  { return f.huh.View() }
func (f *form) Help() string  { return f.help }
