package month

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
)

// June is the one anchor whose eighteen months of history land exactly on a
// calendar year boundary, so the year before it comes back full.
func TestDemoIsAWorldWorthOpening(t *testing.T) {
	anchor := domain.NewPeriod(2026, time.June)
	oldest := anchor.AddMonths(-17)

	db := fixture.DB(t)
	fixture.MustLoad(t, db, fixture.Demo(anchor))

	anchorMonth, err := Load(db, anchor)
	if err != nil {
		t.Fatalf("Load(anchor) unexpected error: %v", err)
	}
	if n := len(anchorMonth.Lines); n < 25 || n > 32 {
		t.Errorf("Load(anchor) returned %d lines, want around thirty so the month list scrolls", n)
	}

	rates, err := catalog.FxRates(db)
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	fx := NewFxTable(rates, decimal.Decimal{}, false, anchor)

	oldestMonth, err := Load(db, oldest)
	if err != nil {
		t.Fatalf("Load(oldest) unexpected error: %v", err)
	}
	totals := ResolveTotals(oldestMonth.Lines, fx.At(oldest))
	if totals.Excluded == 0 {
		t.Error("ResolveTotals(oldest).Excluded = 0, want the confirmed USD line with no rate to convert it counted")
	}

	previousYear := anchor.Year() - 1
	year, err := LoadYear(db, previousYear, fx)
	if err != nil {
		t.Fatalf("LoadYear() unexpected error: %v", err)
	}
	for i, spent := range year.Spent {
		if spent.IsZero() {
			t.Errorf("LoadYear(%d).Spent[%d] = 0, want every month of the year before the anchor populated", previousYear, i)
		}
	}
}
