package tui

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/gabokatta/mes/internal/catalog"
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
}

func New(db *sql.DB) Model {
	return Model{theme: NewTheme(true), db: db, period: domain.PeriodFromTime(time.Now())}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tea.RequestBackgroundColor, loadMonth(m.db, m.period))
}

// monthLoadedMsg is the result of loadMonth's Cmd, delivered back to Update
// once the database read completes.
type monthLoadedMsg struct {
	lines []month.Line
	err   error
}

// loadMonth returns a Cmd that resolves period's lines off the Update loop.
func loadMonth(db *sql.DB, period domain.Period) tea.Cmd {
	return func() tea.Msg {
		lines, err := month.Load(db, period)
		return monthLoadedMsg{lines: lines, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.theme = NewTheme(msg.IsDark())

	case monthLoadedMsg:
		m.lines, m.loadErr = msg.lines, msg.err

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
	b.WriteString(m.viewContent())
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

func (m Model) viewContent() string {
	if m.view != viewMonth {
		return m.theme.Muted.Render(m.view.String() + " — not built yet")
	}
	return m.renderMonth()
}

// monthGroups is the display order for the month view's sections.
var monthGroups = [...]catalog.ConceptKind{catalog.Income, catalog.FixedExpense, catalog.VariableExpense}

func (m Model) renderMonth() string {
	var b strings.Builder
	b.WriteString(m.theme.Muted.Render(m.view.String() + " · " + m.period.String()))

	if m.loadErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("failed to load: " + m.loadErr.Error()))
		return b.String()
	}
	if len(m.lines) == 0 {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("no concepts yet — add some in the Concepts view"))
		return b.String()
	}

	for _, kind := range monthGroups {
		group := linesForKind(m.lines, kind)
		if len(group) == 0 {
			continue
		}
		b.WriteString("\n\n")
		b.WriteString(m.theme.Title.Render(kind.String()))
		for _, l := range group {
			b.WriteString("\n")
			b.WriteString(m.renderLine(l))
		}
	}
	return b.String()
}

func linesForKind(lines []month.Line, kind catalog.ConceptKind) []month.Line {
	var out []month.Line
	for _, l := range lines {
		if l.Concept.Kind == kind {
			out = append(out, l)
		}
	}
	return out
}

func (m Model) renderLine(l month.Line) string {
	check := " "
	if l.Done {
		check = "x"
	}
	status := "confirmed"
	if !l.Confirmed {
		status = m.theme.Muted.Render("projected")
	}
	return fmt.Sprintf("  [%s] %-20s %s %12s  %s", check, l.Concept.Name, l.Concept.Currency,
		l.Amount.StringFixed(2), status)
}
