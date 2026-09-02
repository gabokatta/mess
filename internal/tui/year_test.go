package tui

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
)

func TestYearViewReportsLoadError(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 120, 50
	m.view = viewYear

	updated, _ := m.Update(yearLoadedMsg{err: sql.ErrConnDone})
	m = updated.(Model)
	content := m.View().Content

	if !strings.Contains(content, "failed to load") {
		t.Errorf("year view content = %q, want it to surface the load error", content)
	}
}

func TestYearViewRendersGridCategoriesAndSeries(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 120, 50
	m.view = viewYear

	jan := domain.NewPeriod(2026, time.January)
	feb := domain.NewPeriod(2026, time.February)
	rent := catalog.Concept{Name: "Alquiler", Kind: catalog.Expense, Currency: domain.ARS}
	servicios := catalog.Category{Name: "Servicios"}

	year := month.Year{
		Periods: []domain.Period{jan, feb},
		Months: []month.Month{
			{Lines: []month.Line{{Concept: rent, Amount: amountFor(t, "785000")}}},
			{Lines: []month.Line{{Concept: rent, Amount: amountFor(t, "785000")}}},
		},
		Categories: []month.CategoryTotal{{Category: servicios, Total: amountFor(t, "1570000")}},
		NetWorth:   []month.NetWorth{{Cash: amountFor(t, "1000")}, {Cash: amountFor(t, "1100")}},
		Leftover:   []decimal.Decimal{amountFor(t, "5000"), amountFor(t, "6000")},
	}

	updated, _ := m.Update(yearLoadedMsg{year: year})
	m = updated.(Model)
	content := m.View().Content

	for _, want := range []string{"Alquiler", "ARS", "Servicios", jan.String(), feb.String(), "Net worth", "Leftover pesos", "1100.00"} {
		if !strings.Contains(content, want) {
			t.Errorf("year view content missing %q:\n%s", want, content)
		}
	}
}

func TestYearViewRendersEmptyState(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 120, 50
	m.view = viewYear

	updated, _ := m.Update(yearLoadedMsg{year: month.Year{}})
	m = updated.(Model)
	content := m.View().Content

	if !strings.Contains(content, "no concepts yet") {
		t.Errorf("year view content = %q, want the empty-state message", content)
	}
}
