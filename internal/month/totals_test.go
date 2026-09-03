package month

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func line(kind catalog.ConceptKind, currency domain.Currency, amount int64, confirmed bool) Line {
	return Line{
		Concept: catalog.Concept{Kind: kind, Money: &catalog.MoneyDetails{Currency: currency}},
		Money: &LineMoney{
			Amount:    domain.NewMoney(decimal.NewFromInt(amount), currency),
			Confirmed: confirmed,
		},
	}
}

func rateOf(n int64) Rate {
	return Rate{Value: decimal.NewFromInt(n), Origin: RateLive}
}

func TestResolveTotals(t *testing.T) {
	totals := ResolveTotals([]Line{
		line(catalog.Income, domain.ARS, 2400000, true),
		line(catalog.Expense, domain.ARS, 785000, true),
		line(catalog.Expense, domain.ARS, 34200, true),
		line(catalog.Saving, domain.ARS, 480000, true),
	}, rateOf(1200))

	want := decimal.NewFromInt(2400000 - 785000 - 34200)
	if !totals.Available.Amount().Equal(want) {
		t.Errorf("Available = %s, want %s", totals.Available.Amount(), want)
	}
	if !totals.Saved.Amount().Equal(decimal.NewFromInt(480000)) {
		t.Errorf("Saved = %s, want 480000", totals.Saved.Amount())
	}
	if !totals.Pocket.Amount().Equal(want.Sub(decimal.NewFromInt(480000))) {
		t.Errorf("Pocket = %s, want available - saved", totals.Pocket.Amount())
	}
	if totals.Available.Currency() != domain.ARS {
		t.Error("the header rolls up in ARS")
	}
}

// A baseline you have not typed over is a guess and does not count.
func TestResolveTotalsCountsConfirmedLinesOnly(t *testing.T) {
	totals := ResolveTotals([]Line{
		line(catalog.Income, domain.ARS, 2400000, true),
		line(catalog.Expense, domain.ARS, 785000, false),
	}, rateOf(1200))

	if !totals.Available.Amount().Equal(decimal.NewFromInt(2400000)) {
		t.Errorf("Available = %s, want the unconfirmed expense left out", totals.Available.Amount())
	}
}

// A chore has no money and never reaches the arithmetic.
func TestResolveTotalsIgnoresChores(t *testing.T) {
	totals := ResolveTotals([]Line{
		{Concept: catalog.Concept{Kind: catalog.Chore}, Done: true},
	}, rateOf(1200))

	if !totals.Available.Amount().IsZero() {
		t.Errorf("Available = %s, want zero", totals.Available.Amount())
	}
	if !totals.Saved.Amount().IsZero() {
		t.Errorf("Saved = %s, want zero", totals.Saved.Amount())
	}
}

func TestResolveTotalsFoldsUSDAtThePeriodRate(t *testing.T) {
	totals := ResolveTotals([]Line{
		line(catalog.Income, domain.ARS, 2400000, true),
		line(catalog.Expense, domain.USD, 150, true),
		line(catalog.Saving, domain.USD, 400, true),
	}, rateOf(1200))

	if !totals.Available.Amount().Equal(decimal.NewFromInt(2400000 - 150*1200)) {
		t.Errorf("Available = %s, want the USD expense converted at 1200", totals.Available.Amount())
	}
	if !totals.Saved.Amount().Equal(decimal.NewFromInt(400 * 1200)) {
		t.Errorf("Saved = %s, want 480000", totals.Saved.Amount())
	}
}

// The header drops what it cannot convert and says how many.
func TestResolveTotalsExcludesUnconvertibleLines(t *testing.T) {
	totals := ResolveTotals([]Line{
		line(catalog.Income, domain.ARS, 2400000, true),
		line(catalog.Expense, domain.USD, 150, true),
	}, Rate{})

	if !totals.Available.Amount().Equal(decimal.NewFromInt(2400000)) {
		t.Errorf("Available = %s, want the USD line dropped, not zeroed", totals.Available.Amount())
	}
	if totals.Excluded != 1 {
		t.Errorf("Excluded = %d, want 1", totals.Excluded)
	}
}

// Over-saving is legal: Pocket goes negative rather than refusing the input.
func TestPocketGoesNegativeWhenOverSaved(t *testing.T) {
	totals := ResolveTotals([]Line{
		line(catalog.Income, domain.ARS, 100000, true),
		line(catalog.Saving, domain.ARS, 130000, true),
	}, rateOf(1200))

	if !totals.Pocket.Amount().Equal(decimal.NewFromInt(-30000)) {
		t.Errorf("Pocket = %s, want -30000", totals.Pocket.Amount())
	}
}

func TestSavedUSD(t *testing.T) {
	totals := ResolveTotals([]Line{
		line(catalog.Saving, domain.USD, 400, true),
	}, rateOf(1200))

	if got := totals.SavedUSD(rateOf(1200)); !got.Equal(decimal.NewFromInt(400)) {
		t.Errorf("SavedUSD() = %s, want 400", got)
	}
	if got := totals.SavedUSD(Rate{}); !got.IsZero() {
		t.Errorf("SavedUSD() without a rate = %s, want zero", got)
	}
}

func TestAvailableUSD(t *testing.T) {
	totals := ResolveTotals([]Line{
		line(catalog.Income, domain.ARS, 2400000, true),
	}, rateOf(1200))

	if got := totals.AvailableUSD(rateOf(1200)); !got.Equal(decimal.NewFromInt(2000)) {
		t.Errorf("AvailableUSD() = %s, want 2000", got)
	}
	if got := totals.AvailableUSD(Rate{}); !got.IsZero() {
		t.Errorf("AvailableUSD() without a rate = %s, want zero", got)
	}
}

// PocketUSD keeps Pocket's sign, so an over-saved month divides negative too.
func TestPocketUSD(t *testing.T) {
	totals := ResolveTotals([]Line{
		line(catalog.Income, domain.ARS, 100000, true),
		line(catalog.Saving, domain.ARS, 130000, true),
	}, rateOf(1200))

	if got := totals.PocketUSD(rateOf(1200)); !got.Equal(decimal.NewFromInt(-30000).Div(decimal.NewFromInt(1200))) {
		t.Errorf("PocketUSD() = %s, want pocket / 1200", got)
	}
	if got := totals.PocketUSD(Rate{}); !got.IsZero() {
		t.Errorf("PocketUSD() without a rate = %s, want zero", got)
	}
}

func TestResolveTotalsSkipsAPeriodWithNothingConfirmed(t *testing.T) {
	totals := ResolveTotals(Resolve(domain.NewPeriod(2026, time.September),
		[]catalog.Concept{concept(1, catalog.Expense, 785000)}, nil), rateOf(1200))

	if !totals.Available.Amount().IsZero() {
		t.Errorf("Available = %s, want a month you have not touched to read zero", totals.Available.Amount())
	}
	if totals.Excluded != 0 {
		t.Errorf("Excluded = %d, want 0", totals.Excluded)
	}
}
