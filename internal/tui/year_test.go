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

func TestYearViewRendersConceptsCategoriesAndSeries(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 120, 50
	m.view = viewYear

	jan := domain.NewPeriod(2026, time.January)
	feb := domain.NewPeriod(2026, time.February)
	servicios := catalog.Category{Name: "Servicios"}
	rent := catalog.Concept{Name: "Alquiler", CategoryID: servicios.ID, Kind: catalog.Expense, Money: &catalog.MoneyDetails{Currency: domain.ARS}}
	m.categories = []catalog.Category{servicios}

	year := month.Year{
		Periods: []domain.Period{jan, feb},
		Months: []month.Month{
			{Lines: []month.Line{{Concept: rent, Money: &month.LineMoney{Amount: amountFor(t, "785000")}}}},
			{Lines: []month.Line{{Concept: rent, Money: &month.LineMoney{Amount: amountFor(t, "785000")}}}},
		},
		Categories: []month.CategoryTotal{{Category: servicios, Total: amountFor(t, "1570000")}},
		NetWorth:   []month.NetWorth{{Cash: amountFor(t, "1000")}, {Cash: amountFor(t, "1100")}},
		Leftover:   []decimal.Decimal{amountFor(t, "5000"), amountFor(t, "6000")},
	}

	updated, _ := m.Update(yearLoadedMsg{year: year})
	m = updated.(Model)
	content := m.View().Content

	for _, want := range []string{"Alquiler", "ARS", "Servicios", "785000.00", "Trend", "Cash saved", "1100.00"} {
		if !strings.Contains(content, want) {
			t.Errorf("year view content missing %q:\n%s", want, content)
		}
	}

	breakdownIdx := strings.Index(content, "Category breakdown")
	trendIdx := strings.Index(content, "Trend")
	conceptsIdx := strings.Index(content, "Concepts")
	if breakdownIdx == -1 || trendIdx == -1 || conceptsIdx == -1 {
		t.Fatalf("expected all three sections present:\n%s", content)
	}
	if !(breakdownIdx < trendIdx && trendIdx < conceptsIdx) {
		t.Errorf("section order = breakdown@%d, trend@%d, concepts@%d, want Category breakdown, then Trend, then Concepts", breakdownIdx, trendIdx, conceptsIdx)
	}
}

func TestYearConceptsListShowsAveragePerOccurrenceNotFlatByTwelve(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 120, 50
	m.view = viewYear

	bonus := catalog.Concept{Name: "Aguinaldo", Kind: catalog.Income, Money: &catalog.MoneyDetails{Currency: domain.ARS}}
	m.categories = []catalog.Category{{}}

	year := month.Year{
		Periods: []domain.Period{domain.NewPeriod(2026, time.June), domain.NewPeriod(2026, time.December)},
		Months: []month.Month{
			{Lines: []month.Line{{Concept: bonus, Money: &month.LineMoney{Amount: amountFor(t, "500000")}}}},
			{Lines: []month.Line{{Concept: bonus, Money: &month.LineMoney{Amount: amountFor(t, "500000")}}}},
		},
		NetWorth: []month.NetWorth{{}, {}},
		Leftover: []decimal.Decimal{{}, {}},
	}

	updated, _ := m.Update(yearLoadedMsg{year: year})
	m = updated.(Model)
	content := m.View().Content

	if !strings.Contains(content, "500000.00") {
		t.Errorf("content = %q, want the average per occurrence (500000.00), not the flat-by-12 figure", content)
	}
	if strings.Contains(content, "83333") {
		t.Errorf("content = %q, want it not divided by 12", content)
	}
}

func TestEnterOpensDrillDownAndEscReturnsToList(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 120, 50
	m.view = viewYear
	m.categories = []catalog.Category{{}}

	rent := catalog.Concept{ID: 1, Name: "Alquiler", Kind: catalog.Expense, Money: &catalog.MoneyDetails{Currency: domain.ARS}}
	year := month.Year{
		Periods: []domain.Period{domain.NewPeriod(2026, time.January), domain.NewPeriod(2026, time.February)},
		Months: []month.Month{
			{Lines: []month.Line{{Concept: rent, Money: &month.LineMoney{Amount: amountFor(t, "785000")}}}},
			{Lines: []month.Line{{Concept: rent, Money: &month.LineMoney{Amount: amountFor(t, "800000")}}}},
		},
		NetWorth: []month.NetWorth{{}, {}},
		Leftover: []decimal.Decimal{{}, {}},
	}
	updated, _ := m.Update(yearLoadedMsg{year: year})
	m = updated.(Model)

	updated, _ = m.Update(keyEnter())
	m = updated.(Model)
	if m.yearDrillDown == nil || m.yearDrillDown.Name != "Alquiler" {
		t.Fatalf("yearDrillDown = %v, want Alquiler opened", m.yearDrillDown)
	}
	content := m.View().Content
	for _, want := range []string{"Alquiler", "Jan", "Feb"} {
		if !strings.Contains(content, want) {
			t.Errorf("drill-down content missing %q:\n%s", want, content)
		}
	}
	if strings.Contains(content, "Trend") {
		t.Errorf("content = %q, want the drill-down to replace the normal Year sections", content)
	}

	updated, _ = m.Update(keyEsc())
	m = updated.(Model)
	if m.yearDrillDown != nil {
		t.Error("esc should close the drill-down")
	}
	if !strings.Contains(m.View().Content, "Trend") {
		t.Error("esc should return to the normal Year sections")
	}
}

func TestJKMovesTheYearConceptCursor(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 120, 50
	m.view = viewYear
	m.categories = []catalog.Category{{}}

	a := catalog.Concept{ID: 1, Name: "Alquiler", Kind: catalog.Expense, Money: &catalog.MoneyDetails{Currency: domain.ARS}}
	i := catalog.Concept{ID: 2, Name: "Internet", Kind: catalog.Expense, Money: &catalog.MoneyDetails{Currency: domain.ARS}}
	year := month.Year{
		Periods: []domain.Period{domain.NewPeriod(2026, time.January)},
		Months: []month.Month{{Lines: []month.Line{
			{Concept: a, Money: &month.LineMoney{Amount: amountFor(t, "785000")}},
			{Concept: i, Money: &month.LineMoney{Amount: amountFor(t, "15000")}},
		}}},
		NetWorth: []month.NetWorth{{}},
		Leftover: []decimal.Decimal{{}},
	}
	updated, _ := m.Update(yearLoadedMsg{year: year})
	m = updated.(Model)
	if m.yearConceptCursor != 0 {
		t.Fatalf("yearConceptCursor = %d, want 0 at load", m.yearConceptCursor)
	}

	updated, _ = m.Update(key("j"))
	m = updated.(Model)
	if m.yearConceptCursor != 1 {
		t.Fatalf("yearConceptCursor = %d, want 1 after j", m.yearConceptCursor)
	}
	c, ok := m.cursorYearConcept()
	if !ok || c.Name != "Internet" {
		t.Fatalf("cursorYearConcept() = %+v, %v, want Internet", c, ok)
	}
}

func TestSKeyCyclesTheYearTrendSeries(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 120, 50
	m.view = viewYear

	year := month.Year{
		Periods:  []domain.Period{domain.NewPeriod(2026, time.January)},
		Months:   []month.Month{{}},
		NetWorth: []month.NetWorth{{Cash: amountFor(t, "1000"), Invested: amountFor(t, "2000")}},
		Leftover: []decimal.Decimal{amountFor(t, "3000")},
	}
	updated, _ := m.Update(yearLoadedMsg{year: year})
	m = updated.(Model)
	if m.yearSeries != seriesCash {
		t.Fatalf("yearSeries = %v, want seriesCash by default", m.yearSeries)
	}

	updated, _ = m.Update(key("s"))
	m = updated.(Model)
	if !strings.Contains(m.View().Content, "Invested (current: 2000.00 USD)") {
		t.Errorf("content = %q, want Invested selected after one s", m.View().Content)
	}

	updated, _ = m.Update(key("s"))
	m = updated.(Model)
	if !strings.Contains(m.View().Content, "Pocket money (current: 3000.00 ARS)") {
		t.Errorf("content = %q, want Pocket money selected after a second s", m.View().Content)
	}

	updated, _ = m.Update(key("s"))
	m = updated.(Model)
	if m.yearSeries != seriesCash {
		t.Errorf("yearSeries = %v, want it to cycle back to seriesCash after a third s", m.yearSeries)
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
