package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/gabokatta/mess/internal/month"
)

// renderFinanceColumns is the Finance pane's two-column layout: the
// concept-line list on the left, this period's expense-by-category chart on
// the right — the same breakdown the Year view sums across all twelve
// months, scoped here to just this one period.
func (m Model) renderFinanceColumns() string {
	left := m.renderFinanceLines()
	right := m.renderFinanceChart()
	if right == "" {
		return left
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "    ", right)
}

// renderFinanceChart draws one horizontal bar per category with an expense
// in this period, colored per category, empty when there's nothing to show
// yet (a fresh period, or an Income-only one).
func (m Model) renderFinanceChart() string {
	totals := month.CategoryTotals(m.categories, m.lines)
	if len(totals) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(m.theme.Title.Render("Spending"))
	b.WriteString("\n")
	b.WriteString(m.renderCategoryBars(totals, chartWidth(m.width/2)))
	return b.String()
}
