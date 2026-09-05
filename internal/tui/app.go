package tui

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
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

	today  domain.Period
	period domain.Period

	settings catalog.Settings
	lines    []month.Line
	year     month.Year
	notes    []catalog.Note
	monthSeq uint64
	yearSeq  uint64

	concepts   []catalog.Concept
	categories []catalog.Category

	// Retired concepts are hidden until asked for: the only reason to look at
	// one is to bring it back, and the meta cluster says how many there are.
	showRetired bool

	stored []catalog.FxRate
	quotes []rates.Quote

	monthList    scroller
	yearList     scroller
	notesList    scroller
	conceptsList scroller
	ratesList    scroller
	detail       scroller
	notesFocus   notesFocusArea

	// A stack, so a confirm can open over a modal and return to it. Only the
	// top one sees input and only the top one renders.
	modals []modal

	// Timer sequence numbers prevent an older timer from clearing a newer flash.
	flash    string
	flashSeq int
}

// flashDuration is how long a refusal stays on screen: long enough to read a
// sentence, short enough to be gone before the next thing you do.
const flashDuration = 4 * time.Second

type flashExpired struct{ seq int }

// A nil error leaves the current flash and its timer alone.
func (m Model) flashError(err error) (Model, tea.Cmd) {
	if err == nil {
		return m, nil
	}
	m.flash = err.Error()
	m.flashSeq++
	seq := m.flashSeq
	return m, tea.Tick(flashDuration, func(time.Time) tea.Msg { return flashExpired{seq: seq} })
}

// Invalidate the pending timer as well as clearing the message.
func (m Model) clearFlash() Model {
	m.flash = ""
	m.flashSeq++
	return m
}

func New(db *sql.DB) Model {
	now := domain.PeriodFromTime(time.Now())
	return Model{
		theme:  NewTheme(true),
		db:     db,
		client: rates.NewClient(),
		today:  now,
		period: now,
		// The rates table is a year of months, so its cursor's home is the
		// month the rest of the app is showing.
		ratesList: scroller{cursor: int(now.Month()) - 1},
	}
}

// The seed's savedMsg starts the first load after default categories exist.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.RequestBackgroundColor,
		seedCategories(m.db),
		fetchQuotes(m.client),
		backfillCloses(m.db, m.client, m.period.Year(), m.today),
	)
}

func (m *Model) reload() tea.Cmd {
	return tea.Batch(
		m.loadMonth(),
		loadNotes(m.db),
		loadCatalog(m.db),
		loadRates(m.db),
	)
}

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
		if msg.seq != m.monthSeq {
			return m, nil
		}
		if msg.err != nil {
			return m.flashError(msg.err)
		}
		m.lines = msg.lines

	case yearMsg:
		if msg.seq != m.yearSeq {
			return m, nil
		}
		if msg.err != nil {
			return m.flashError(msg.err)
		}
		m.year = msg.year

	case notesMsg:
		if msg.err != nil {
			return m.flashError(msg.err)
		}
		m.notes = msg.notes

	case catalogMsg:
		if msg.err != nil {
			return m.flashError(msg.err)
		}
		m.concepts, m.categories = msg.concepts, msg.categories

	case ratesMsg:
		if msg.err != nil {
			return m.flashError(msg.err)
		}
		m.stored, m.settings = msg.stored, msg.settings
		cmd := m.loadYear()
		return m, cmd

	case quotesMsg:
		if msg.err != nil {
			return m.flashError(msg.err)
		}
		m.quotes = msg.quotes
		cmd := m.loadYear()
		return m, cmd

	case backfilledMsg:
		if msg.saved == 0 {
			return m, nil
		}
		return m, loadRates(m.db)

	case flashExpired:
		if msg.seq == m.flashSeq {
			m.flash = ""
		}

	case savedMsg:
		// The reload that follows reports success on five messages, and every
		// one of them used to overwrite the refusal that started it.
		if msg.err == nil {
			m = m.clearFlash()
		}
		next, cmd := m.flashError(msg.err)
		reload := next.reload()
		return next, tea.Batch(cmd, reload)

	default:
		return m.forwardToModal(msg)
	}
	return m, nil
}

func (m Model) topModal() modal {
	if len(m.modals) == 0 {
		return nil
	}
	return m.modals[len(m.modals)-1]
}

// Update returns itself to stay open, nil to close, or another modal to push.
func (m Model) forwardToModal(msg tea.Msg) (Model, tea.Cmd) {
	top := m.topModal()
	if top == nil {
		return m, nil
	}

	next, cmd := top.Update(msg)
	switch {
	case next == nil:
		m.modals = m.modals[:len(m.modals)-1]
	case next != top:
		// A modal opened over another still has to start, the same as one
		// opened from a screen.
		m.modals = append(m.modals, next)
		cmd = tea.Batch(cmd, next.Init())
	}
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.topModal() != nil {
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
		if m.notesFocus == focusBody {
			return m.handleNoteBodyKey(msg)
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
	m.modals = append(m.modals, next)
	return m, next.Init()
}

func (m Model) switchView(delta int) (Model, tea.Cmd) {
	n := view(len(viewNames))
	m.view = (m.view + view(delta) + n) % n
	m.notesFocus = focusList
	if m.view == viewRates {
		m.ratesList.cursor = int(m.period.Month()) - 1
	}
	return m, nil
}

func (m Model) showsPeriod() bool { return m.view != viewConcepts }

func (m Model) shiftPeriod(delta int) (Model, tea.Cmd) {
	if !m.showsPeriod() {
		return m, nil
	}
	// Both screens show a year, so both step by one.
	if m.view == viewYear || m.view == viewRates {
		delta *= 12
	}
	return m.goTo(m.period.AddMonths(delta))
}

func (m Model) goTo(p domain.Period) (Model, tea.Cmd) {
	previousYear := m.period.Year()
	m.period = p
	m.lines = nil
	m.year = month.Year{Year: p.Year()}
	m.monthList.cursor = 0
	m.yearList.cursor = 0
	m.notesList.cursor = 0
	m.ratesList.cursor = int(p.Month()) - 1
	m.notesFocus = focusList

	cmds := []tea.Cmd{m.loadMonth(), m.loadYear()}
	if m.period.Year() != previousYear {
		cmds = append(cmds, backfillCloses(m.db, m.client, m.period.Year(), m.today))
	}
	return m, tea.Batch(cmds...)
}

func (m Model) wandered() bool {
	return m.showsPeriod() && !m.period.Equal(m.today)
}

func (m Model) moveCursor(delta int) Model {
	switch m.view {
	case viewMonth:
		m.monthList = m.monthList.move(delta, len(m.lines))
	case viewYear:
		m.yearList = m.yearList.move(delta, len(m.year.Categories))
	case viewNotes:
		if m.notesFocus == focusBody {
			m.detail = m.detail.move(delta, len(m.noteBodyLines()))
			break
		}
		m.notesList = m.notesList.move(delta, len(m.shownNotes()))
	case viewConcepts:
		m.conceptsList = m.conceptsList.move(delta, len(m.concepts))
	case viewRates:
		m.ratesList = m.ratesList.move(delta, ratesMonths)
	}
	return m
}

func (m Model) sync() Model {
	// A modal holding its own copy of the catalog reads it back after every
	// write, so the list never shows state the database has moved past.
	if categories, ok := m.topModal().(*categoryList); ok {
		categories.refresh(m.categories, m.conceptCounts())
	}

	switch m.view {
	case viewMonth:
		m.monthList.cursor = clamp(m.monthList.cursor, len(m.lines))
		rows, anchors := m.monthRows()
		m.monthList = m.monthList.show(rows, anchors, tableWidth,
			viewportHeight(len(rows), m.monthAvailHeight()))
	case viewYear:
		// The list has no cursor of its own to render: up and down pan the
		// viewport, since nothing on this screen opens.
		m.yearList.cursor = clamp(m.yearList.cursor, len(m.year.Categories))
		barWidth := m.catBarWidth(m.yearInterior())
		rows := m.categoryRows(barWidth, m.yearList.cursor)
		// The list never grows past the boxes beside it; a sixth category
		// pans instead, so the two lower blocks stay level at every size.
		height := max(min(len(rows), catVisibleRows), 1)
		m.yearList = m.yearList.show(rows, rowAnchors(len(rows)), m.catRowWidth(barWidth), height)
	case viewNotes:
		m.notesList.cursor = clamp(m.notesList.cursor, len(m.shownNotes()))
		rows, anchors := m.noteRows()
		m.notesList = m.notesList.show(rows, anchors, m.noteListWidth(),
			viewportHeight(len(rows), m.listViewHeight(len(rows))))

		// Clamp before painting the gutter, using the same rendered lines.
		lines := m.noteBodyLines()
		m.detail.cursor = clamp(m.detail.cursor, len(lines))
		body, stops := m.noteBodyRows(lines)
		m.detail = m.detail.show(body, stops, m.notePaneWidth(), viewportHeight(len(body), m.paneViewHeight()))
	case viewConcepts:
		m.conceptsList.cursor = clamp(m.conceptsList.cursor, len(m.concepts))
		rows, anchors := m.conceptRows()
		// Title, its blank line, and the column header sit above the list.
		m.conceptsList = m.conceptsList.show(rows, anchors, conceptsTableWidth,
			viewportHeight(len(rows), m.bodyHeight(3)))
	case viewRates:
		m.ratesList.cursor = clamp(m.ratesList.cursor, ratesMonths)
		rows := m.rateTableRows()
		m.ratesList = m.ratesList.show(rows, rowAnchors(len(rows)), m.ratesTableWidth(),
			viewportHeight(len(rows), m.ratesListHeight()))
	}
	return m
}

const (
	minUsableWidth        = 135
	minUsableHeight       = 30
	tooSmallHeadlineWidth = 41
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
func (m Model) contentHeight() int { return max(m.height-4, 1) }

func (m Model) bodyHeight(headerLines int) int {
	return max(m.contentHeight()-headerLines-1-lipgloss.Height(m.helpBlock()), 1)
}

func (m Model) helpBlock() string {
	return m.theme.Help.Width(max(m.contentWidth()-logoTail-logoGap-logoWidth, 1)).Render(m.help())
}

func (m Model) helpRow() string {
	return lipgloss.JoinHorizontal(lipgloss.Bottom,
		m.helpBlock(),
		strings.Repeat(" ", logoGap),
		m.theme.Logo.Render(logoLines[0]))
}

func (m Model) renderTooSmall() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	if m.width >= minUsableWidth && m.height >= minUsableHeight {
		return ""
	}

	headline := m.theme.Muted.Width(min(m.width, tooSmallHeadlineWidth)).Align(lipgloss.Center).
		Render("make the terminal bigger to see your mess")
	have := m.theme.Muted.Render("have ") + m.shortSide(m.width, minUsableWidth) +
		m.theme.Muted.Render(" × ") + m.shortSide(m.height, minUsableHeight)
	need := m.theme.Muted.Render(fmt.Sprintf("need %3d × %3d", minUsableWidth, minUsableHeight))

	block := lipgloss.JoinVertical(lipgloss.Center, headline, "", have, need)
	return lipgloss.PlaceVertical(m.height, lipgloss.Center,
		lipgloss.PlaceHorizontal(m.width, lipgloss.Center, block))
}

func (m Model) shortSide(have, need int) string {
	text := fmt.Sprintf("%3d", have)
	if have >= need {
		return m.theme.Muted.Render(text)
	}
	return m.theme.Alert.Render(text)
}

func (m Model) renderApp() string {
	footer := lipgloss.NewStyle().Width(m.width).Align(lipgloss.Right).Render(m.tabs())
	app := m.theme.App.Width(m.width).Height(m.height - 1)
	rendered := overlayLogo(app.Render(m.viewContent()), m.theme.Logo)
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

func (m Model) viewContent() string {
	help := m.helpBlock()
	height := m.contentHeight() - 1 - lipgloss.Height(help)
	body := lipgloss.NewStyle().Height(height).MaxHeight(height).Render(m.renderBody())

	// Alert, not muted: this line only ever says that something you asked for
	// did not happen, and the one before it was easy to miss.
	status := ""
	if m.flash != "" {
		status = m.theme.Alert.Bold(true).Render(m.flash)
	}
	return body + "\n" + status + "\n" + m.helpRow()
}

func (m Model) renderBody() string {
	// A modal that sizes itself to its own content has to be placed, or it
	// draws in the corner of whatever space it was handed.
	switch top := m.topModal(); top.(type) {
	case nil, *amountEdit:
		// Nothing: the screen renders, and an amount edit draws inside its row.
	default:
		return m.viewTitle() + "\n\n" + m.centerInBox(top.View(), 2)
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
	if top := m.topModal(); top != nil {
		return top.Help()
	}
	keys := m.viewKeys()
	if m.wandered() {
		keys = append(keys, "t today")
	}
	return strings.Join(append(keys, "tab switch", "q quit"), " · ")
}

func (m Model) viewKeys() []string {
	switch m.view {
	case viewMonth:
		return []string{"↑/↓", "space tick", "e edit", "←/→ month"}
	case viewYear:
		return []string{"↑/↓", "←/→ year"}
	case viewNotes:
		if m.notesFocus == focusBody {
			return []string{"↑/↓", "space tick", "e edit", "esc list"}
		}
		return []string{"↑/↓", "enter read", "space close", "p pin", "n new", "←/→ month"}
	case viewConcepts:
		keys := []string{"↑/↓", "n new", "e edit", "d delete", "c categories"}
		if m.retiredCount() > 0 {
			keys = append(keys, "r retired")
		}
		return keys
	default:
		return []string{"↑/↓", "e set rate", "d clear", "h house", "←/→ year"}
	}
}

func (m Model) centerInBox(content string, headerRows int) string {
	return lipgloss.Place(m.contentWidth(), m.bodyHeight(headerRows), lipgloss.Center, lipgloss.Center, content)
}
