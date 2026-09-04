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

	concepts   []catalog.Concept
	categories []catalog.Category

	stored []catalog.FxRate
	quotes []rates.Quote
	house  int

	monthList    scroller
	yearList     scroller
	notesList    scroller
	conceptsList scroller
	detail       scroller
	notesFocus   notesFocusArea

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

// Init reads through the savedMsg that seeding reports, so the first catalog
// read cannot race the seed.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.RequestBackgroundColor,
		seedCategories(m.db),
		fetchQuotes(m.client),
		backfillCloses(m.db, m.client, m.period.Year(), m.today),
	)
}

func (m Model) reload() tea.Cmd {
	return tea.Batch(
		loadMonth(m.db, m.period),
		loadYear(m.db, m.period.Year(), m.fx()),
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
		m.lines, m.lastErr = msg.lines, msg.err

	case yearMsg:
		m.year, m.lastErr = msg.year, msg.err

	case notesMsg:
		m.notes, m.lastErr = msg.notes, msg.err

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
	m.modal = next
	return m, next.Init()
}

func (m Model) switchView(delta int) (Model, tea.Cmd) {
	n := view(len(viewNames))
	m.view = (m.view + view(delta) + n) % n
	m.notesFocus = focusList
	if m.view == viewRates {
		m.house = houseIndex(m.settings.FxHouse)
	}
	return m, nil
}

func (m Model) showsPeriod() bool { return m.view != viewConcepts }

func (m Model) shiftPeriod(delta int) (Model, tea.Cmd) {
	if !m.showsPeriod() {
		return m, nil
	}
	if m.view == viewYear {
		delta *= 12
	}
	return m.goTo(m.period.AddMonths(delta))
}

func (m Model) goTo(p domain.Period) (Model, tea.Cmd) {
	previousYear := m.period.Year()
	m.period = p
	m.monthList.cursor = 0
	m.yearList.cursor = 0
	m.notesList.cursor = 0
	m.notesFocus = focusList

	cmds := []tea.Cmd{loadMonth(m.db, m.period), loadYear(m.db, m.period.Year(), m.fx())}
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
		m.house = clamp(m.house+delta, len(rates.Houses))
	}
	return m
}

func (m Model) sync() Model {
	width := m.contentWidth()
	switch m.view {
	case viewMonth:
		m.monthList.cursor = clamp(m.monthList.cursor, len(m.lines))
		rows, anchors := m.monthRows()
		// A month that fits gets a viewport its own size, so renderMonth can
		// centre the card rather than pad it out; a month that doesn't fills
		// the screen, keeping a line back for the scroll hint.
		avail := m.monthAvailHeight()
		height := max(len(rows), 1)
		if len(rows) > avail {
			height = max(avail-1, 1)
		}
		m.monthList = m.monthList.show(rows, anchors, tableWidth, height)
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

		// The body is rendered once and then painted: the cursor has to be
		// clamped before the gutter is drawn, and running the markdown through
		// glamour twice a frame to get there is not worth it.
		lines := m.noteBodyLines()
		m.detail.cursor = clamp(m.detail.cursor, len(lines))
		body, stops := m.noteBodyRows(lines)
		m.detail = m.detail.show(body, stops, m.notePaneWidth(), viewportHeight(len(body), m.paneViewHeight()))
	case viewConcepts:
		m.conceptsList.cursor = clamp(m.conceptsList.cursor, len(m.concepts))
		rows, anchors := m.conceptRows()
		m.conceptsList = m.conceptsList.show(rows, anchors, width, m.bodyHeight(2))
	}
	return m
}

// Set above what the layout strictly needs: mess is full-screen only. The
// floor is Year's: two twelve-month charts beside each other, then a category
// ranking beside four totals boxes, all on one card.
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

	status := ""
	if m.lastErr != nil {
		status = m.theme.Muted.Render(m.lastErr.Error())
	}
	return body + "\n" + status + "\n" + m.helpRow()
}

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
		return []string{"↑/↓", "n new", "e edit", "d delete"}
	default:
		return []string{"↑/↓", "enter use house", "e set rate", "←/→ month"}
	}
}

func (m Model) centerInBox(content string, headerRows int) string {
	return lipgloss.Place(m.contentWidth(), m.bodyHeight(headerRows), lipgloss.Center, lipgloss.Center, content)
}
