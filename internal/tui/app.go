package tui

import (
	"database/sql"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
	"github.com/gabokatta/mess/internal/note"
	"github.com/gabokatta/mess/internal/rates"
)

type view int

const (
	viewMonth view = iota
	viewYear
	viewNotes
	viewConcepts
	viewRates
)

var viewNames = [...]string{"Month", "Year", "Notes", "Concepts", "Rates"}

func (v view) String() string { return viewNames[v] }

type Model struct {
	theme  Theme
	view   view
	width  int
	height int

	db     *sql.DB
	client *rates.Client

	// today is read once, at start-up: which month is still running decides
	// every period's status and whether its rate is live or closed.
	today  domain.Period
	period domain.Period

	settings catalog.Settings
	lines    []month.Line
	year     month.Year
	notes    []catalog.Note

	concepts   []catalog.Concept
	categories []catalog.Category

	stored []catalog.FxRate
	quotes []rates.Quote
	house  int

	monthList    scroller
	notesList    scroller
	conceptsList scroller
	detail       scroller
	openNote     *catalog.Note

	modal   modal
	lastErr error
}

func New(db *sql.DB) Model {
	now := domain.PeriodFromTime(time.Now())
	return Model{
		theme:  NewTheme(true),
		db:     db,
		client: rates.NewClient(),
		today:  now,
		period: now,
	}
}

// Init reads through the savedMsg that seeding reports, so the first
// catalog read cannot race the seed.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.RequestBackgroundColor,
		seedCategories(m.db),
		fetchQuotes(m.client),
		backfillCloses(m.db, m.client, m.period.Year(), m.today),
	)
}

// reload re-reads every view. The database is a few hundred rows, so a
// write reloads the lot rather than routing each one to what it touches.
func (m Model) reload() tea.Cmd {
	return tea.Batch(
		loadMonth(m.db, m.period),
		loadYear(m.db, m.period.Year(), m.fx()),
		loadNotes(m.db),
		loadCatalog(m.db),
		loadRates(m.db),
	)
}

// fx is the stored closes plus today's quote for the month still running,
// read from whichever house the Rates view has adopted.
func (m Model) fx() month.FxTable {
	live, ok := rates.Sell(m.quotes, m.settings.FxHouse)
	return month.NewFxTable(m.stored, live, ok, m.today)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	return next.sync(), cmd
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.theme = NewTheme(msg.IsDark())

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m.forwardToModal(msg)

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case monthMsg:
		m.lines, m.lastErr = msg.lines, msg.err

	case yearMsg:
		m.year, m.lastErr = msg.year, msg.err

	case notesMsg:
		m.notes, m.lastErr = msg.notes, msg.err
		m.openNote = reopen(m.openNote, msg.notes)

	case catalogMsg:
		m.concepts, m.categories, m.lastErr = msg.concepts, msg.categories, msg.err

	case ratesMsg:
		m.stored, m.settings, m.lastErr = msg.stored, msg.settings, msg.err
		return m, loadYear(m.db, m.period.Year(), m.fx())

	case quotesMsg:
		m.quotes, m.lastErr = msg.quotes, msg.err
		return m, loadYear(m.db, m.period.Year(), m.fx())

	case backfilledMsg:
		if msg.saved == 0 {
			return m, nil
		}
		return m, loadRates(m.db)

	case savedMsg:
		m.lastErr = msg.err
		return m, m.reload()

	default:
		return m.forwardToModal(msg)
	}
	return m, nil
}

// forwardToModal hands a message to an open overlay as well as the model —
// a resize has to reach the form or textarea that is on screen.
func (m Model) forwardToModal(msg tea.Msg) (Model, tea.Cmd) {
	if m.modal == nil {
		return m, nil
	}
	next, cmd := m.modal.Update(msg)
	m.modal = next
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.modal != nil {
		return m.forwardToModal(msg)
	}
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "tab":
		return m.switchView(1)
	case "shift+tab":
		return m.switchView(-1)
	case "left":
		return m.shiftPeriod(-1)
	case "right":
		return m.shiftPeriod(1)
	case "t":
		if !m.wandered() {
			return m, nil
		}
		return m.goTo(m.today)
	case "up":
		return m.moveCursor(-1), nil
	case "down":
		return m.moveCursor(1), nil
	}

	switch m.view {
	case viewMonth:
		return m.handleMonthKey(msg)
	case viewNotes:
		if m.openNote != nil {
			return m.handleNoteDetailKey(msg)
		}
		return m.handleNotesKey(msg)
	case viewConcepts:
		return m.handleConceptsKey(msg)
	case viewRates:
		return m.handleRatesKey(msg)
	}
	return m, nil
}

func (m Model) openModal(next modal) (Model, tea.Cmd) {
	m.modal = next
	return m, next.Init()
}

func (m Model) switchView(delta int) (Model, tea.Cmd) {
	n := view(len(viewNames))
	m.view = (m.view + view(delta) + n) % n
	m.openNote = nil
	if m.view == viewRates {
		m.house = houseIndex(m.settings.FxHouse)
	}
	return m, nil
}

// showsPeriod reports whether a period is on screen for the arrows to move.
// The catalog is period-free, and the note detail is one note rather than a
// month of them.
func (m Model) showsPeriod() bool {
	switch m.view {
	case viewConcepts:
		return false
	case viewNotes:
		return m.openNote == nil
	default:
		return true
	}
}

// shiftPeriod moves a year at a time in the Year view, since a year is the
// unit on that screen.
func (m Model) shiftPeriod(delta int) (Model, tea.Cmd) {
	if !m.showsPeriod() {
		return m, nil
	}
	if m.view == viewYear {
		delta *= 12
	}
	return m.goTo(m.period.AddMonths(delta))
}

// goTo shows another period. The cursor resets because the row it pointed at
// belongs to the month being left.
func (m Model) goTo(p domain.Period) (Model, tea.Cmd) {
	previousYear := m.period.Year()
	m.period = p
	m.monthList.cursor = 0
	m.notesList.cursor = 0
	m.openNote = nil

	cmds := []tea.Cmd{loadMonth(m.db, m.period), loadYear(m.db, m.period.Year(), m.fx())}
	if m.period.Year() != previousYear {
		cmds = append(cmds, backfillCloses(m.db, m.client, m.period.Year(), m.today))
	}
	return m, tea.Batch(cmds...)
}

// wandered reports whether the shown period is somewhere other than the
// month still running, which is the only time going back to it means
// anything.
func (m Model) wandered() bool {
	return m.showsPeriod() && !m.period.Equal(m.today)
}

func (m Model) moveCursor(delta int) Model {
	switch m.view {
	case viewMonth:
		m.monthList = m.monthList.move(delta, len(m.lines))
	case viewNotes:
		if m.openNote != nil {
			m.detail = m.detail.move(delta, len(note.Checkboxes(m.openNote.BodyMD)))
			break
		}
		m.notesList = m.notesList.move(delta, len(m.shownNotes()))
	case viewConcepts:
		m.conceptsList = m.conceptsList.move(delta, len(m.concepts))
	case viewRates:
		m.house = clamp(m.house+delta, len(rates.Houses))
	}
	return m
}

// sync rebuilds the focused view's scrolling region after every message, so
// the rendered rows and the cursor can never disagree.
func (m Model) sync() Model {
	width := m.contentWidth()
	switch m.view {
	case viewMonth:
		m.monthList.cursor = clamp(m.monthList.cursor, len(m.lines))
		rows, anchors := m.monthRows()
		m.monthList = m.monthList.show(rows, anchors, width, m.bodyHeight(3))
	case viewNotes:
		if m.openNote != nil {
			rows, anchors := m.noteDetailRows()
			m.detail = m.detail.show(rows, anchors, m.detailWidth(), m.bodyHeight(2))
			break
		}
		m.notesList.cursor = clamp(m.notesList.cursor, len(m.shownNotes()))
		rows, anchors := m.noteRows()
		m.notesList = m.notesList.show(rows, anchors, width, m.bodyHeight(2))
	case viewConcepts:
		m.conceptsList.cursor = clamp(m.conceptsList.cursor, len(m.concepts))
		rows, anchors := m.conceptRows()
		m.conceptsList = m.conceptsList.show(rows, anchors, width, m.bodyHeight(2))
	}
	return m
}

// Below this floor renderTooSmall takes over instead of a garbled layout.
const (
	minUsableWidth  = 40
	minUsableHeight = 12
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

func (m Model) contentWidth() int  { return max(m.width-6, 1) }
func (m Model) contentHeight() int { return max(m.height-5, 1) }

// bodyHeight is the box minus the view's own header lines, the blank line
// above the help, and the help itself — which is two rows on a narrow
// terminal with a busy view, so it is measured rather than assumed.
func (m Model) bodyHeight(headerLines int) int {
	return max(m.contentHeight()-headerLines-1-lipgloss.Height(m.helpBlock()), 1)
}

// helpBlock wraps the help to the box instead of letting it run past the
// border and push the layout off the bottom of the terminal.
func (m Model) helpBlock() string {
	return m.theme.Help.Width(m.contentWidth()).Render(m.help())
}

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
	footer := lipgloss.NewStyle().Width(m.width).Align(lipgloss.Right).Render(m.tabs())
	app := m.theme.App.Width(m.width).Height(m.height - 1)
	rendered := app.Render(m.viewContent())
	if m.width >= logoMinWidth && m.height >= logoMinHeight {
		rendered = overlayLogo(rendered, m.theme.Logo)
	}
	return rendered + "\n" + footer
}

func (m Model) tabs() string {
	labels := make([]string, len(viewNames))
	for i, name := range viewNames {
		style := m.theme.Tab
		if view(i) == m.view {
			style = m.theme.Active
		}
		labels[i] = style.Render(name)
	}
	return strings.Join(labels, "")
}

// viewContent pads the body so the help always lands on the last line
// inside the box rather than floating under a short view.
func (m Model) viewContent() string {
	help := m.helpBlock()
	height := m.contentHeight() - 1 - lipgloss.Height(help)
	body := lipgloss.NewStyle().Height(height).MaxHeight(height).Render(m.renderBody())

	status := ""
	if m.lastErr != nil {
		status = m.theme.Muted.Render(m.lastErr.Error())
	}
	return body + "\n" + status + "\n" + help
}

// renderBody hands over to an open modal, except the inline amount edit,
// which draws inside the Month row it belongs to.
func (m Model) renderBody() string {
	if _, inline := m.modal.(*amountEdit); m.modal != nil && !inline {
		return m.viewTitle() + "\n\n" + m.modal.View()
	}
	switch m.view {
	case viewMonth:
		return m.renderMonth()
	case viewYear:
		return m.renderYear()
	case viewNotes:
		return m.renderNotes()
	case viewConcepts:
		return m.renderConcepts()
	default:
		return m.renderRates()
	}
}

func (m Model) viewTitle() string {
	return m.theme.Muted.Render(m.view.String() + " · " + m.period.String())
}

func (m Model) help() string {
	if m.modal != nil {
		return m.modal.Help()
	}
	keys := m.viewKeys()
	if m.wandered() {
		keys = append(keys, "t today")
	}
	return strings.Join(append(keys, "tab switch", "q quit"), " · ")
}

// viewKeys is what the focused view adds to the two every screen carries.
func (m Model) viewKeys() []string {
	switch m.view {
	case viewMonth:
		return []string{"↑/↓", "space tick", "e edit", "←/→ month"}
	case viewYear:
		return []string{"←/→ year"}
	case viewNotes:
		if m.openNote != nil {
			return []string{"↑/↓", "space tick", "e edit", "esc back"}
		}
		return []string{"↑/↓", "enter open", "c done", "p pin", "n new", "←/→ month"}
	case viewConcepts:
		return []string{"↑/↓", "n new", "e edit", "d delete"}
	default:
		return []string{"↑/↓", "enter use house", "e set rate", "←/→ month"}
	}
}

// centerInBox centers sparse content rather than pinning it top-left.
// headerRows is what the view has already drawn above it.
func (m Model) centerInBox(content string, headerRows int) string {
	return lipgloss.Place(m.contentWidth(), m.bodyHeight(headerRows), lipgloss.Center, lipgloss.Center, content)
}
