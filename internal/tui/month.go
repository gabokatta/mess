package tui

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
	"github.com/gabokatta/mess/internal/project"
)

// monthGroups is the display order for the month view's sections.
var monthGroups = [...]catalog.ConceptKind{catalog.Income, catalog.Expense}

// monthPane is the Month view's two panes, toggled with f — the same
// pattern Projects uses for pending/closed. Splitting the view isn't
// splitting the data: Finance and Chores share one period, but each gets
// its own cursor space.
type monthPane int

const (
	paneFinance monthPane = iota
	paneChores
)

func (p monthPane) String() string {
	if p == paneChores {
		return "Chores"
	}
	return "Finance"
}

func togglePane(p monthPane) monthPane {
	if p == paneFinance {
		return paneChores
	}
	return paneFinance
}

// monthLoadedMsg is the result of loadMonth's Cmd, delivered back to Update
// once the database read completes.
type monthLoadedMsg struct {
	lines  []month.Line
	chores []month.ChoreLine
	err    error
}

// loadMonth returns a Cmd that resolves period's lines and chores off the
// Update loop.
func loadMonth(db *sql.DB, period domain.Period) tea.Cmd {
	return func() tea.Msg {
		loaded, err := month.Load(db, period)
		return monthLoadedMsg{lines: loaded.Lines, chores: loaded.Chores, err: err}
	}
}

// entrySavedMsg is the result of a month_entry or chore_entry write, which
// always triggers a reload so the resolved lines reflect it.
type entrySavedMsg struct {
	err error
}

func setDone(db *sql.DB, conceptID int64, period domain.Period, done bool) tea.Cmd {
	return func() tea.Msg {
		return entrySavedMsg{err: catalog.SetMonthEntryDone(db, conceptID, period, done)}
	}
}

func setChoreDone(db *sql.DB, choreID int64, period domain.Period, done bool) tea.Cmd {
	return func() tea.Msg {
		return entrySavedMsg{err: catalog.SetChoreEntryDone(db, choreID, period, done)}
	}
}

func setAmount(db *sql.DB, conceptID int64, period domain.Period, amount *decimal.Decimal) tea.Cmd {
	return func() tea.Msg {
		return entrySavedMsg{err: catalog.SetMonthEntryAmount(db, conceptID, period, amount)}
	}
}

// editState is the inline amount edit in progress for one line, keyed by
// concept so a reload mid-edit can't desync it from the line it targets.
type editState struct {
	conceptID int64
	input     textinput.Model
}

// orderedLines is m.lines in the same grouped order the month view renders,
// so cursor index and render index always agree.
func (m Model) orderedLines() []month.Line {
	var lines []month.Line
	for _, kind := range monthGroups {
		lines = append(lines, linesForKind(m.lines, kind)...)
	}
	return lines
}

// financeRowCount is every cursor position in the Finance pane: concept
// lines, then allocations — the same order renderFinancePane walks them in.
func (m Model) financeRowCount() int {
	return len(m.orderedLines()) + len(m.allocations)
}

func (m Model) moveFinanceCursor(delta int) int {
	return clampCursor(m.financeCursor+delta, m.financeRowCount())
}

// choreRowCount is every cursor position in the Chores pane: its own list,
// independent of Finance's.
func (m Model) choreRowCount() int {
	return len(m.chores)
}

func (m Model) moveChoreCursor(delta int) int {
	return clampCursor(m.choreCursor+delta, m.choreRowCount())
}

// moveMonthCursor moves whichever pane's cursor is active — j/k's single
// entry point, so the caller doesn't need to know which pane it's in.
func (m Model) moveMonthCursor(delta int) Model {
	if m.monthPane == paneChores {
		m.choreCursor = m.moveChoreCursor(delta)
	} else {
		m.financeCursor = m.moveFinanceCursor(delta)
	}
	return m
}

func clampCursor(cursor, n int) int {
	if n == 0 {
		return 0
	}
	if cursor < 0 {
		return 0
	}
	if cursor >= n {
		return n - 1
	}
	return cursor
}

// cursorLine reports the concept line under the Finance pane's cursor, if
// it's on one rather than an allocation.
func (m Model) cursorLine() (month.Line, bool) {
	lines := m.orderedLines()
	if m.financeCursor >= len(lines) {
		return month.Line{}, false
	}
	return lines[m.financeCursor], true
}

// cursorChore reports the chore under the Chores pane's cursor.
func (m Model) cursorChore() (month.ChoreLine, bool) {
	if m.choreCursor >= len(m.chores) {
		return month.ChoreLine{}, false
	}
	return m.chores[m.choreCursor], true
}

// cursorAllocation reports the allocation under the Finance pane's cursor,
// if it's on one rather than a concept line.
func (m Model) cursorAllocation() (catalog.SavingAllocation, bool) {
	idx := m.financeCursor - len(m.orderedLines())
	if idx < 0 || idx >= len(m.allocations) {
		return catalog.SavingAllocation{}, false
	}
	return m.allocations[idx], true
}

// toggleDone flips the done state under the active pane's cursor and
// persists it immediately — ticking and editing the amount are separate
// intents, so this touches nothing else. A chore has no amount, so its
// shape is the same.
func (m Model) toggleDone() (Model, tea.Cmd) {
	if m.monthPane == paneChores {
		c, ok := m.cursorChore()
		if !ok {
			return m, nil
		}
		return m, setChoreDone(m.db, c.Chore.ID, m.period, !c.Done)
	}
	l, ok := m.cursorLine()
	if !ok {
		return m, nil
	}
	return m, setDone(m.db, l.Concept.ID, m.period, !l.Done)
}

func (m Model) startEdit() (Model, tea.Cmd) {
	l, ok := m.cursorLine()
	if !ok {
		return m, nil
	}
	return m.beginEdit(l), nil
}

func (m Model) beginEdit(l month.Line) Model {
	ti := textinput.New()
	ti.SetValue(l.Amount.StringFixed(2))
	ti.CursorEnd()
	ti.Focus()
	m.editing = &editState{conceptID: l.Concept.ID, input: ti}
	return m
}

func (m Model) updateEditing(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.editing = nil
		return m, nil
	case "enter":
		return m.commitEdit()
	}
	var cmd tea.Cmd
	m.editing.input, cmd = m.editing.input.Update(msg)
	return m, cmd
}

func (m Model) commitEdit() (Model, tea.Cmd) {
	value := strings.TrimSpace(m.editing.input.Value())
	conceptID := m.editing.conceptID

	var amount *decimal.Decimal
	if value != "" {
		d, err := decimal.NewFromString(value)
		if err != nil {
			m.editing.input.Err = err
			return m, nil
		}
		amount = &d
	}

	m.editing = nil
	return m, setAmount(m.db, conceptID, m.period, amount)
}

func (m Model) renderMonth() string {
	var b strings.Builder
	b.WriteString(m.theme.Muted.Render(m.view.String() + " · " + m.monthPane.String() + " · " + m.period.String() + " · " + currentPeriodStatus(m.period).String()))

	if m.loadErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("failed to load: " + m.loadErr.Error()))
		return b.String()
	}
	if m.fxErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("fx quote unavailable: " + m.fxErr.Error()))
	}
	if m.choreForm != nil {
		b.WriteString("\n\n")
		b.WriteString(m.choreForm.form.View())
		return b.String()
	}
	assigned := projectsForPeriod(m.projects, m.period)
	if len(m.lines) == 0 && len(m.chores) == 0 && len(assigned) == 0 && m.choreSaveErr == nil {
		b.WriteString("\n")
		b.WriteString(m.centerInBox(m.theme.Muted.Render("no concepts yet — add some in the Concepts view")))
		return b.String()
	}

	if m.incomeConfirmForm != nil {
		b.WriteString("\n\n")
		b.WriteString(m.incomeConfirmForm.form.View())
		return b.String()
	}
	if m.allocationForm != nil {
		b.WriteString("\n\n")
		b.WriteString(m.allocationForm.form.View())
		return b.String()
	}

	if m.monthPane == paneChores {
		b.WriteString(m.renderChoresPane())
	} else {
		b.WriteString(m.renderFinancePane(assigned))
	}
	b.WriteString(m.renderLastMonthUnfinished())

	if m.saveErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("failed to save: " + m.saveErr.Error()))
	}
	if m.choreSaveErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("failed to save chore: " + m.choreSaveErr.Error()))
	}
	return b.String()
}

// renderLastMonthUnfinished is the quiet heads-up shared by both panes —
// visible without switching to Chores just to see it.
func (m Model) renderLastMonthUnfinished() string {
	if m.lastMonthChoresErr != nil {
		return "\n\n" + m.theme.Muted.Render("last month's chores unavailable: "+m.lastMonthChoresErr.Error())
	}
	return fmt.Sprintf("\n\nLast month: %d unfinished", m.lastMonthUnfinished)
}

// renderFinancePane is the default pane: totals, the concept-line list
// beside this period's category chart, the allocation panel, and this
// period's assigned projects.
func (m Model) renderFinancePane(assigned []catalog.Project) string {
	var b strings.Builder
	if totals := m.renderTotals(); totals != "" {
		b.WriteString("\n")
		b.WriteString(totals)
	}

	b.WriteString("\n\n")
	b.WriteString(m.renderFinanceColumns())

	b.WriteString("\n\n")
	b.WriteString(m.renderAllocations(len(m.orderedLines())))

	if len(assigned) > 0 {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Title.Render("Projects"))
		for _, p := range assigned {
			done, total := project.Progress(p.BodyMD)
			fmt.Fprintf(&b, "\n  %-20s %d/%d", p.Name, done, total)
		}
	}
	return b.String()
}

// renderFinanceLines is the concept-line list, grouped Income/Expense, the
// left column of the Finance pane.
func (m Model) renderFinanceLines() string {
	var b strings.Builder
	idx := 0
	for i, kind := range monthGroups {
		group := linesForKind(m.lines, kind)
		if len(group) == 0 {
			continue
		}
		if i > 0 || b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(m.theme.Title.Render(kind.String()))
		for _, l := range group {
			b.WriteString("\n")
			b.WriteString(m.renderLine(l, idx == m.financeCursor))
			idx++
		}
	}
	return b.String()
}

// renderChoresPane is the Chores pane: its own cursor-addressable list,
// unrelated to Finance's cursor space.
func (m Model) renderChoresPane() string {
	if len(m.chores) == 0 {
		return "\n" + m.centerInBox(m.theme.Muted.Render("no chores yet — press n to add one"))
	}
	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString(m.theme.Title.Render("Chores"))
	for i, c := range m.chores {
		b.WriteString("\n")
		b.WriteString(m.renderChoreLine(c, i == m.choreCursor))
	}
	return b.String()
}

// totalCurrencies is the fixed render order for the header's per-currency
// totals; domain only defines these two.
var totalCurrencies = [...]domain.Currency{domain.ARS, domain.USD}

// renderTotals renders the month header's net: your share leads household,
// projected against confirmed, one row pair per currency actually present
// among this month's lines.
func (m Model) renderTotals() string {
	totals := month.ResolveTotals(m.lines)
	var b strings.Builder
	for _, cur := range totalCurrencies {
		projected, ok := totals.Projected[cur]
		if !ok {
			continue
		}
		rows := [2]struct {
			label string
			net   month.Net
		}{
			{"projected", projected},
			{"confirmed", totals.Confirmed[cur]},
		}
		for _, row := range rows {
			fmt.Fprintf(&b, "%s %s  share %12s  household %12s\n", cur, row.label, row.net.Share.StringFixed(2), row.net.Household.StringFixed(2))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func projectsForPeriod(projects []catalog.Project, period domain.Period) []catalog.Project {
	var assigned []catalog.Project
	for _, p := range projects {
		if p.Period == period {
			assigned = append(assigned, p)
		}
	}
	return assigned
}

// linesForKind filters to kind and orders by due day ascending, unset (0)
// last — SliceStable so ties keep the catalog's own sort_order/name order.
func linesForKind(lines []month.Line, kind catalog.ConceptKind) []month.Line {
	var out []month.Line
	for _, l := range lines {
		if l.Concept.Kind == kind {
			out = append(out, l)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return dueDayRank(out[i].Concept.DueDay) < dueDayRank(out[j].Concept.DueDay)
	})
	return out
}

// dueDayRank sorts after every real day 1-31, so a concept with no due day
// falls to the end of its group instead of jumping the queue at "day 0".
func dueDayRank(dueDay int) int {
	if dueDay == 0 {
		return 32
	}
	return dueDay
}

// isLate reports whether l's due day has passed within period, and only
// within the actual current calendar period — a past period is closed by
// construction, not late, and a future one hasn't arrived yet.
func isLate(l month.Line, period domain.Period) bool {
	if l.Concept.DueDay == 0 || l.Done {
		return false
	}
	if currentPeriodStatus(period) != periodCurrent {
		return false
	}
	return time.Now().Day() > l.Concept.DueDay
}

func (m Model) renderLine(l month.Line, selected bool) string {
	cursor := " "
	if selected {
		cursor = ">"
	}
	check := " "
	if l.Done {
		check = "x"
	}
	name := categoryStyle(m.categories, l.Concept.CategoryID).Render(fmt.Sprintf("%-20s", l.Concept.Name))
	if m.editing != nil && m.editing.conceptID == l.Concept.ID {
		return fmt.Sprintf("%s [%s] %s %s %s", cursor, check, name, l.Concept.Currency, m.editing.input.View())
	}
	status := "confirmed"
	if !l.Confirmed {
		status = m.theme.Muted.Render("projected")
	}
	line := fmt.Sprintf("%s [%s] %s %s %12s  %s", cursor, check, name, l.Concept.Currency,
		l.Amount.StringFixed(2), status)
	if isLate(l, m.period) {
		line += "  " + m.theme.Muted.Render("late")
	}
	return line
}

func (m Model) renderChoreLine(c month.ChoreLine, selected bool) string {
	cursor := " "
	if selected {
		cursor = ">"
	}
	check := " "
	if c.Done {
		check = "x"
	}
	line := fmt.Sprintf("%s [%s] %s", cursor, check, c.Chore.Name)
	if isChoreLate(c, m.period) {
		line += "  " + m.theme.Muted.Render("late")
	}
	return line
}

// sortChoresByDueDay orders by due day ascending, unset (0) last —
// SliceStable so ties keep the catalog's own sort_order/name order, the
// same rule linesForKind applies to concept lines.
func sortChoresByDueDay(chores []month.ChoreLine) []month.ChoreLine {
	sorted := make([]month.ChoreLine, len(chores))
	copy(sorted, chores)
	sort.SliceStable(sorted, func(i, j int) bool {
		return dueDayRank(sorted[i].Chore.DueDay) < dueDayRank(sorted[j].Chore.DueDay)
	})
	return sorted
}

// isChoreLate is isLate's counterpart for chores.
func isChoreLate(c month.ChoreLine, period domain.Period) bool {
	if c.Chore.DueDay == 0 || c.Done {
		return false
	}
	if currentPeriodStatus(period) != periodCurrent {
		return false
	}
	return time.Now().Day() > c.Chore.DueDay
}
