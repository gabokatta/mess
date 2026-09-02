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

	rent := catalog.Concept{Name: "Alquiler", CategoryID: 1, Kind: catalog.Expense, Currency: domain.ARS}
	updated, _ := m.Update(monthLoadedMsg{
		lines: []month.Line{{Concept: rent, Amount: amountFor(t, "785000")}},
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

func TestFinancePaneHidesTheChoresList(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40

	trash := catalog.Chore{Name: "Sacar la basura"}
	updated, _ := m.Update(monthLoadedMsg{chores: []month.ChoreLine{{Chore: trash}}})
	m = updated.(Model)
	content := m.View().Content

	if strings.Contains(content, "Sacar la basura") {
		t.Errorf("content = %q, want the Finance pane to hide Chores' own list", content)
	}
}
