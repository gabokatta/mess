package month

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func TestAvailableToSaveIsConfirmedArsShare(t *testing.T) {
	lines := []Line{
		lineOf(catalog.Income, domain.ARS, 1000000, 100, true),
		lineOf(catalog.FixedExpense, domain.ARS, 785000, 100, true),
		lineOf(catalog.FixedExpense, domain.ARS, 50000, 100, false),
	}

	got := AvailableToSave(ResolveTotals(lines))

	if !got.Equal(amount(215000)) {
		t.Errorf("AvailableToSave() = %s, want 215000 (confirmed lines only)", got)
	}
}

func TestResolveLeftoverPesosFoldsNetMinusAllocations(t *testing.T) {
	aug := domain.NewPeriod(2026, time.August)
	sept := domain.NewPeriod(2026, time.September)
	opening := catalog.OpeningBalances{Period: aug, LeftoverPesos: amount(10000)}
	monthlyNets := map[domain.Period]decimal.Decimal{aug: amount(50000), sept: amount(60000)}
	allocations := []catalog.SavingAllocation{alloc(sept, catalog.Cash, 40000, domain.ARS)}

	got, err := ResolveLeftoverPesos(sept, opening, monthlyNets, allocations, nil)
	if err != nil {
		t.Fatalf("ResolveLeftoverPesos() unexpected error: %v", err)
	}
	want := amount(10000 + 50000 + 60000 - 40000)
	if !got.Equal(want) {
		t.Errorf("ResolveLeftoverPesos() = %s, want %s", got, want)
	}
}

func TestResolveLeftoverPesosConvertsUsdAllocationsAtTheirOwnRate(t *testing.T) {
	sept := domain.NewPeriod(2026, time.September)
	opening := catalog.OpeningBalances{Period: sept}
	monthlyNets := map[domain.Period]decimal.Decimal{sept: amount(100000)}
	allocations := []catalog.SavingAllocation{alloc(sept, catalog.Invested, 50, domain.USD)}
	rates := []catalog.FxRate{{Period: sept, Value: amount(1200)}}

	got, err := ResolveLeftoverPesos(sept, opening, monthlyNets, allocations, rates)
	if err != nil {
		t.Fatalf("ResolveLeftoverPesos() unexpected error: %v", err)
	}
	if !got.Equal(amount(100000 - 60000)) {
		t.Errorf("ResolveLeftoverPesos() = %s, want 40000 (100000 net - 50 USD at 1200)", got)
	}
}

func TestResolveLeftoverPesosZeroOpeningHasNothingToFold(t *testing.T) {
	sept := domain.NewPeriod(2026, time.September)

	got, err := ResolveLeftoverPesos(sept, catalog.OpeningBalances{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("ResolveLeftoverPesos() unexpected error: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("ResolveLeftoverPesos() = %s, want 0 (no settings row yet)", got)
	}
}

func TestResolveLeftoverPesosFailsWithoutRateForUsdAllocation(t *testing.T) {
	sept := domain.NewPeriod(2026, time.September)
	opening := catalog.OpeningBalances{Period: sept}
	monthlyNets := map[domain.Period]decimal.Decimal{sept: amount(0)}
	allocations := []catalog.SavingAllocation{alloc(sept, catalog.Cash, 50, domain.USD)}

	if _, err := ResolveLeftoverPesos(sept, opening, monthlyNets, allocations, nil); err == nil {
		t.Error("ResolveLeftoverPesos() error = nil, want an error (no fx rate to convert the USD allocation)")
	}
}

func TestMonthlyNetsARSResolvesEachPeriodInRange(t *testing.T) {
	db := openTestStore(t)
	cat, err := catalog.CreateCategory(db, "Ingresos", 0)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	salary, err := catalog.CreateConcept(db, catalog.Concept{
		Name:       "Sueldo",
		CategoryID: cat.ID,
		Kind:       catalog.Income,
		Currency:   domain.ARS,
		MonthMask:  domain.Monthly,
		ActiveFrom: domain.NewPeriod(2026, time.January),
	})
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}
	if err := catalog.SetBaseAmount(db, salary.ID, domain.NewPeriod(2026, time.January), amount(1000000)); err != nil {
		t.Fatalf("SetBaseAmount() unexpected error: %v", err)
	}
	aug := domain.NewPeriod(2026, time.August)
	sept := domain.NewPeriod(2026, time.September)
	if err := catalog.SetMonthEntryAmount(db, salary.ID, aug, ptr(amount(1000000))); err != nil {
		t.Fatalf("SetMonthEntryAmount() unexpected error: %v", err)
	}
	if err := catalog.SetMonthEntryAmount(db, salary.ID, sept, ptr(amount(1100000))); err != nil {
		t.Fatalf("SetMonthEntryAmount() unexpected error: %v", err)
	}

	got, err := MonthlyNetsARS(db, aug, sept)
	if err != nil {
		t.Fatalf("MonthlyNetsARS() unexpected error: %v", err)
	}
	if len(got) != 2 || !got[aug].Equal(amount(1000000)) || !got[sept].Equal(amount(1100000)) {
		t.Errorf("MonthlyNetsARS() = %+v, want August 1000000 and September 1100000", got)
	}
}

func TestMonthlyNetsARSZeroOpeningIsEmpty(t *testing.T) {
	db := openTestStore(t)

	got, err := MonthlyNetsARS(db, domain.Period{}, domain.NewPeriod(2026, time.September))
	if err != nil {
		t.Fatalf("MonthlyNetsARS() unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("MonthlyNetsARS() = %+v, want empty (zero opening period)", got)
	}
}

func ptr(d decimal.Decimal) *decimal.Decimal { return &d }
