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
	"github.com/gabokatta/mess/internal/list"
	"github.com/gabokatta/mess/internal/month"
)

// monthGroups is the display order for the month view's sections: one list,
// one cursor, money and chore concepts alike.
var monthGroups = [...]catalog.ConceptKind{catalog.Income, catalog.Expense, catalog.Chore}

// monthLoadedMsg is the result of loadMonth's Cmd, delivered back to Update
// once the database read completes.
type monthLoadedMsg struct {
	lines []month.Line
	err   error
}

// loadMonth returns a Cmd that resolves period's lines off the Update loop —
// money and chore concepts alike, one pipeline since S43.
func loadMonth(db *sql.DB, period domain.Period) tea.Cmd {
	return func() tea.Msg {
		loaded, err := month.Load(db, period)
		return monthLoadedMsg{lines: loaded.Lines, err: err}
	}
}

// entrySavedMsg is the result of a month_entry write, which always triggers
// a reload so the resolved lines reflect it.
type entrySavedMsg struct {
	err error
}

func setDone(db *sql.DB, conceptID int64, period domain.Period, done bool) tea.Cmd {
	return func() tea.Msg {
		return entrySavedMsg{err: catalog.SetMonthEntryDone(db, conceptID, period, done)}
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

// rowCount is every cursor position in the Month view: every line, then
// allocations — the same order renderMonthBody walks them in.
func (m Model) rowCount() int {
	return len(m.orderedLines()) + len(m.allocations)
}

func (m Model) moveCursor(delta int) int {
	return clampCursor(m.cursor+delta, m.rowCount())
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

// cursorLine reports the line under the cursor, if it's on one rather than
// an allocation.
func (m Model) cursorLine() (month.Line, bool) {
	lines := m.orderedLines()
	if m.cursor >= len(lines) {
		return month.Line{}, false
	}
	return lines[m.cursor], true
}

// cursorAllocation reports the allocation under the cursor, if it's on one
// rather than a line.
func (m Model) cursorAllocation() (catalog.SavingAllocation, bool) {
	idx := m.cursor - len(m.orderedLines())
	if idx < 0 || idx >= len(m.allocations) {
		return catalog.SavingAllocation{}, false
	}
	return m.allocations[idx], true
}

// toggleDone flips the done state under the cursor and persists it
// immediately — ticking and editing the amount are separate intents, so
// this touches nothing else. A chore has no amount, so its shape is the
// same as a money line.
func (m Model) toggleDone() (Model, tea.Cmd) {
	l, ok := m.cursorLine()
	if !ok {
		return m, nil
	}
	return m, setDone(m.db, l.Concept.ID, m.period, !l.Done)
}

// startEdit opens the amount edit for the cursor's line, or is a no-op on a
// Chore line — there's no amount behind it to edit.
func (m Model) startEdit() (Model, tea.Cmd) {
	l, ok := m.cursorLine()
	if !ok || l.Money == nil {
		return m, nil
	}
	return m.beginEdit(l), nil
}

func (m Model) beginEdit(l month.Line) Model {
	ti := textinput.New()
	ti.SetValue(l.Money.Amount.StringFixed(2))
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
	b.WriteString(m.theme.Muted.Render(m.view.String() + " · " + m.period.String() + " · " + currentPeriodStatus(m.period).String()))

	if m.loadErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("failed to load: " + m.loadErr.Error()))
		return b.String()
	}
	if m.fxErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("fx quote unavailable: " + m.fxErr.Error()))
	}
	assigned := listsForPeriod(m.lists, m.period)
	if len(m.lines) == 0 && len(assigned) == 0 && m.conceptSaveErr == nil {
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

	b.WriteString(m.renderMonthBody(assigned))
	b.WriteString(m.renderLastMonthUnfinished())

	if m.saveErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("failed to save: " + m.saveErr.Error()))
	}
	if m.conceptSaveErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("failed to save: " + m.conceptSaveErr.Error()))
	}
	return b.String()
}

// renderLastMonthUnfinished is the quiet heads-up naming last period's
// unfinished chores — visible without turning into a to-do list of things
// from months ago.
func (m Model) renderLastMonthUnfinished() string {
	if m.lastMonthChoresErr != nil {
		return "\n\n" + m.theme.Muted.Render("last month's chores unavailable: "+m.lastMonthChoresErr.Error())
	}
	return fmt.Sprintf("\n\nLast month: %d unfinished", m.lastMonthUnfinished)
}

// renderMonthBody is the Month view's content: totals, the line list beside
// this period's category chart, the allocation panel, and this period's
// assigned lists.
func (m Model) renderMonthBody(assigned []catalog.List) string {
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
		b.WriteString(m.theme.Title.Render("Lists"))
		for _, p := range assigned {
			done, total := list.Progress(p.BodyMD)
			fmt.Fprintf(&b, "\n  %-20s %d/%d", p.Name, done, total)
		}
	}
	return b.String()
}

// renderFinanceLines is the line list, grouped Income/Expense/Chore, the
// left column of the Month view.
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
			b.WriteString(m.renderLine(l, idx == m.cursor))
			idx++
		}
	}
	return b.String()
}

// renderTotals renders the month header's net: your share leads household,
// every currency folded into one ARS figure at the period's resolved fx
// rate — no separate projected total, since a mid-month forecast is rarely
// the number that ships. A "N of M confirmed" count over money lines and an
// "X of Y chores done" count over Chore lines say what's left to check.
func (m Model) renderTotals() string {
	rate, hasRate := month.ResolveFxRate(m.period, m.rates)
	header := month.ResolveHeaderNet(m.lines, rate, hasRate)
	done, total := month.ChoresDone(m.lines)

	var b strings.Builder
	fmt.Fprintf(&b, "ARS  share %12s  household %12s", header.Net.Share.StringFixed(2), header.Net.Household.StringFixed(2))
	if hasRate && !rate.IsZero() {
		fmt.Fprintf(&b, "  %s", m.theme.Muted.Render("("+header.Net.Share.Div(rate).StringFixed(2)+" USD)"))
	}
	fmt.Fprintf(&b, "  ·  %d of %d confirmed", header.Confirmed, header.Lines)
	if total > 0 {
		fmt.Fprintf(&b, "  ·  %d of %d chores done", done, total)
	}
	return b.String()
}

func listsForPeriod(lists []catalog.List, period domain.Period) []catalog.List {
	var assigned []catalog.List
	for _, p := range lists {
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

// renderLine renders one line — a money line's currency, amount and
// confirmed/projected status, or, for a Chore line with no Money, just the
// check and name. Both share the same cursor/late shape.
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
	line := fmt.Sprintf("%s [%s] %s", cursor, check, name)

	if l.Money != nil {
		if m.editing != nil && m.editing.conceptID == l.Concept.ID {
			line += fmt.Sprintf(" %s %s", l.Concept.Money.Currency, m.editing.input.View())
		} else {
			status := "confirmed"
			if !l.Money.Confirmed {
				status = m.theme.Muted.Render("projected")
			}
			line += fmt.Sprintf(" %s %12s  %s", l.Concept.Money.Currency, l.Money.Amount.StringFixed(2), status)
		}
	}
	if isLate(l, m.period) {
		line += "  " + m.theme.Muted.Render("late")
	}
	return line
}
