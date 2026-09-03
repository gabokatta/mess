package tui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/month"
)

// The Year card is two blocks stacked: a pair of twelve-month charts, and a
// row that puts the category ranking beside a 2x2 grid of totals boxes. Both
// blocks hang off the same left edge, and the card centres as a whole.
const (
	yearGap = 4

	// The charts stop saying anything new past ten plot rows, so the slack on
	// a tall terminal goes to the category list instead. On a short one the
	// charts give way first, down to yearPlotMin.
	yearPlotHeight = 10
	yearPlotMin    = 4

	// Chrome is every line of the card that is not a plot row or a category
	// row: title, the two blank separators, the chart header, its marker line
	// and the blank under that, axis, labels, the section title and its
	// column header.
	yearChrome = 10

	// The box grid is two rows of two, four lines each with a blank between.
	yearGridHeight = 9

	// Five categories is where the ranking stops telling you anything: the
	// sixth is already a sliver against the leader. Past it the list pans,
	// rather than growing the block and leaving the card ragged.
	catVisibleRows = 5

	yearBarMin, yearBarMax = 3, 6
	yearBoxInterior        = 26

	catGutterWidth = 2
	catNameWidth   = 14
	catAmountMin   = 12
	catShareWidth  = 5
	catBarMin      = 12
	catBarMax      = 38
)

// eighths fill a cell from the bottom up; leftEighths fill it left to right.
// Index 0 is a sliver too small to draw.
var (
	eighths     = [...]string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇"}
	leftEighths = [...]string{" ", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}
)

// yearBar is one column of a year chart. Pending marks a month that has not
// happened, whose label reads as waiting rather than as a confirmed zero.
type yearBar struct {
	label   string
	value   decimal.Decimal
	current bool
	pending bool
}

func (m Model) renderYear() string {
	header := m.yearHeader()
	if m.year.Confirmed() == 0 {
		return header + "\n\n" + m.centerInBox(m.theme.Muted.Render("nothing confirmed this year yet"), 2)
	}

	interior := m.yearInterior()
	grid := m.renderBoxGrid(interior)

	lower := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderCategoryBlock(m.catBarWidth(interior)), strings.Repeat(" ", yearGap), grid)
	card := lipgloss.Width(lower)

	// The card travels as one piece, the way Month's does: the slack a quiet
	// year leaves over sits half above it and half below rather than pooling
	// at the bottom.
	body := header + "\n\n" + m.renderCharts(card, m.yearPlot()) + "\n\n" + lower
	top := max(0, (m.bodyHeight(0)-lipgloss.Height(body))/2)
	left := max(0, (m.contentWidth()-card)/2)
	return lipgloss.NewStyle().MarginLeft(left).Render(strings.Repeat("\n", top) + body)
}

// yearHeader is the year and how much of it has happened. A year that lost
// lines for want of a rate says so here rather than quietly under-reporting.
func (m Model) yearHeader() string {
	head := m.theme.Title.Underline(true).Render(strconv.Itoa(m.period.Year()))
	meta := fmt.Sprintf("%d of 12 months confirmed", m.year.Confirmed())
	if m.year.Excluded > 0 {
		return head + "  " + m.theme.Muted.Render(meta) +
			m.theme.Alert.Render(fmt.Sprintf("  ·  %d left out, no rate", m.year.Excluded))
	}
	return head + "  " + m.theme.Muted.Render(meta)
}

// yearPlot is how many rows the charts get. The category list does not take
// the slack on a tall terminal: it stays the height of the boxes beside it, so
// the card centres with the two lower blocks level. The plot gives way first
// on a short one.
func (m Model) yearPlot() int {
	avail := m.bodyHeight(0)
	return min(yearPlotHeight, max(avail-yearChrome-yearGridHeight, yearPlotMin))
}

// ── charts ──────────────────────────────────────────────────────────────────

// renderCharts puts pocket beside spend. Pocket leads because it is the only
// one of the four figures with any shape to it: earned, spent and saved all
// drift up a few percent a month, so from a zero baseline every bar of theirs
// is within a fifth of the tallest.
func (m Model) renderCharts(card, plot int) string {
	half := (card - yearGap) / 2
	barWidth := min(max((half-11)/12, yearBarMin), yearBarMax)
	width := 12*(barWidth+1) - 1

	pocket := m.renderChart("POCKET PER MONTH", month.MonthTotals.Pocket, true, barWidth, plot, width)
	spent := m.renderChart("SPENT PER MONTH", spentOf, false, barWidth, plot, width)

	// Twelve bars rarely divide the half exactly, so the leftover goes into
	// the gap: the pair then shares both edges with the row beneath it.
	gap := max(card-2*width, yearGap)
	return lipgloss.JoinHorizontal(lipgloss.Top, pocket, strings.Repeat(" ", gap), spent)
}

func spentOf(t month.MonthTotals) decimal.Decimal { return t.Spent }

func (m Model) renderChart(title string, value func(month.MonthTotals) decimal.Decimal,
	low bool, barWidth, plot, width int) string {

	bars := make([]yearBar, len(m.year.Months))
	for i, t := range m.year.Months {
		bars[i] = yearBar{
			label:   shortMonth(t.Period.Month()),
			value:   value(t),
			current: t.Period.Equal(m.today),
			pending: !t.Confirmed && t.Period.After(m.today),
		}
	}

	// The marker gets its own line rather than sharing the header's. Beside a
	// sixteen-column title it had forty columns, which a year running to
	// eight figures spends on the peso alone, and the dollar half it dropped
	// was the half worth having. On its own line it has the whole chart.
	// Both charts always spend the line, so their plots stay level.
	head := m.theme.Title.Render(title) + "\n" + m.chartNote(value, low, width)
	return head + "\n\n" + m.renderPlot(bars, barWidth, plot) + "\n" + m.renderAxis(bars, barWidth, width)
}

// chartNote names the month a series turns at, in both currencies, so a bar's
// height carries a number. It drops the dollar half, then itself, rather than
// running past the chart it labels.
func (m Model) chartNote(value func(month.MonthTotals) decimal.Decimal, low bool, room int) string {
	t, ok := turningPoint(m.year.Months, value, low)
	if !ok {
		return ""
	}

	word := "peak"
	if low {
		word = "low"
	}
	short := fmt.Sprintf("%s %s · ARS %s", word, shortMonth(t.Period.Month()), formatAmount(value(t)))

	if rate := m.fx().At(t.Period); rate.OK() && !rate.Value.IsZero() {
		if full := short + " · USD " + formatAmount(value(t).Div(rate.Value)); lipgloss.Width(full) <= room {
			return m.theme.Muted.Render(full)
		}
	}
	if lipgloss.Width(short) > room {
		return ""
	}
	return m.theme.Muted.Render(short)
}

// turningPoint is the confirmed month where value is highest, or lowest when
// low is set. Pending months are not candidates: a year that has not reached
// December has no low there.
func turningPoint(months []month.MonthTotals, value func(month.MonthTotals) decimal.Decimal,
	low bool) (month.MonthTotals, bool) {

	var best month.MonthTotals
	found := false
	for _, t := range months {
		switch {
		case !t.Confirmed:
		case !found, low && value(t).LessThan(value(best)), !low && value(t).GreaterThan(value(best)):
			best, found = t, true
		}
	}
	return best, found
}

// renderPlot draws the bars from a shared zero baseline that floats to make
// room for negative months. Those hang below it in the alert colour: a month
// that over-saved must not round away into an empty column.
func (m Model) renderPlot(bars []yearBar, barWidth, height int) string {
	high, low := 0.0, 0.0
	for _, b := range bars {
		v := b.value.InexactFloat64()
		high, low = math.Max(high, v), math.Min(low, v)
	}

	up := height
	if low < 0 {
		up = min(max(int(math.Round(float64(height)*high/(high-low))), 1), height-1)
		// The negative half is capped at half the plot. One month deep enough
		// to crush the other eleven into a row each says nothing the low
		// marker in the header has not already said, and costs every good
		// month its shape. Past the cap the two halves are on their own
		// scales, each filling its side.
		up = max(up, (height+1)/2)
	}

	rows := make([]string, height)
	for r := range rows {
		cells := make([]string, len(bars))
		for i, b := range bars {
			cells[i] = m.plotCell(b, r, up, height-up, high, low, barWidth)
		}
		rows[r] = strings.Join(cells, " ")
	}
	return strings.Join(rows, "\n")
}

func (m Model) plotCell(b yearBar, row, up, down int, high, low float64, width int) string {
	blank := strings.Repeat(" ", width)
	v := b.value.InexactFloat64()

	style := m.theme.Bright
	if b.current {
		style = m.theme.Accent
	}

	switch {
	case row < up && v > 0 && high > 0:
		// How much of this cell the bar reaches, counting up from the baseline.
		filled := v/high*float64(up) - float64(up-row-1)
		if filled >= 1 {
			return style.Render(strings.Repeat("█", width))
		}
		if filled > 0 {
			return style.Render(strings.Repeat(eighths[int(filled*8)], width))
		}
	case row >= up && v < 0 && low < 0:
		// Whole cells only: Unicode has no run of upper eighths to taper with.
		if v/low*float64(down) >= float64(row-up+1) {
			return m.theme.Alert.Render(strings.Repeat("█", width))
		}
	}
	return blank
}

func (m Model) renderAxis(bars []yearBar, barWidth, width int) string {
	labels := make([]string, len(bars))
	for i, b := range bars {
		style := m.theme.Muted
		switch {
		case b.current:
			style = m.theme.Accent
		case b.pending:
			style = m.theme.Muted.Faint(true)
		}
		labels[i] = style.Width(barWidth).Render(b.label)
	}
	return m.theme.Muted.Render(strings.Repeat("─", width)) + "\n" + strings.Join(labels, " ")
}

// ── category ranking ────────────────────────────────────────────────────────

func (m Model) catBarWidth(interior int) int {
	grid := 2*(interior+2) + yearGap
	room := m.contentWidth() - yearGap - grid - m.catChrome()
	return min(max(room, catBarMin), catBarMax)
}

// catAmountWidth measures the amount column against the figures the year
// actually produced. A total that outgrows the column widens it rather than
// overflowing the row, which the viewport would clip a character off.
func (m Model) catAmountWidth() int {
	width := catAmountMin
	for _, c := range m.year.Categories {
		width = max(width, len([]rune(formatAmount(c.Total))))
	}
	return width
}

// catChrome is every column of a category row except the bar, which takes
// whatever the boxes and the figures leave behind.
func (m Model) catChrome() int {
	return catGutterWidth + catNameWidth + 2 + m.catAmountWidth() + 2 + catShareWidth
}

func (m Model) catRowWidth(barWidth int) int { return m.catChrome() + barWidth }

func (m Model) renderCategoryBlock(barWidth int) string {
	title := m.theme.Title.Render("SPEND BY CATEGORY")
	if len(m.year.Categories) == 0 {
		return title + "\n\n" + m.theme.Muted.Render("no confirmed spending yet")
	}

	block := title + "\n" + m.categoryColumnHeader(barWidth) + "\n" + m.yearList.View()

	// The hint sits under the last row it is talking about. The list is
	// capped short of the box grid beside it, so the extra line costs the
	// card no height.
	if hint := m.categoryHint(); hint != "" {
		block += "\n" + strings.Repeat(" ", catGutterWidth) + hint
	}
	return block
}

func (m Model) categoryColumnHeader(barWidth int) string {
	right := lipgloss.NewStyle().Width(m.catAmountWidth()).Align(lipgloss.Right).Render("ARS") +
		strings.Repeat(" ", 2) +
		lipgloss.NewStyle().Width(catShareWidth).Align(lipgloss.Right).Render("SHARE")
	return m.theme.Muted.Render(strings.Repeat(" ", catGutterWidth+catNameWidth+barWidth+2) + right)
}

func (m Model) categoryHint() string {
	above, below := m.yearList.hidden()
	if above == 0 && below == 0 {
		return ""
	}
	var marks []string
	if above > 0 {
		marks = append(marks, fmt.Sprintf("↑ %d", above))
	}
	if below > 0 {
		marks = append(marks, fmt.Sprintf("↓ %d", below))
	}
	return m.theme.Muted.Render(strings.Join(marks, " · ") + " more")
}

// categoryRows ranks the year's spend. The bar scales against the largest
// category rather than the year's total, so the smallest slice is still
// visible; the share column is what says how much of the year it was.
//
// The cursor is drawn even though no row opens. With the list capped at five,
// panning a longer ranking moves nothing on screen until the cursor reaches
// the window's edge, and four dead keystrokes read as a broken key rather
// than as a list that has not scrolled yet.
func (m Model) categoryRows(barWidth, cursor int) []string {
	if len(m.year.Categories) == 0 {
		return nil
	}
	largest := m.year.Categories[0].Total

	rows := make([]string, len(m.year.Categories))
	for i, c := range m.year.Categories {
		gutter, nameStyle := strings.Repeat(" ", catGutterWidth), lipgloss.NewStyle()
		if i == cursor {
			gutter, nameStyle = m.theme.Accent.Render("> "), m.theme.Accent
		}
		name := nameStyle.Width(catNameWidth).
			Render(ansi.Truncate(c.Category.Name, catNameWidth-1, "…"))
		bar := categoryStyle(m.categories, c.Category.ID).Render(hbar(c.Total, largest, barWidth))
		amount := m.theme.Bright.Render(padLeft(formatAmount(c.Total), m.catAmountWidth()))
		share := m.theme.Muted.Render(padLeft(sharePercent(c.Total, m.year.Spent.ARS), catShareWidth))
		rows[i] = gutter + name + bar + "  " + amount + "  " + share
	}
	return rows
}

func sharePercent(part, whole decimal.Decimal) string {
	if whole.IsZero() {
		return "—"
	}
	return part.Div(whole).Mul(decimal.NewFromInt(100)).StringFixed(0) + "%"
}

// hbar is one horizontal bar padded out to width, tapering with an eighth
// block so two close categories do not land on the same length.
func hbar(value, largest decimal.Decimal, width int) string {
	if largest.IsZero() || !value.IsPositive() {
		return strings.Repeat(" ", width)
	}
	length, _ := value.Div(largest).Float64()
	length *= float64(width)

	full := min(int(length), width)
	bar := strings.Repeat("█", full)
	if frac := length - float64(full); full < width && frac > 0 {
		bar += leftEighths[int(frac*8)]
		full++
	}
	return bar + strings.Repeat(" ", width-full)
}

// padLeft right-aligns s by hand rather than through Style.Width, so a figure
// wider than its column overflows on one line instead of wrapping onto two.
func padLeft(s string, width int) string {
	return strings.Repeat(" ", max(width-len([]rune(s)), 0)) + s
}

// ── totals boxes ────────────────────────────────────────────────────────────

// yearInterior is the content width the four boxes share: as wide as the
// year's widest figure needs, growing toward yearBoxInterior only while the
// category block beside it keeps a usable bar.
func (m Model) yearInterior() int {
	required := railMinInterior
	for _, b := range m.yearBoxes() {
		required = max(required, len([]rune(b.title))+3)
		for _, l := range b.lines {
			required = max(required, l.width()+4)
		}
	}

	room := m.contentWidth() - yearGap - m.catChrome() - catBarMin
	afford := (room-yearGap)/2 - 2
	return min(max(yearBoxInterior, required), max(afford, required))
}

func (m Model) yearBoxes() []railBox {
	rated := m.year.Confirmed() > 0
	return []railBox{
		m.totalsBox("earned", m.year.Earned.ARS, m.year.Earned.USD, rated, shortfall{}),
		m.totalsBox("spent", m.year.Spent.ARS, m.year.Spent.USD, rated, shortfall{}),
		m.totalsBox("saved", m.year.Saved.ARS, m.year.Saved.USD, rated, shortfall{}),
		m.totalsBox("pocket", m.year.Pocket.ARS, m.year.Pocket.USD, rated,
			negativeShortfall(m.year.Pocket.ARS, m.year.Overspent())),
	}
}

func (m Model) renderBoxGrid(interior int) string {
	boxes := m.yearBoxes()
	rendered := make([]string, len(boxes))
	for i, b := range boxes {
		rendered[i] = m.renderBox(b, interior)
	}

	gap := strings.Repeat(" ", yearGap)
	top := lipgloss.JoinHorizontal(lipgloss.Top, rendered[0], gap, rendered[1])
	bottom := lipgloss.JoinHorizontal(lipgloss.Top, rendered[2], gap, rendered[3])
	return top + "\n\n" + bottom
}
