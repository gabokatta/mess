package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

type view int

const (
	viewMonth view = iota
	viewYear
	viewConcepts
	viewProjects
	viewSettings
)

var viewNames = [...]string{"Month", "Year", "Concepts", "Projects", "Settings"}

func (v view) String() string { return viewNames[v] }

type Model struct {
	theme  Theme
	view   view
	width  int
	height int
}

func New() Model {
	return Model{theme: NewTheme(true)}
}

func (m Model) Init() tea.Cmd {
	return tea.RequestBackgroundColor
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.theme = NewTheme(msg.IsDark())

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "l":
			m.view = (m.view + 1) % view(len(viewNames))
		case "shift+tab", "h":
			m.view = (m.view - 1 + view(len(viewNames))) % view(len(viewNames))
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	var b strings.Builder
	b.WriteString(m.theme.Title.Render("mes"))
	b.WriteString("  ")
	b.WriteString(m.tabs())
	b.WriteString("\n")
	b.WriteString(m.theme.Rule.Render(strings.Repeat("─", max(m.contentWidth(), 1))))
	b.WriteString("\n\n")
	b.WriteString(m.theme.Muted.Render(m.view.String() + " — not built yet"))
	b.WriteString("\n\n")
	b.WriteString(m.theme.Help.Render("tab/shift+tab switch · q quit"))

	v := tea.NewView(m.theme.App.Render(b.String()))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeNone
	v.WindowTitle = "mes"
	return v
}

func (m Model) tabs() string {
	labels := make([]string, len(viewNames))
	for i, name := range viewNames {
		style := m.theme.Tab
		if view(i) == m.view {
			style = m.theme.TabActive
		}
		labels[i] = style.Render(name)
	}
	return strings.Join(labels, "")
}

func (m Model) contentWidth() int {
	const framing = 4
	return m.width - framing
}
