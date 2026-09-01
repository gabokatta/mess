package tui

import (
	"database/sql"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/gabokatta/mess/internal/dolarapi"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
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
	theme    Theme
	view     view
	width    int
	height   int
	db       *sql.DB
	fxClient *dolarapi.Client
	period   domain.Period
	lines    []month.Line
	chores   []month.ChoreLine
	loadErr  error
	cursor   int
	editing  *editState
	saveErr  error
	fxErr    error
	year     month.Year
	yearErr  error
}

func New(db *sql.DB) Model {
	return Model{
		theme:    NewTheme(true),
		db:       db,
		fxClient: dolarapi.NewClient(),
		period:   domain.PeriodFromTime(time.Now()),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.RequestBackgroundColor,
		loadMonth(m.db, m.period),
		fillCurrentFxRate(m.db, m.fxClient, m.period),
		loadYear(m.db, m.period.Year()),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.theme = NewTheme(msg.IsDark())

	case monthLoadedMsg:
		m.lines, m.chores, m.loadErr = msg.lines, msg.chores, msg.err

	case entrySavedMsg:
		m.saveErr = msg.err
		return m, loadMonth(m.db, m.period)

	case fxFilledMsg:
		m.fxErr = msg.err

	case yearLoadedMsg:
		m.year, m.yearErr = msg.year, msg.err

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

// Below this floor renderTooSmall takes over instead of a garbled layout.
const (
	minUsableWidth  = 40
	minUsableHeight = 10
)

func (m Model) View() tea.View {
	content := m.renderTooSmall()
	if content == "" {
		content = m.renderApp()
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeNone
	v.WindowTitle = "mess"
	return v
}

// renderTooSmall reports the "grow your terminal" message once a real,
// too-small size is known, or "" when the normal layout should render.
func (m Model) renderTooSmall() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	if m.width >= minUsableWidth && m.height >= minUsableHeight {
		return ""
	}
	msg := m.theme.Muted.Width(m.width).Align(lipgloss.Center).Render("make the terminal bigger to see your mess")
	return lipgloss.PlaceVertical(m.height, lipgloss.Center, msg)
}

func (m Model) renderApp() string {
	footer := m.renderFooter()
	footerRows := strings.Count(footer, "\n") + 1
	boxHeight := m.height - footerRows
	app := m.theme.App
	if m.width > 0 && boxHeight > 0 {
		app = app.Width(m.width).Height(boxHeight)
	}
	rendered := app.Render(m.viewContent())
	if m.width >= logoMinWidth && m.height >= logoMinHeight {
		rendered = overlayLogo(rendered, m.theme.Logo)
	}

	return rendered + "\n" + footer
}

// renderFooter is the strip below the box: key legend left, tabs right.
// They share one row when both fit, else the tabs get a row of their own,
// still right-aligned.
func (m Model) renderFooter() string {
	left := "  " + m.theme.Help.Render(m.helpText())
	tabs := m.tabs()
	if lipgloss.Width(left)+lipgloss.Width(tabs) >= m.width {
		return left + "\n" + lipgloss.NewStyle().Width(m.width).Align(lipgloss.Right).Render(tabs)
	}
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(tabs)
	return left + strings.Repeat(" ", gap) + tabs
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

func (m Model) viewContent() string {
	switch m.view {
	case viewMonth:
		return m.renderMonth()
	case viewYear:
		return m.renderYear()
	default:
		return m.theme.Muted.Render(m.view.String() + " — not built yet")
	}
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
