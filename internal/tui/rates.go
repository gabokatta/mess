package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
	"github.com/gabokatta/mess/internal/rates"
)

// Measure the rate column against its values; other columns have fixed widths.
const (
	rateMonthWidth  = 5
	rateAmountMin   = 9
	rateDeltaWidth  = 7
	rateSourceWidth = 9
	rateHouseWidth  = 8
	rateSpreadWidth = 4
)

// Keep the pane width stable as the selected rate changes.
const (
	ratePaneWidth = 35
	ratePaneLabel = 10
	ratesGap      = 6
)

// rateStatus is where a period's rate came from, as the word the SOURCE column
// carries. It is derived from the resolved rate and today, never stored.
type rateStatus string

const (
	statusLive      rateStatus = "live"
	statusClose     rateStatus = "close"
	statusManual    rateStatus = "manual"
	statusInherited rateStatus = "inherited"
	statusPending   rateStatus = "pending"
)

// House is present for stored closes and live quotes, absent for manual and inherited rates.
type rateRow struct {
	period   domain.Period
	rate     month.Rate
	status   rateStatus
	house    *domain.FxHouse
	delta    decimal.Decimal
	hasDelta bool
	mismatch bool
	current  bool
}

// Inherited rates have no new measurement and do not contribute a monthly delta.
func (r rateRow) measured() bool {
	return r.status == statusClose || r.status == statusManual || r.status == statusLive
}

var hundred = decimal.NewFromInt(100)

// Resolve table rows once per call, with future months left pending.
func (m Model) rateRows() []rateRow {
	fx := m.fx()
	stored := make(map[domain.Period]catalog.FxRate, len(m.stored))
	for _, r := range m.stored {
		stored[r.Period] = r
	}

	rows := make([]rateRow, 12)
	previous := lastRateBefore(m.stored, domain.NewPeriod(m.period.Year(), time.January))
	for i := range rows {
		p := domain.NewPeriod(m.period.Year(), time.Month(i+1))
		row := rateRow{period: p, current: p.Equal(m.period)}

		if !p.After(m.today) {
			row.rate = fx.At(p)
		}
		row.status = statusOf(row.rate)

		switch row.status {
		case statusClose:
			row.house = stored[p].House
		case statusLive:
			house := m.settings.FxHouse
			row.house = &house
		}
		row.mismatch = row.house != nil && *row.house != m.settings.FxHouse

		if row.measured() && !row.rate.Value.IsZero() {
			if !previous.IsZero() {
				row.delta = row.rate.Value.Sub(previous).Div(previous).Mul(hundred)
				row.hasDelta = true
			}
			previous = row.rate.Value
		}
		rows[i] = row
	}
	return rows
}

// January measures against the December before it, which is not on this table.
func lastRateBefore(stored []catalog.FxRate, p domain.Period) decimal.Decimal {
	var last decimal.Decimal
	for _, r := range stored {
		if !r.Period.Before(p) {
			break
		}
		last = r.Value
	}
	return last
}

func statusOf(rate month.Rate) rateStatus {
	switch rate.Origin {
	case month.RateLive:
		return statusLive
	case month.RateClose:
		return statusClose
	case month.RateManual:
		return statusManual
	case month.RateInherited:
		return statusInherited
	default:
		return statusPending
	}
}

func (m Model) cursorRate() rateRow {
	rows := m.rateRows()
	return rows[clamp(m.ratesList.cursor, len(rows))]
}

func (m Model) handleRatesKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "h":
		next := rates.Houses[(houseIndex(m.settings.FxHouse)+1)%len(rates.Houses)]
		return m, write(func() error { return catalog.SetFxHouse(m.db, next) })
	case "e":
		row := m.cursorRate()
		// A projection is not a rate. A manual row in December would become
		// the value every month after it inherits.
		if row.status == statusPending && row.period.After(m.today) {
			return m, nil
		}
		return m.openModal(m.manualRateForm(row.period))
	case "d":
		row := m.cursorRate()
		// An inherited or unreached month stores nothing of its own, so there
		// is nothing here to take back.
		if !row.measured() || row.status == statusLive {
			return m, nil
		}
		return m, clearRate(m.db, m.client, row.period, m.today)
	}
	return m, nil
}

func houseIndex(house domain.FxHouse) int {
	for i, h := range rates.Houses {
		if h == house {
			return i
		}
	}
	return 0
}

func (m Model) manualRateForm(period domain.Period) *form {
	value := m.fx().At(period).Value.StringFixed(2)

	return newForm(m.theme, m.width, m.height,
		[]*huh.Group{
			huh.NewGroup(
				huh.NewInput().Title("Rate for " + period.String()).
					Description("pesos per dollar").
					Value(&value).Validate(validateDecimal),
			).Title("Set rate by hand"),
		},
		func() tea.Cmd {
			return write(func() error {
				parsed, err := decimal.NewFromString(value)
				if err != nil {
					return err
				}
				return catalog.SetManualFxRate(m.db, period, parsed)
			})
		})
}

func (m Model) rateAmountWidth() int {
	width := rateAmountMin
	for _, r := range m.rateRows() {
		if r.rate.OK() {
			width = max(width, len([]rune(formatAmount(r.rate.Value))))
		}
	}
	return width
}

func (m Model) ratesTableWidth() int {
	return gutterWidth + rateMonthWidth + colGap + m.rateAmountWidth() + colGap +
		rateDeltaWidth + colGap + rateSourceWidth + colGap + rateHouseWidth
}

func (m Model) rateColumnHeader() string {
	return m.theme.Muted.Render(strings.Repeat(" ", gutterWidth) +
		padRight("MONTH", rateMonthWidth) + strings.Repeat(" ", colGap) +
		padLeft("RATE", m.rateAmountWidth()) + strings.Repeat(" ", colGap) +
		padLeft("Δ", rateDeltaWidth) + strings.Repeat(" ", colGap) +
		padRight("SOURCE", rateSourceWidth) + strings.Repeat(" ", colGap) +
		"HOUSE")
}

func (m Model) rateTableRows() []string {
	amount := m.rateAmountWidth()
	source := m.rateRows()
	rows := make([]string, len(source))
	for i, r := range source {
		rows[i] = m.rateTableRow(r, i == m.ratesList.cursor, amount)
	}
	return rows
}

func (m Model) rateTableRow(r rateRow, onCursor bool, amount int) string {
	gutter := strings.Repeat(" ", gutterWidth)
	if onCursor {
		gutter = m.theme.Accent.Render("> ")
	}

	// Underline only the month text, keeping padding outside the style.
	label := m.theme.Bright
	if r.current {
		label = label.Underline(true)
	}
	month := label.Render(shortMonth(r.period.Month())) +
		strings.Repeat(" ", rateMonthWidth-len(shortMonth(r.period.Month())))

	value, delta := m.theme.Muted.Render(padLeft(emDash, amount)), strings.Repeat(" ", rateDeltaWidth)
	if r.rate.OK() {
		value = m.theme.Bright.Render(padLeft(formatAmount(r.rate.Value), amount))
	}
	if r.hasDelta {
		delta = m.theme.Bright.Render(padLeft(signedPercent(r.delta), rateDeltaWidth))
	}

	source := m.theme.Bright
	if r.status == statusInherited || r.status == statusPending {
		source = m.theme.Muted
	}

	house, houseStyle := emDash, m.theme.Muted
	if r.house != nil {
		house = strings.ToLower(r.house.String())
		if r.mismatch {
			houseStyle = m.theme.Alert
		}
	}

	gap := strings.Repeat(" ", colGap)
	return gutter + month + gap +
		value + gap + delta + gap +
		source.Render(padRight(string(r.status), rateSourceWidth)) + gap +
		houseStyle.Render(house)
}

// Include the sign and use the same decimal comma as other displayed amounts.
func signedPercent(d decimal.Decimal) string {
	s := strings.Replace(d.StringFixed(1), ".", ",", 1) + "%"
	if !d.IsNegative() {
		s = "+" + s
	}
	return s
}

func padRight(s string, width int) string {
	return s + strings.Repeat(" ", max(width-len([]rune(s)), 0))
}

const emDash = "—"

func (m Model) renderRates() string {
	title := m.rateHeader()

	table := m.rateColumnHeader() + "\n" +
		m.ratesList.View() + m.scrollHint(m.ratesList, gutterWidth)
	sidebar := m.ratePane(m.cursorRate()) + "\n\n" + m.renderHouses()
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		table, strings.Repeat(" ", ratesGap), sidebar)

	card := title + "\n\n" + body + "\n\n" + m.renderDeltaChart(m.ratesCardWidth(), m.ratesPlot())

	top := max(0, (m.bodyHeight(0)-lipgloss.Height(card))/2)
	left := max(0, (m.contentWidth()-m.ratesCardWidth())/2)
	return lipgloss.NewStyle().MarginLeft(left).Render(strings.Repeat("\n", top) + card)
}

func (m Model) ratesCardWidth() int { return m.ratesTableWidth() + ratesGap + ratePaneWidth }

func (m Model) rateHeader() string {
	counts := make(map[rateStatus]int, 5)
	mismatched := 0
	for _, r := range m.rateRows() {
		counts[r.status]++
		if r.mismatch {
			mismatched++
		}
	}

	var parts []string
	for _, status := range []rateStatus{statusClose, statusManual, statusLive, statusInherited, statusPending} {
		if n := counts[status]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", status, n))
		}
	}
	right := m.theme.Muted.Render(strings.Join(parts, " · "))
	if mismatched > 0 {
		right += m.theme.Muted.Render(" · ") +
			m.theme.Alert.Render(fmt.Sprintf("%d at another house", mismatched))
	}

	left := m.theme.Title.Render(strconv.Itoa(m.period.Year()))
	gap := max(m.ratesCardWidth()-lipgloss.Width(left)-lipgloss.Width(right), 1)
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) renderHouses() string {
	rows := make([]string, 0, len(rates.Houses)+2)
	rows = append(rows, m.theme.Title.Render("HOUSES"))

	for _, house := range rates.Houses {
		name := m.theme.Bright.Render(padRight(strings.ToLower(house.String()), rateHouseWidth))

		sell, spread := m.theme.Muted.Render(padLeft(emDash, rateAmountMin)), strings.Repeat(" ", rateSpreadWidth)
		if q, ok := quoteFor(m.quotes, house); ok {
			sell = m.theme.Bright.Render(padLeft(formatAmount(q.Sell), rateAmountMin))
			spread = m.theme.Muted.Render(padLeft(formatAmount(q.Sell.Sub(q.Buy)), rateSpreadWidth))
		}

		using := ""
		if house == m.settings.FxHouse {
			using = m.theme.Accent.Render("using")
		}
		rows = append(rows, strings.Repeat(" ", gutterWidth)+name+strings.Repeat(" ", colGap)+
			sell+strings.Repeat(" ", colGap)+spread+strings.Repeat(" ", colGap)+using)
	}

	// The market is shut at weekends and on holidays, so a quote is the last
	// day it traded, which is not always today. The date says which.
	quoted := "no quote today"
	if len(m.quotes) > 0 {
		quoted = "quoted " + m.quotes[0].Date.Format(time.DateOnly)
	}
	return strings.Join(append(rows, strings.Repeat(" ", gutterWidth)+m.theme.Muted.Render(quoted)), "\n")
}

func quoteFor(quotes []rates.Quote, house domain.FxHouse) (rates.Quote, bool) {
	for _, q := range quotes {
		if q.House == house {
			return q, true
		}
	}
	return rates.Quote{}, false
}

func (m Model) ratePane(r rateRow) string {
	lines := []string{
		m.theme.Title.Render(strings.ToUpper(r.period.Month().String()) + " " + strconv.Itoa(r.period.Year())),
		"",
		m.paneLine("Rate", rateValueText(r)),
		m.paneLine("Source", string(r.status)),
	}

	house := emDash
	if r.house != nil {
		house = strings.ToLower(r.house.String())
	}
	lines = append(lines, m.paneLine("House", house))

	// Only the live row has a day behind it. A stored close records no date,
	// so there is nothing here that could be checked against anything.
	switch {
	case r.status == statusLive && len(m.quotes) > 0:
		lines = append(lines, m.paneLine("Quoted", m.quotes[0].Date.Format(time.DateOnly)))
	case r.status == statusInherited:
		lines = append(lines, m.paneLine("From", r.rate.From.String()))
	default:
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func rateValueText(r rateRow) string {
	if !r.rate.OK() {
		return emDash
	}
	return formatAmount(r.rate.Value)
}

func (m Model) paneLine(label, value string) string {
	return m.theme.Muted.Render(padRight(label, ratePaneLabel)) + m.theme.Bright.Render(value)
}

func (m Model) rateDeltaBars() []yearBar {
	rows := m.rateRows()
	bars := make([]yearBar, len(rows))
	for i, r := range rows {
		bars[i] = yearBar{
			label:   shortMonth(r.period.Month()),
			value:   r.delta,
			current: r.current,
			pending: !r.hasDelta,
		}
	}
	return bars
}

// Compare absolute changes so a decrease can be the year's largest move.
func (m Model) steepestMove() (rateRow, bool) {
	var best rateRow
	found := false
	for _, r := range m.rateRows() {
		if r.hasDelta && (!found || r.delta.Abs().GreaterThan(best.delta.Abs())) {
			best, found = r, true
		}
	}
	return best, found
}

// Compare the first and last available rates, even when January has none.
func (m Model) yearDrift() (decimal.Decimal, bool) {
	var first, last decimal.Decimal
	for _, r := range m.rateRows() {
		if !r.rate.OK() || r.rate.Value.IsZero() {
			continue
		}
		if first.IsZero() {
			first = r.rate.Value
		}
		last = r.rate.Value
	}
	if first.IsZero() || first.Equal(last) {
		return decimal.Decimal{}, false
	}
	return last.Sub(first).Div(first).Mul(hundred), true
}

func (m Model) renderDeltaChart(card, plot int) string {
	barWidth := min(max((card-11)/12, yearBarMin), yearBarMax)
	width := 12*(barWidth+1) - 1
	bars := m.rateDeltaBars()

	head := m.theme.Title.Render("MONTH ON MONTH") + "\n" + m.deltaChartNote(width)
	return head + "\n\n" + m.renderPlot(bars, barWidth, plot) + "\n" + m.renderAxis(bars, barWidth, width)
}

func (m Model) deltaChartNote(room int) string {
	var left, right string
	if steepest, ok := m.steepestMove(); ok {
		left = "steepest " + shortMonth(steepest.period.Month()) + " · " + signedPercent(steepest.delta)
	}
	if drift, ok := m.yearDrift(); ok {
		right = "year " + signedPercent(drift)
	}

	gap := room - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return m.theme.Muted.Render(left + strings.Repeat(" ", gap) + right)
}

func shortMonth(m time.Month) string { return strings.ToLower(m.String()[:3]) }

// ratesMonths is the table's height in rows: a year, always, whether or not
// anybody has reached the end of it.
const ratesMonths = 12

func (m Model) ratesListHeight() int { return m.bodyHeight(3) }

func (m Model) ratesPlot() int {
	const chrome = 1 + ratesMonths + 1 + 5
	return min(max(m.bodyHeight(2)-chrome, yearPlotMin), yearPlotHeight)
}
