package tui

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
)

var monthGroups = [...]catalog.ConceptKind{catalog.Income, catalog.Expense, catalog.Saving, catalog.Chore}

// Month and Concepts share column widths; totals use the remaining space.
const (
	checkWidth    = 4
	nameWidth     = 30
	categoryWidth = 13
	currencyWidth = 3
	amountWidth   = 14

	tableWidth = gutterWidth + checkWidth + nameWidth + colGap + categoryWidth +
		colGap + currencyWidth + colGap + amountWidth

	railMinInterior = 19

	monthGap = 6
)

func (m Model) orderedLines() []month.Line {
	ordered := make([]month.Line, 0, len(m.lines))
	for _, kind := range monthGroups {
		ordered = append(ordered, linesOfKind(m.lines, m.categories, kind)...)
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
		return m.periodHeading() + "\n\n" +
			m.centerInBox(m.theme.Muted.Render("no concepts yet — add some in Concepts"), 3)
	}

	rate := m.fx().At(m.period)
	totals := month.ResolveTotals(m.lines, rate)
	rows := m.monthList.View() + m.scrollHint(m.monthList, gutterWidth)
	sidebar := m.renderRail(totals, rate) + "\n\n" + m.monthMeta(totals)
	body := joinRowsAndSidebar(rows, sidebar)

	card := m.periodHeading() + "\n\n" + m.monthColumnHeader() + "\n" + body
	top := max(0, (m.bodyHeight(0)-lipgloss.Height(card))/2)
	left := max(0, (m.contentWidth()-lipgloss.Width(body))/2)

	return lipgloss.NewStyle().MarginLeft(left).Render(strings.Repeat("\n", top) + card)
}

// Keep rows aligned with the column header; center a shorter sidebar beside them.
func joinRowsAndSidebar(rows, sidebar string) string {
	if pad := (lipgloss.Height(rows) - lipgloss.Height(sidebar)) / 2; pad > 0 {
		sidebar = strings.Repeat("\n", pad) + sidebar
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, rows, strings.Repeat(" ", monthGap), sidebar)
}

func (m Model) monthMeta(totals month.Totals) string {
	done, total := month.DoneCount(m.lines)
	lines := []string{m.theme.Muted.Render(fmt.Sprintf("done  %d / %d", done, total))}
	if totals.Excluded > 0 {
		lines = append(lines, m.theme.Alert.Render(fmt.Sprintf("%d left out, no rate", totals.Excluded)))
	}
	return strings.Join(lines, "\n")
}

func (m Model) monthColumnHeader() string {
	row := strings.Repeat(" ", gutterWidth+checkWidth) +
		leftCol(nameWidth, "CONCEPT") + leftCol(categoryWidth, "CATEGORY") + leftCol(currencyWidth, "CUR") +
		lipgloss.NewStyle().Width(amountWidth).Align(lipgloss.Right).Render("AMOUNT")
	return m.theme.Muted.Render(row)
}

func leftCol(width int, s string) string {
	return lipgloss.NewStyle().Width(width).Render(s) + strings.Repeat(" ", colGap)
}

func (m Model) monthAvailHeight() int {
	const titleLine, columnHeaderLine, blankLine = 1, 1, 1
	return m.bodyHeight(titleLine + blankLine + columnHeaderLine)
}

func (m Model) monthRows() ([]string, []int) {
	rate := m.fx().At(m.period)
	groups := make([]group, len(monthGroups))
	index := 0
	for i, kind := range monthGroups {
		lines := linesOfKind(m.lines, m.categories, kind)
		rendered := make([]string, len(lines))
		for j, l := range lines {
			rendered[j] = m.renderLine(l, index == m.monthList.cursor)
			index++
		}
		groups[i] = group{label: m.kindHeader(kind, month.KindTotal(m.lines, rate, kind)), rows: rendered}
	}
	return groupedRows(groups)
}

func (m Model) kindHeader(kind catalog.ConceptKind, subtotal decimal.Decimal) string {
	label := strings.ToUpper(kind.String())
	if subtotal.IsZero() {
		return m.ruleHeader(label, tableWidth)
	}

	amount := formatAmount(subtotal)
	rule := strings.Repeat("─", max(tableWidth-len(label)-len(amount)-2, 0))
	return m.theme.Title.Render(label) + " " + m.theme.Muted.Render(rule) + " " +
		m.theme.Title.Render(amount)
}

// Keep this order shared by row rendering and cursor selection.
func linesOfKind(lines []month.Line, categories []catalog.Category, kind catalog.ConceptKind) []month.Line {
	var out []month.Line
	for _, l := range lines {
		if l.Concept.Kind == kind {
			out = append(out, l)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return categoryName(categories, out[i].Concept.CategoryID) < categoryName(categories, out[j].Concept.CategoryID)
	})
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
	// Truncated before Style.Width sees it: Width wraps what overflows onto a
	// second line, which desyncs the scroller's one-line-per-row cursor math.
	name := lipgloss.NewStyle().Width(nameWidth).
		Render(ansi.Truncate(l.Concept.Name, nameWidth, "…"))
	category := categoryStyle(m.categories, l.Concept.CategoryID).Width(categoryWidth).
		Render(ansi.Truncate(categoryName(m.categories, l.Concept.CategoryID), categoryWidth, "…"))
	row := cursor + check + name + strings.Repeat(" ", colGap) + category + strings.Repeat(" ", colGap)

	if l.Money == nil {
		return row + strings.Repeat(" ", currencyWidth+colGap+amountWidth)
	}

	currency := m.theme.Muted.Width(currencyWidth).Render(l.Money.Amount.Currency().String())
	row += currency + strings.Repeat(" ", colGap)

	if edit, ok := m.topModal().(*amountEdit); ok && edit.conceptID == l.Concept.ID {
		return row + lipgloss.NewStyle().Width(amountWidth).Align(lipgloss.Right).Render(edit.View())
	}

	style := m.theme.Muted
	if l.Money.Overridden {
		style = m.theme.Bright
	}
	return row + style.Width(amountWidth).Align(lipgloss.Right).Render(formatAmount(l.Money.Amount.Amount()))
}

func (m Model) renderRail(totals month.Totals, rate month.Rate) string {
	boxes := m.railBoxes(totals, rate)
	interior := m.railInterior(boxes)

	rendered := make([]string, len(boxes))
	for i, b := range boxes {
		rendered[i] = m.renderBox(b, interior)
	}
	return strings.Join(rendered, "\n\n")
}

// railLine is one line inside a box, held as parts so the rail can measure
// it before anything is drawn.
type railLine struct {
	label string
	value string
	style lipgloss.Style
}

func (l railLine) width() int { return len([]rune(l.label)) + len([]rune(l.value)) }

type railBox struct {
	title string
	lines []railLine
}

func (m Model) railBoxes(totals month.Totals, rate month.Rate) []railBox {
	return []railBox{
		m.totalsBox("available", totals.Available.Amount(), totals.AvailableUSD(rate), rate.OK(), shortfall{}),
		m.totalsBox("saved", totals.Saved.Amount(), totals.SavedUSD(rate), rate.OK(), shortfall{}),
		m.totalsBox("pocket", totals.Pocket.Amount(), totals.PocketUSD(rate), rate.OK(),
			negativeShortfall(totals.Pocket.Amount(), totals.Overspent())),
	}
}

// Overspending uses an alert; saving beyond the remainder uses a neutral label.
type shortfall struct {
	label string
	alert bool
}

func negativeShortfall(ars decimal.Decimal, overspent bool) shortfall {
	switch {
	case !ars.IsNegative():
		return shortfall{}
	case overspent:
		return shortfall{label: "short by", alert: true}
	default:
		return shortfall{label: "over by"}
	}
}

func (m Model) totalsBox(title string, ars, usd decimal.Decimal, hasRate bool, short shortfall) railBox {
	label, value, style := "ARS", formatAmount(ars), m.theme.Bright
	if short.label != "" {
		// The ARS row already reads "<label> <amount>"; the USD row mirrors
		// that framing instead of repeating the sign as a bare minus.
		label, value = short.label, formatAmount(ars.Abs())
		usd = usd.Abs()
		if short.alert {
			style = m.theme.Alert
		}
	}
	usdValue := "—"
	if hasRate {
		usdValue = formatAmount(usd)
	}
	return railBox{title: title, lines: []railLine{
		{label, value, style},
		{"USD", usdValue, m.theme.Muted},
	}}
}

// Size all boxes for the widest figure, bounded by the space beside the table.
func (m Model) railInterior(boxes []railBox) int {
	interior := railMinInterior
	for _, b := range boxes {
		interior = max(interior, len([]rune(b.title))+3)
		for _, l := range b.lines {
			interior = max(interior, l.width()+4)
		}
	}
	room := m.contentWidth() - tableWidth - monthGap - 2
	return min(interior, max(room, railMinInterior))
}

func (m Model) renderBox(b railBox, interior int) string {
	border := m.theme.Muted
	dashes := max(interior-2-len([]rune(b.title)), 1)

	rendered := []string{
		border.Render("┌ ") + m.theme.Title.Render(b.title) + border.Render(" "+strings.Repeat("─", dashes)+"┐"),
	}
	for _, l := range b.lines {
		rendered = append(rendered, border.Render("│")+railField(l, interior)+border.Render("│"))
	}
	return strings.Join(append(rendered, border.Render("└"+strings.Repeat("─", interior)+"┘")), "\n")
}

// Pad without Style.Width so wide amounts stay on one line.
func railField(l railLine, interior int) string {
	pad := max(interior-3-l.width(), 1)
	return l.style.Render(" " + l.label + strings.Repeat(" ", pad) + l.value + "  ")
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
