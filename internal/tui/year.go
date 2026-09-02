package tui

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/barchart"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
)

// trendSeries is the Year Trend chart's three-way selection, cycled with s
// — only one renders at a time.
type trendSeries int

const (
	seriesCash trendSeries = iota
	seriesInvested
	seriesPocket
)

func (s trendSeries) Label() string {
	switch s {
	case seriesInvested:
		return "Invested"
	case seriesPocket:
		return "Pocket money"
	default:
		return "Cash saved"
	}
}

func (s trendSeries) Currency() domain.Currency {
	if s == seriesPocket {
		return domain.ARS
	}
	return domain.USD
}

func nextTrendSeries(s trendSeries) trendSeries {
	return (s + 1) % 3
}

// trendValues is the selected series' resolved value for each period of
// the year, in the same order as y.Periods.
func trendValues(y month.Year, s trendSeries) []decimal.Decimal {
	values := make([]decimal.Decimal, len(y.NetWorth))
	for i := range values {
		switch s {
		case seriesInvested:
			values[i] = y.NetWorth[i].Invested
		case seriesPocket:
			values[i] = y.Leftover[i]
		default:
			values[i] = y.NetWorth[i].Cash
		}
	}
	return values
}

// yearLoadedMsg is the result of loadYear's Cmd, delivered back to Update
// once the database read completes.
type yearLoadedMsg struct {
	year month.Year
	err  error
}

// loadYear returns a Cmd that resolves calendarYear's grid, category totals
// and series off the Update loop.
func loadYear(db *sql.DB, calendarYear int) tea.Cmd {
	return func() tea.Msg {
		y, err := month.LoadYear(db, calendarYear)
		return yearLoadedMsg{year: y, err: err}
	}
}

// chartWidth keeps a chart inside the app's box regardless of terminal
// width, without ever making one so wide it swamps the layout.
func chartWidth(termWidth int) int {
	w := termWidth - 4
	if w > 60 {
		w = 60
	}
	if w < 10 {
		w = 10
	}
	return w
}

func (m Model) renderYear() string {
	var b strings.Builder
	b.WriteString(m.theme.Muted.Render(fmt.Sprintf("%s · %d", m.view.String(), m.period.Year())))

	if m.yearErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("failed to load: " + m.yearErr.Error()))
		return b.String()
	}
	if len(m.year.Periods) == 0 {
		b.WriteString("\n")
		b.WriteString(m.centerInBox(m.theme.Muted.Render("no concepts yet — add some in the Concepts view")))
		return b.String()
	}

	if m.yearDrillDown != nil {
		b.WriteString("\n")
		b.WriteString(m.centerInBox(m.renderYearDrillDown()))
		return b.String()
	}

	if len(m.year.Categories) > 0 {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Title.Render("Category breakdown"))
		b.WriteString("\n")
		b.WriteString(m.renderCategoryBarChart())
	}

	b.WriteString("\n\n")
	b.WriteString(m.theme.Title.Render("Trend"))
	values := trendValues(m.year, m.yearSeries)
	current := values[len(values)-1]
	fmt.Fprintf(&b, "  %s (current: %s %s)\n", m.yearSeries.Label(), current.StringFixed(2), m.yearSeries.Currency())
	b.WriteString(m.renderTrendChart(values))

	b.WriteString("\n\n")
	b.WriteString(m.theme.Title.Render("Concepts"))
	b.WriteString("\n")
	b.WriteString(m.renderYearConcepts())

	return b.String()
}

// renderTrendChart draws one vertical bar per month, real values rather
// than a sparkline's bare shape — vertical is ntcharts' default bar
// orientation, WithHorizontalBars is what opts into the category
// breakdown's horizontal layout instead.
func (m Model) renderTrendChart(values []decimal.Decimal) string {
	return m.renderMonthlyBars(values, m.yearSeries.Label(), m.theme.Chart)
}

// renderMonthlyBars draws one vertical bar per period in m.year.Periods —
// shared by the Trend chart (a net-worth series) and a concept's
// drill-down (its own twelve months), which differ only in the series name,
// its values, and its color.
func (m Model) renderMonthlyBars(values []decimal.Decimal, seriesName string, style lipgloss.Style) string {
	bars := make([]barchart.BarData, len(values))
	for i, v := range values {
		bars[i] = barchart.BarData{
			Label:  m.year.Periods[i].Month().String()[:3],
			Values: []barchart.BarValue{{Name: seriesName, Value: v.InexactFloat64(), Style: style}},
		}
	}
	bc := barchart.New(chartWidth(m.width), 12, barchart.WithDataSet(bars))
	bc.Draw()
	return bc.View()
}

// renderCategoryBarChart draws one horizontal bar per category, in the same
// order Categories() returns — sort_order, the app's one ordering rule —
// rather than resorting by magnitude.
func (m Model) renderCategoryBarChart() string {
	return m.renderCategoryBars(m.year.Categories, chartWidth(m.width))
}

// renderCategoryBars is the horizontal category breakdown shared by the
// Year view (all twelve months pooled) and the Finance pane (this period
// alone) — same bars, different totals and width.
func (m Model) renderCategoryBars(totals []month.CategoryTotal, width int) string {
	bars := make([]barchart.BarData, len(totals))
	for i, c := range totals {
		style := categoryStyle(m.categories, c.Category.ID)
		bars[i] = barchart.BarData{
			Label:  c.Category.Name,
			Values: []barchart.BarValue{{Name: c.Category.Name, Value: c.Total.InexactFloat64(), Style: style}},
		}
	}
	height := len(bars) * 2
	if height < 4 {
		height = 4
	}
	// WithDataSet must precede WithHorizontalBars: the origin recompute that
	// makes room for labels happens when horizontal mode is set, so it needs
	// the data already pushed to measure label widths against.
	bc := barchart.New(width, height, barchart.WithDataSet(bars), barchart.WithHorizontalBars())
	bc.Draw()
	return bc.View()
}

// renderYearConcepts is a category-grouped list, one row per concept that
// occurred anywhere in the year, showing its average resolved amount per
// occurrence — divided by the months it actually occurred in, not by 12, so
// a twice-a-year bonus reads as itself rather than a sixth of its size.
func (m Model) renderYearConcepts() string {
	if len(m.yearConceptRows()) == 0 {
		return m.theme.Muted.Render("no lines this year")
	}
	averages := conceptYearAverages(m.year.Months)
	concepts := yearConcepts(m.year.Months)

	var b strings.Builder
	idx := 0
	for _, cat := range m.categories {
		group := conceptsForCategory(concepts, cat.ID)
		if len(group) == 0 {
			continue
		}
		if idx > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(categoryStyle(m.categories, cat.ID).Bold(true).Render(cat.Name))
		for _, c := range group {
			b.WriteString("\n")
			b.WriteString(m.renderYearConceptRow(c, averages[c.ID], idx == m.yearConceptCursor))
			idx++
		}
	}
	return b.String()
}

func (m Model) renderYearConceptRow(c catalog.Concept, avg decimal.Decimal, selected bool) string {
	cursor := " "
	if selected {
		cursor = ">"
	}
	name := categoryStyle(m.categories, c.CategoryID).Render(fmt.Sprintf("%-20s", c.Name))
	return fmt.Sprintf("%s %s %s %12s", cursor, name, c.Currency, avg.StringFixed(2))
}

// conceptYearAverages sums each concept's resolved amounts across months
// and divides by the count it actually occurred in.
func conceptYearAverages(months []month.Month) map[int64]decimal.Decimal {
	sums := make(map[int64]decimal.Decimal)
	counts := make(map[int64]int)
	for _, mo := range months {
		for _, l := range mo.Lines {
			sums[l.Concept.ID] = sums[l.Concept.ID].Add(l.Amount)
			counts[l.Concept.ID]++
		}
	}
	averages := make(map[int64]decimal.Decimal, len(sums))
	for id, sum := range sums {
		if n := counts[id]; n > 0 {
			averages[id] = sum.Div(decimal.NewFromInt(int64(n)))
		}
	}
	return averages
}

// yearConceptRows is m.year's concepts in the same category-grouped order
// renderYearConcepts renders them, so cursor index and render index agree.
func (m Model) yearConceptRows() []catalog.Concept {
	concepts := yearConcepts(m.year.Months)
	var out []catalog.Concept
	for _, cat := range m.categories {
		out = append(out, conceptsForCategory(concepts, cat.ID)...)
	}
	return out
}

func (m Model) moveYearConceptCursor(delta int) int {
	return clampCursor(m.yearConceptCursor+delta, len(m.yearConceptRows()))
}

// cursorYearConcept reports the concept under the Year view's own cursor.
func (m Model) cursorYearConcept() (catalog.Concept, bool) {
	rows := m.yearConceptRows()
	if m.yearConceptCursor >= len(rows) {
		return catalog.Concept{}, false
	}
	return rows[m.yearConceptCursor], true
}

// startYearDrillDown opens the cursor's concept as a full-screen twelve-month
// chart, or is a no-op if the list is empty.
func (m Model) startYearDrillDown() (Model, tea.Cmd) {
	c, ok := m.cursorYearConcept()
	if !ok {
		return m, nil
	}
	m.yearDrillDown = &c
	return m, nil
}

// updateYearDrillDown is the drill-down screen's only interaction: esc
// returns to the concept list.
func (m Model) updateYearDrillDown(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.yearDrillDown = nil
	}
	return m, nil
}

// conceptTwelveMonths is one concept's resolved amount for each of months,
// zero where it didn't occur that month.
func conceptTwelveMonths(months []month.Month, conceptID int64) []decimal.Decimal {
	values := make([]decimal.Decimal, len(months))
	for i, mo := range months {
		for _, l := range mo.Lines {
			if l.Concept.ID == conceptID {
				values[i] = l.Amount
				break
			}
		}
	}
	return values
}

// renderYearDrillDown draws the cursor's concept across all twelve months of
// the year, the same vertical-bar shape as the Trend chart above.
func (m Model) renderYearDrillDown() string {
	c := *m.yearDrillDown
	values := conceptTwelveMonths(m.year.Months, c.ID)
	style := categoryStyle(m.categories, c.CategoryID)

	var b strings.Builder
	b.WriteString(m.theme.Title.Render(c.Name))
	b.WriteString("\n")
	b.WriteString(m.renderMonthlyBars(values, c.Name, style))
	return b.String()
}

// yearConcepts is the union of every concept resolved in any month of the
// year, ordered the same way the catalog is: sort_order, then name.
func yearConcepts(months []month.Month) []catalog.Concept {
	seen := make(map[int64]bool)
	var concepts []catalog.Concept
	for _, mo := range months {
		for _, l := range mo.Lines {
			if !seen[l.Concept.ID] {
				seen[l.Concept.ID] = true
				concepts = append(concepts, l.Concept)
			}
		}
	}
	sort.Slice(concepts, func(i, j int) bool {
		if concepts[i].SortOrder != concepts[j].SortOrder {
			return concepts[i].SortOrder < concepts[j].SortOrder
		}
		return concepts[i].Name < concepts[j].Name
	})
	return concepts
}
