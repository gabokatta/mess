package month

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func lineOf(kind catalog.ConceptKind, cur domain.Currency, amt int64, share int64, confirmed bool) Line {
	return Line{
		Concept: catalog.Concept{
			Kind:  kind,
			Money: &catalog.MoneyDetails{Currency: cur, Share: domain.NewPercent(share)},
		},
		Money: &LineMoney{Amount: amount(amt), Confirmed: confirmed},
	}
}

func TestResolveTotalsNetsIncomeAgainstExpenses(t *testing.T) {
	lines := []Line{
		lineOf(catalog.Income, domain.ARS, 1000000, 100, true),
		lineOf(catalog.Expense, domain.ARS, 785000, 100, false),
	}

	totals := ResolveTotals(lines)

	got := totals.Projected[domain.ARS].Household
	if !got.Equal(amount(215000)) {
		t.Errorf("Projected household = %s, want 215000 (1000000 income - 785000 expense)", got)
	}
}

func TestResolveTotalsSharesAppliesConceptShare(t *testing.T) {
	lines := []Line{lineOf(catalog.Expense, domain.ARS, 785000, 50, true)}

	totals := ResolveTotals(lines)

	got := totals.Projected[domain.ARS].Share
	if !got.Equal(amount(-392500)) {
		t.Errorf("Projected share = %s, want -392500 (50%% of -785000)", got)
	}
}

func TestResolveTotalsConfirmedOnlyCountsOverrides(t *testing.T) {
	lines := []Line{
		lineOf(catalog.Expense, domain.ARS, 785000, 100, true),
		lineOf(catalog.Expense, domain.ARS, 15000, 100, false),
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
		lineOf(catalog.Expense, domain.ARS, 785000, 100, true),
		lineOf(catalog.Expense, domain.USD, 50, 100, true),
	}

	totals := ResolveTotals(lines)

	if got := totals.Projected[domain.ARS].Household; !got.Equal(amount(-785000)) {
		t.Errorf("ARS household = %s, want -785000 (USD line must not bleed in)", got)
	}
	if got := totals.Projected[domain.USD].Household; !got.Equal(amount(-50)) {
		t.Errorf("USD household = %s, want -50", got)
	}
}

func TestResolveHeaderNetFoldsUSDLineIntoARSAtRate(t *testing.T) {
	lines := []Line{
		lineOf(catalog.Expense, domain.ARS, 785000, 100, true),
		lineOf(catalog.Expense, domain.USD, 450, 100, true),
	}

	h := ResolveHeaderNet(lines, amount(1000), true)

	want := amount(-785000).Sub(amount(450000))
	if !h.Net.Household.Equal(want) {
		t.Errorf("Household = %s, want %s (785000 ARS + 450 USD at 1000)", h.Net.Household, want)
	}
}

func TestResolveHeaderNetCountsConfirmedOverTotalLines(t *testing.T) {
	lines := []Line{
		lineOf(catalog.Expense, domain.ARS, 785000, 100, true),
		lineOf(catalog.Expense, domain.ARS, 15000, 100, false),
	}

	h := ResolveHeaderNet(lines, decimal.Decimal{}, false)

	if h.Confirmed != 1 || h.Lines != 2 {
		t.Errorf("Confirmed/Lines = %d/%d, want 1/2", h.Confirmed, h.Lines)
	}
}

func TestResolveHeaderNetSkipsUnconvertibleLineWithoutRate(t *testing.T) {
	lines := []Line{lineOf(catalog.Expense, domain.USD, 450, 100, true)}

	h := ResolveHeaderNet(lines, decimal.Decimal{}, false)

	if !h.Net.Household.IsZero() {
		t.Errorf("Household = %s, want 0 (no rate to convert the USD line)", h.Net.Household)
	}
	if h.Confirmed != 1 {
		t.Errorf("Confirmed = %d, want 1 (still counted, just not folded into Net)", h.Confirmed)
	}
}
