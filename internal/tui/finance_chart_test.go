package tui

import (
	"strings"
	"testing"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
)

func TestFinancePaneRendersCategoryChartBesideTheLineList(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40
	m.categories = []catalog.Category{{ID: 1, Name: "Home", SortOrder: 0}}

	rent := catalog.Concept{Name: "Alquiler", CategoryID: 1, Kind: catalog.Expense, Money: &catalog.MoneyDetails{Currency: domain.ARS}}
	updated, _ := m.Update(monthLoadedMsg{
		lines: []month.Line{{Concept: rent, Money: &month.LineMoney{Amount: amountFor(t, "785000")}}},
	})
	m = updated.(Model)
	content := m.View().Content

	for _, want := range []string{"Spending", "Home", "Alquiler"} {
		if !strings.Contains(content, want) {
			t.Errorf("month view content missing %q (Finance pane's chart or line list):\n%s", want, content)
		}
	}
}

func TestFinancePaneHasNoChartWithNoExpenses(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40

	if got := m.renderFinanceChart(); got != "" {
		t.Errorf("renderFinanceChart() = %q, want empty with no expense lines", got)
	}
}

func TestSpendingChartExcludesChoreLines(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40

	trash := catalog.Concept{Name: "Sacar la basura", Kind: catalog.Chore}
	updated, _ := m.Update(monthLoadedMsg{lines: []month.Line{{Concept: trash}}})
	m = updated.(Model)

	if got := m.renderFinanceChart(); got != "" {
		t.Errorf("renderFinanceChart() = %q, want empty — a chore has no category spend to chart", got)
	}
}
