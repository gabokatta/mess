package month

import (
	"testing"
	"time"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func alloc(period domain.Period, dest catalog.Destination, amt int64, cur domain.Currency) catalog.SavingAllocation {
	return catalog.SavingAllocation{Period: period, Destination: dest, Amount: amount(amt), Currency: cur}
}

func TestResolveNetWorthAddsOpeningAndAllocationsByDestination(t *testing.T) {
	sept := domain.NewPeriod(2026, time.September)
	opening := catalog.OpeningBalances{CashUSD: amount(1000), InvestedUSD: amount(5000)}
	allocations := []catalog.SavingAllocation{
		alloc(sept, catalog.Cash, 200, domain.USD),
		alloc(sept, catalog.Invested, 300, domain.USD),
	}

	got, err := ResolveNetWorth(sept, opening, allocations, nil)
	if err != nil {
		t.Fatalf("ResolveNetWorth() unexpected error: %v", err)
	}
	if !got.Cash.Equal(amount(1200)) {
		t.Errorf("Cash = %s, want 1200 (1000 opening + 200 allocated)", got.Cash)
	}
	if !got.Invested.Equal(amount(5300)) {
		t.Errorf("Invested = %s, want 5300 (5000 opening + 300 allocated)", got.Invested)
	}
}

func TestResolveNetWorthConvertsArsAtItsOwnPeriodRate(t *testing.T) {
	sept := domain.NewPeriod(2026, time.September)
	allocations := []catalog.SavingAllocation{alloc(sept, catalog.Cash, 120000, domain.ARS)}
	rates := []catalog.FxRate{{Period: sept, Value: amount(1200)}}

	got, err := ResolveNetWorth(sept, catalog.OpeningBalances{}, allocations, rates)
	if err != nil {
		t.Fatalf("ResolveNetWorth() unexpected error: %v", err)
	}
	if !got.Cash.Equal(amount(100)) {
		t.Errorf("Cash = %s, want 100 (120000 ARS / 1200)", got.Cash)
	}
}

func TestResolveNetWorthIgnoresAllocationsAfterPeriod(t *testing.T) {
	aug := domain.NewPeriod(2026, time.August)
	sept := domain.NewPeriod(2026, time.September)
	allocations := []catalog.SavingAllocation{alloc(sept, catalog.Cash, 500, domain.USD)}

	got, err := ResolveNetWorth(aug, catalog.OpeningBalances{}, allocations, nil)
	if err != nil {
		t.Fatalf("ResolveNetWorth() unexpected error: %v", err)
	}
	if !got.Cash.IsZero() {
		t.Errorf("Cash = %s, want 0 (September's allocation is after August)", got.Cash)
	}
}

func TestResolveNetWorthIgnoresAllocationsBeforeOpening(t *testing.T) {
	aug := domain.NewPeriod(2026, time.August)
	sept := domain.NewPeriod(2026, time.September)
	opening := catalog.OpeningBalances{Period: sept, CashUSD: amount(1000)}
	allocations := []catalog.SavingAllocation{alloc(aug, catalog.Cash, 500, domain.USD)}

	got, err := ResolveNetWorth(sept, opening, allocations, nil)
	if err != nil {
		t.Fatalf("ResolveNetWorth() unexpected error: %v", err)
	}
	if !got.Cash.Equal(amount(1000)) {
		t.Errorf("Cash = %s, want 1000 (August's allocation predates the opening anchor)", got.Cash)
	}
}

func TestResolveNetWorthFailsWithoutRateForArsAllocation(t *testing.T) {
	sept := domain.NewPeriod(2026, time.September)
	allocations := []catalog.SavingAllocation{alloc(sept, catalog.Cash, 120000, domain.ARS)}

	if _, err := ResolveNetWorth(sept, catalog.OpeningBalances{}, allocations, nil); err == nil {
		t.Error("ResolveNetWorth() error = nil, want an error (no fx rate to convert the ARS allocation)")
	}
}

func TestNetWorthTotalSumsCashAndInvested(t *testing.T) {
	nw := NetWorth{Cash: amount(100), Invested: amount(200)}
	if !nw.Total().Equal(amount(300)) {
		t.Errorf("Total() = %s, want 300", nw.Total())
	}
}
