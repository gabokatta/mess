package month

import (
	"testing"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func lineOf(kind catalog.ConceptKind, cur domain.Currency, amt int64, share int64, confirmed bool) Line {
	return Line{
		Concept: catalog.Concept{
			Kind:     kind,
			Currency: cur,
			Share:    domain.NewPercent(share),
		},
		Amount:    amount(amt),
		Confirmed: confirmed,
	}
}

func TestResolveTotalsNetsIncomeAgainstExpenses(t *testing.T) {
	lines := []Line{
		lineOf(catalog.Income, domain.ARS, 1000000, 100, true),
		lineOf(catalog.FixedExpense, domain.ARS, 785000, 100, false),
	}

	totals := ResolveTotals(lines)

	got := totals.Projected[domain.ARS].Household
	if !got.Equal(amount(215000)) {
		t.Errorf("Projected household = %s, want 215000 (1000000 income - 785000 expense)", got)
	}
}

func TestResolveTotalsSharesAppliesConceptShare(t *testing.T) {
	lines := []Line{lineOf(catalog.FixedExpense, domain.ARS, 785000, 50, true)}

	totals := ResolveTotals(lines)

	got := totals.Projected[domain.ARS].Share
	if !got.Equal(amount(-392500)) {
		t.Errorf("Projected share = %s, want -392500 (50%% of -785000)", got)
	}
}

func TestResolveTotalsConfirmedOnlyCountsOverrides(t *testing.T) {
	lines := []Line{
		lineOf(catalog.FixedExpense, domain.ARS, 785000, 100, true),
		lineOf(catalog.FixedExpense, domain.ARS, 15000, 100, false),
	}

	totals := ResolveTotals(lines)

	if got := totals.Projected[domain.ARS].Household; !got.Equal(amount(-800000)) {
		t.Errorf("Projected household = %s, want -800000 (both lines)", got)
	}
	if got := totals.Confirmed[domain.ARS].Household; !got.Equal(amount(-785000)) {
		t.Errorf("Confirmed household = %s, want -785000 (only the confirmed line)", got)
	}
}

func TestResolveTotalsGroupsByCurrency(t *testing.T) {
	lines := []Line{
		lineOf(catalog.FixedExpense, domain.ARS, 785000, 100, true),
		lineOf(catalog.FixedExpense, domain.USD, 50, 100, true),
	}

	totals := ResolveTotals(lines)

	if got := totals.Projected[domain.ARS].Household; !got.Equal(amount(-785000)) {
		t.Errorf("ARS household = %s, want -785000 (USD line must not bleed in)", got)
	}
	if got := totals.Projected[domain.USD].Household; !got.Equal(amount(-50)) {
		t.Errorf("USD household = %s, want -50", got)
	}
}
