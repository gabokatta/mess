package tui

import (
	"database/sql"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/gabokatta/mes/internal/domain"
	"github.com/gabokatta/mes/internal/month"
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
	theme   Theme
	view    view
	width   int
	height  int
	db      *sql.DB
	period  domain.Period
	lines   []month.Line
	loadErr error
	cursor  int
	editing *editState
	saveErr error
}

func New(db *sql.DB) Model {
	return Model{theme: NewTheme(true), db: db, period: domain.PeriodFromTime(time.Now())}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tea.RequestBackgroundColor, loadMonth(m.db, m.period))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.theme = NewTheme(msg.IsDark())

	case monthLoadedMsg:
		m.lines, m.loadErr = msg.lines, msg.err

	case entrySavedMsg:
		m.saveErr = msg.err
		return m, loadMonth(m.db, m.period)

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.editing != nil {
		return m.updateEditing(msg)
	}
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "tab", "l":
		m.view = (m.view + 1) % view(len(viewNames))
	case "shift+tab", "h":
		m.view = (m.view - 1 + view(len(viewNames))) % view(len(viewNames))
	case "j", "down":
		if m.view == viewMonth {
			m.cursor = m.moveCursor(1)
		}
	case "k", "up":
		if m.view == viewMonth {
			m.cursor = m.moveCursor(-1)
		}
	case "space":
		if m.view == viewMonth {
			return m.toggleDone()
		}
	case "enter":
		if m.view == viewMonth {
			return m.startEdit()
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
	b.WriteString(m.viewContent())
	b.WriteString("\n\n")
	b.WriteString(m.theme.Help.Render(m.helpText()))

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

func (m Model) viewContent() string {
	if m.view != viewMonth {
		return m.theme.Muted.Render(m.view.String() + " — not built yet")
	}
	return m.renderMonth()
}

func (m Model) helpText() string {
	if m.editing != nil {
		return "enter confirm · esc cancel"
	}
	if m.view == viewMonth {
		return "j/k move · space tick · enter edit · tab/shift+tab switch · q quit"
	}
	return "tab/shift+tab switch · q quit"
}
