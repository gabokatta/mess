package tui

import (
	"strconv"

	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/barchart"
)

// renderYear puts the whole year on one screen: no drill-down, no cycling.
func (m Model) renderYear() string {
	left := m.theme.Muted.Render("Year · ") + m.theme.Title.Render(strconv.Itoa(m.period.Year()))
	right := m.theme.Muted.Render("saved ") + formatAmount(m.year.SavedTotal) +
		m.theme.Muted.Render("   spent ") + formatAmount(m.year.SpentTotal)
	header := m.spread(left, right)

	if m.year.SavedTotal.IsZero() && m.year.SpentTotal.IsZero() {
		return header + "\n\n" + m.centerInBox(m.theme.Muted.Render("nothing confirmed this year yet"), 2)
	}

	available := m.bodyHeight(2)
	top := max((available-4)/2, 4)
	bottom := max(available-top-3, 4)
	half := chartWidth(m.width / 2)

	saved := groupStyle(2).Render("SAVED PER MONTH") + "\n" + m.renderSavedChart(half, top)
	spent := groupStyle(5).Render("SPENT PER MONTH") + "\n" + m.renderSpentChart(half, top)
	columns := lipgloss.JoinHorizontal(lipgloss.Top, saved, "  ", spent)

	return header + "\n\n" + columns + "\n\n" +
		groupStyle(4).Render("SPEND BY CATEGORY") + "\n" + m.renderCategoryChart(bottom)
}

// renderSavedChart stacks each month's bar by the concept that put the
// money away, so one saving reads apart from another.
func (m Model) renderSavedChart(width, height int) string {
	bars := make([]barchart.BarData, len(m.year.Periods))
	for i, p := range m.year.Periods {
		values := make([]barchart.BarValue, 0, len(m.year.SavingConcepts))
		for _, c := range m.year.SavingConcepts {
			amount := m.year.Saved[i][c.ID]
			if amount.IsZero() {
				continue
			}
			values = append(values, barchart.BarValue{
				Name:  c.Name,
				Value: amount.InexactFloat64(),
				Style: categoryStyle(m.categories, c.CategoryID),
			})
		}
		bars[i] = barchart.BarData{Label: shortMonth(p.Month()), Values: values}
	}
	return drawBars(bars, width, height, false)
}

func (m Model) renderSpentChart(width, height int) string {
	bars := make([]barchart.BarData, len(m.year.Periods))
	for i, p := range m.year.Periods {
		style := m.theme.Muted
		if p.Equal(m.period) {
			style = m.theme.Accent
		}
		bars[i] = barchart.BarData{
			Label:  shortMonth(p.Month()),
			Values: []barchart.BarValue{{Name: "spent", Value: m.year.Spent[i].InexactFloat64(), Style: style}},
		}
	}
	return drawBars(bars, width, height, false)
}

func (m Model) renderCategoryChart(height int) string {
	if len(m.year.Categories) == 0 {
		return m.theme.Muted.Render("no confirmed spending yet")
	}
	bars := make([]barchart.BarData, len(m.year.Categories))
	for i, c := range m.year.Categories {
		bars[i] = barchart.BarData{
			Label: c.Category.Name,
			Values: []barchart.BarValue{{
				Name:  c.Category.Name,
				Value: c.Total.InexactFloat64(),
				Style: categoryStyle(m.categories, c.Category.ID),
			}},
		}
	}
	return drawBars(bars, chartWidth(m.width), min(len(bars)*2+1, height), true)
}
