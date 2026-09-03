package tui

import (
	"database/sql"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
)

var monthGroups = [...]catalog.ConceptKind{catalog.Income, catalog.Expense, catalog.Saving, catalog.Chore}

const (
	nameWidth   = 26
	amountWidth = 14
)

func (m Model) orderedLines() []month.Line {
	ordered := make([]month.Line, 0, len(m.lines))
	for _, kind := range monthGroups {
		ordered = append(ordered, linesOfKind(m.lines, kind)...)
	}
	return ordered
}

func (m Model) cursorLine() (month.Line, bool) {
	ordered := m.orderedLines()
	if m.monthList.cursor >= len(ordered) {
		return month.Line{}, false
	}
	return ordered[m.monthList.cursor], true
}

func (m Model) handleMonthKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	l, ok := m.cursorLine()
	if !ok {
		return m, nil
	}
	switch msg.String() {
	case "space":
		id, done := l.Concept.ID, !l.Done
		return m, write(func() error {
			return catalog.SetMonthEntryDone(m.db, id, m.period, done)
		})
	case "e":
		if l.Money == nil {
			return m, nil
		}
		return m.openModal(newAmountEdit(l, m.db, m.period))
	}
	return m, nil
}

type amountEdit struct {
	conceptID int64
	input     textinput.Model
	commit    func(*decimal.Decimal) tea.Cmd
}

func newAmountEdit(l month.Line, db *sql.DB, period domain.Period) *amountEdit {
	ti := textinput.New()
	ti.Prompt = ""
	ti.SetValue(l.Money.Amount.Amount().StringFixed(2))
	ti.SetWidth(amountWidth)
	ti.CursorEnd()
	ti.Focus()

	id := l.Concept.ID
	return &amountEdit{
		conceptID: id,
		input:     ti,
		commit: func(amount *decimal.Decimal) tea.Cmd {
			return write(func() error { return catalog.SetMonthEntryAmount(db, id, period, amount) })
		},
	}
}

func (e *amountEdit) Update(msg tea.Msg) (modal, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc":
			return nil, nil
		case "enter":
			value := strings.TrimSpace(e.input.Value())
			if value == "" {
				return nil, e.commit(nil)
			}
			amount, err := decimal.NewFromString(value)
			if err != nil {
				e.input.Err = err
				return e, nil
			}
			return nil, e.commit(&amount)
		}
	}
	var cmd tea.Cmd
	e.input, cmd = e.input.Update(msg)
	return e, cmd
}

func (e *amountEdit) Init() tea.Cmd { return nil }
func (e *amountEdit) View() string  { return e.input.View() }
func (e *amountEdit) Help() string  { return "enter confirm · blank clears · esc cancel" }

func (m Model) renderMonth() string {
	if len(m.lines) == 0 {
		return m.monthHeader() + "\n\n" +
			m.centerInBox(m.theme.Muted.Render("no concepts yet — add some in Concepts"), 3)
	}
	return m.monthHeader() + "\n\n" + m.monthList.View()
}

func (m Model) monthHeader() string {
	done, total := month.DoneCount(m.lines)
	status := periodStatus(m.period, m.today)
	if m.settings.LastExport != nil {
		status += " · exported " + m.settings.LastExport.Local().Format("2006-01-02")
	}

	left := m.theme.Title.Render(m.period.String()) + "  " + m.theme.Muted.Render(status)
	right := m.theme.Muted.Render(fmt.Sprintf("%d of %d done", done, total))
	return m.spread(left, right) + "\n" + m.renderTotals()
}

func (m Model) renderTotals() string {
	rate := m.fx().At(m.period)
	totals := month.ResolveTotals(m.lines, rate)

	saved := "saved " + formatAmount(totals.Saved.Amount())
	if usd := totals.SavedUSD(rate); !usd.IsZero() {
		saved += m.theme.Muted.Render(" (" + formatAmount(usd) + " USD)")
	}

	pocket := "pocket " + formatAmount(totals.Pocket.Amount())
	if totals.Pocket.Amount().IsNegative() {
		pocket = m.theme.Accent.Render("over by " + formatAmount(totals.Pocket.Amount().Neg()))
	}

	line := strings.Join([]string{
		"available " + formatAmount(totals.Available.Amount()),
		saved,
		pocket,
	}, "   ")
	if totals.Excluded > 0 {
		line += m.theme.Muted.Render(fmt.Sprintf("   · %d left out, no rate", totals.Excluded))
	}
	return line
}

func (m Model) monthRows() ([]string, []int) {
	groups := make([]group, len(monthGroups))
	index := 0
	for i, kind := range monthGroups {
		lines := linesOfKind(m.lines, kind)
		rendered := make([]string, len(lines))
		for j, l := range lines {
			rendered[j] = m.renderLine(l, index == m.monthList.cursor)
			index++
		}
		groups[i] = group{label: groupStyle(i).Render(strings.ToUpper(kind.String())), rows: rendered}
	}
	return groupedRows(groups)
}

func linesOfKind(lines []month.Line, kind catalog.ConceptKind) []month.Line {
	var out []month.Line
	for _, l := range lines {
		if l.Concept.Kind == kind {
			out = append(out, l)
		}
	}
	return out
}

func (m Model) renderLine(l month.Line, selected bool) string {
	cursor := "  "
	if selected {
		cursor = m.theme.Accent.Render("> ")
	}
	check := "[ ] "
	if l.Done {
		check = "[x] "
	}
	name := categoryStyle(m.categories, l.Concept.CategoryID).Width(nameWidth).MaxWidth(nameWidth).Render(l.Concept.Name)
	row := cursor + check + name

	if l.Money == nil {
		return row
	}
	if edit, ok := m.modal.(*amountEdit); ok && edit.conceptID == l.Concept.ID {
		return row + " " + l.Money.Amount.Currency().String() + " " + edit.View()
	}

	style := m.theme.Muted
	if l.Money.Confirmed {
		style = m.theme.Bright
	}
	amount := style.Width(amountWidth).Align(lipgloss.Right).Render(formatAmount(l.Money.Amount.Amount()))
	return row + " " + m.theme.Muted.Render(l.Money.Amount.Currency().String()) + " " + amount
}

func (m Model) spread(left, right string) string {
	gap := m.contentWidth() - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func formatAmount(d decimal.Decimal) string {
	fixed := d.Abs().StringFixed(2)
	point := strings.IndexByte(fixed, '.')
	whole, cents := fixed[:point], fixed[point+1:]

	var b strings.Builder
	if d.IsNegative() {
		b.WriteByte('-')
	}
	for i := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteByte(whole[i])
	}
	if cents != "00" {
		b.WriteByte(',')
		b.WriteString(cents)
	}
	return b.String()
}
