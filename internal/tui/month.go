package tui

import (
	"database/sql"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mes/internal/catalog"
	"github.com/gabokatta/mes/internal/domain"
	"github.com/gabokatta/mes/internal/month"
)

// monthGroups is the display order for the month view's sections.
var monthGroups = [...]catalog.ConceptKind{catalog.Income, catalog.FixedExpense, catalog.VariableExpense}

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

// rowCount is every cursor position in the month view: concept lines then
// chores, the same order renderMonth walks them in.
func (m Model) rowCount() int {
	return len(m.orderedLines()) + len(m.chores)
}

func (m Model) moveCursor(delta int) int {
	n := m.rowCount()
	if n == 0 {
		return 0
	}
	cursor := m.cursor + delta
	if cursor < 0 {
		return 0
	}
	if cursor >= n {
		return n - 1
	}
	return cursor
}

// cursorLine reports the concept line under the cursor, if the cursor is on
// one rather than a chore.
func (m Model) cursorLine() (month.Line, bool) {
	lines := m.orderedLines()
	if m.cursor >= len(lines) {
		return month.Line{}, false
	}
	return lines[m.cursor], true
}

// cursorChore reports the chore under the cursor, if the cursor is on one
// rather than a concept line.
func (m Model) cursorChore() (month.ChoreLine, bool) {
	idx := m.cursor - len(m.orderedLines())
	if idx < 0 || idx >= len(m.chores) {
		return month.ChoreLine{}, false
	}
	return m.chores[idx], true
}

// toggleDone flips the done state under the cursor. Ticking a concept line
// (false to true) also opens the amount edit, prefilled with the resolved
// amount, so confirming what you owe and checking it off is one motion. A
// chore has no amount, so it only ever flips.
func (m Model) toggleDone() (Model, tea.Cmd) {
	if c, ok := m.cursorChore(); ok {
		return m, setChoreDone(m.db, c.Chore.ID, m.period, !c.Done)
	}
	l, ok := m.cursorLine()
	if !ok {
		return m, nil
	}
	done := !l.Done
	cmd := setDone(m.db, l.Concept.ID, m.period, done)
	if done {
		m = m.beginEdit(l)
	}
	return m, cmd
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
	b.WriteString(m.theme.Muted.Render(m.view.String() + " · " + m.period.String()))

	if m.loadErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("failed to load: " + m.loadErr.Error()))
		return b.String()
	}
	if len(m.lines) == 0 && len(m.chores) == 0 {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("no concepts yet — add some in the Concepts view"))
		return b.String()
	}

	if totals := m.renderTotals(); totals != "" {
		b.WriteString("\n")
		b.WriteString(totals)
	}

	idx := 0
	for _, kind := range monthGroups {
		group := linesForKind(m.lines, kind)
		if len(group) == 0 {
			continue
		}
		b.WriteString("\n\n")
		b.WriteString(m.theme.Title.Render(kind.String()))
		for _, l := range group {
			b.WriteString("\n")
			b.WriteString(m.renderLine(l, idx == m.cursor))
			idx++
		}
	}
	if len(m.chores) > 0 {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Title.Render("Chores"))
		for _, c := range m.chores {
			b.WriteString("\n")
			b.WriteString(m.renderChoreLine(c, idx == m.cursor))
			idx++
		}
	}
	if m.saveErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("failed to save: " + m.saveErr.Error()))
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

func linesForKind(lines []month.Line, kind catalog.ConceptKind) []month.Line {
	var out []month.Line
	for _, l := range lines {
		if l.Concept.Kind == kind {
			out = append(out, l)
		}
	}
	return out
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
	if m.editing != nil && m.editing.conceptID == l.Concept.ID {
		return fmt.Sprintf("%s [%s] %-20s %s %s", cursor, check, l.Concept.Name, l.Concept.Currency, m.editing.input.View())
	}
	status := "confirmed"
	if !l.Confirmed {
		status = m.theme.Muted.Render("projected")
	}
	return fmt.Sprintf("%s [%s] %-20s %s %12s  %s", cursor, check, l.Concept.Name, l.Concept.Currency,
		l.Amount.StringFixed(2), status)
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
	return fmt.Sprintf("%s [%s] %s", cursor, check, c.Chore.Name)
}
