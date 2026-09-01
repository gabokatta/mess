package tui

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/ntcharts/v2/barchart"
	"github.com/NimbleMarkets/ntcharts/v2/sparkline"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/month"
)

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
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("no concepts yet — add some in the Concepts view"))
		return b.String()
	}

	b.WriteString("\n\n")
	b.WriteString(m.theme.Title.Render("Net worth"))
	b.WriteString(fmt.Sprintf("  (current: %s)\n", m.year.NetWorth[len(m.year.NetWorth)-1].Total().StringFixed(2)))
	b.WriteString(m.renderSparkline(netWorthSeries(m.year)))

	b.WriteString("\n\n")
	b.WriteString(m.theme.Title.Render("Leftover pesos"))
	b.WriteString(fmt.Sprintf("  (current: %s)\n", m.year.Leftover[len(m.year.Leftover)-1].StringFixed(2)))
	b.WriteString(m.renderSparkline(leftoverSeries(m.year)))

	if len(m.year.Categories) > 0 {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Title.Render("Category breakdown"))
		b.WriteString("\n")
		b.WriteString(m.renderCategoryBarChart())
	}

	b.WriteString("\n\n")
	b.WriteString(m.theme.Title.Render("Grid"))
	b.WriteString("\n")
	b.WriteString(m.renderGrid())

	return b.String()
}

func netWorthSeries(y month.Year) []float64 {
	series := make([]float64, len(y.NetWorth))
	for i, nw := range y.NetWorth {
		series[i] = nw.Total().InexactFloat64()
	}
	return series
}

func leftoverSeries(y month.Year) []float64 {
	series := make([]float64, len(y.Leftover))
	for i, v := range y.Leftover {
		series[i] = v.InexactFloat64()
	}
	return series
}

func (m Model) renderSparkline(data []float64) string {
	sl := sparkline.New(chartWidth(m.width), 5, sparkline.WithStyle(m.theme.Chart))
	sl.PushAll(data)
	sl.Draw()
	return sl.View()
}

// renderCategoryBarChart draws one horizontal bar per category, in the same
// order Categories() returns — sort_order, the app's one ordering rule —
// rather than resorting by magnitude.
func (m Model) renderCategoryBarChart() string {
	bars := make([]barchart.BarData, len(m.year.Categories))
	for i, c := range m.year.Categories {
		bars[i] = barchart.BarData{
			Label:  c.Category.Name,
			Values: []barchart.BarValue{{Name: c.Category.Name, Value: c.Total.InexactFloat64(), Style: m.theme.Chart}},
		}
	}
	height := len(bars) * 2
	if height < 4 {
		height = 4
	}
	// WithDataSet must precede WithHorizontalBars: the origin recompute that
	// makes room for labels happens when horizontal mode is set, so it needs
	// the data already pushed to measure label widths against.
	bc := barchart.New(chartWidth(m.width), height, barchart.WithDataSet(bars), barchart.WithHorizontalBars())
	bc.Draw()
	return bc.View()
}

// renderGrid draws one row per concept that occurred anywhere in the year,
// labeled with its currency since ARS and USD concepts share the grid, one
// column per period, blank where the concept didn't occur that month.
func (m Model) renderGrid() string {
	concepts := yearConcepts(m.year.Months)
	if len(concepts) == 0 {
		return m.theme.Muted.Render("no lines this year")
	}
	amounts := gridAmounts(m.year.Months, concepts)

	var b strings.Builder
	fmt.Fprintf(&b, "%-20s", "")
	for _, p := range m.year.Periods {
		fmt.Fprintf(&b, " %10s", p.String())
	}
	for i, c := range concepts {
		b.WriteString("\n")
		fmt.Fprintf(&b, "%-16s %-3s", c.Name, c.Currency)
		for _, cell := range amounts[i] {
			display := "·"
			if cell.present {
				display = cell.amount.StringFixed(2)
			}
			fmt.Fprintf(&b, " %10s", display)
		}
	}
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

// gridCell is one concept's resolved amount for one period, distinguishing
// a real zero amount from the concept not occurring that month at all.
type gridCell struct {
	amount  decimal.Decimal
	present bool
}

// gridAmounts is concepts x months — outer index is the concept, in
// yearConcepts' order.
func gridAmounts(months []month.Month, concepts []catalog.Concept) [][]gridCell {
	rows := make([][]gridCell, len(concepts))
	for i, c := range concepts {
		row := make([]gridCell, len(months))
		for j, mo := range months {
			for _, l := range mo.Lines {
				if l.Concept.ID == c.ID {
					row[j] = gridCell{amount: l.Amount, present: true}
					break
				}
			}
		}
		rows[i] = row
	}
	return rows
}
