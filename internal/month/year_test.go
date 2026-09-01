package month

import (
	"testing"
	"time"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func TestLoadYearResolvesEachPeriodOfTheYear(t *testing.T) {
	db := openTestStore(t)
	cat, err := catalog.CreateCategory(db, "Servicios", 0)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	rent, err := catalog.CreateConcept(db, catalog.Concept{
		Name:       "Alquiler",
		CategoryID: cat.ID,
		Kind:       catalog.FixedExpense,
		Currency:   domain.ARS,
		MonthMask:  domain.Monthly,
		ActiveFrom: domain.NewPeriod(2026, time.January),
	})
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}
	if err := catalog.SetBaseAmount(db, rent.ID, domain.NewPeriod(2026, time.January), amount(785000)); err != nil {
		t.Fatalf("SetBaseAmount() unexpected error: %v", err)
	}

	got, err := LoadYear(db, 2026)
	if err != nil {
		t.Fatalf("LoadYear() unexpected error: %v", err)
	}

	if len(got.Periods) != 12 || len(got.Months) != 12 {
		t.Fatalf("LoadYear() has %d periods and %d months, want 12 each", len(got.Periods), len(got.Months))
	}
	if want := domain.NewPeriod(2026, time.January); !got.Periods[0].Equal(want) {
		t.Errorf("Periods[0] = %s, want %s", got.Periods[0], want)
	}
	if want := domain.NewPeriod(2026, time.December); !got.Periods[11].Equal(want) {
		t.Errorf("Periods[11] = %s, want %s", got.Periods[11], want)
	}
	for i, p := range got.Periods {
		lines := got.Months[i].Lines
		if len(lines) != 1 || !lines[0].Amount.Equal(amount(785000)) {
			t.Errorf("Months[%d] (%s) lines = %+v, want Alquiler at 785000", i, p, lines)
		}
	}
}

func TestLoadYearCategoryTotalsSumExpensesOnlyInArs(t *testing.T) {
	db := openTestStore(t)
	servicios, err := catalog.CreateCategory(db, "Servicios", 0)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	ingresos, err := catalog.CreateCategory(db, "Ingresos", 1)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	jan := domain.NewPeriod(2026, time.January)

	rent, err := catalog.CreateConcept(db, catalog.Concept{
		Name: "Alquiler", CategoryID: servicios.ID, Kind: catalog.FixedExpense,
		Currency: domain.ARS, MonthMask: domain.Monthly, ActiveFrom: jan,
	})
	if err != nil {
		t.Fatalf("CreateConcept(rent) unexpected error: %v", err)
	}
	if err := catalog.SetBaseAmount(db, rent.ID, jan, amount(100000)); err != nil {
		t.Fatalf("SetBaseAmount(rent) unexpected error: %v", err)
	}

	internet, err := catalog.CreateConcept(db, catalog.Concept{
		Name: "Internet", CategoryID: servicios.ID, Kind: catalog.FixedExpense,
		Currency: domain.ARS, MonthMask: domain.Monthly, ActiveFrom: jan,
	})
	if err != nil {
		t.Fatalf("CreateConcept(internet) unexpected error: %v", err)
	}
	if err := catalog.SetBaseAmount(db, internet.ID, jan, amount(20000)); err != nil {
		t.Fatalf("SetBaseAmount(internet) unexpected error: %v", err)
	}

	vps, err := catalog.CreateConcept(db, catalog.Concept{
		Name: "VPS", CategoryID: servicios.ID, Kind: catalog.FixedExpense,
		Currency: domain.USD, MonthMask: domain.Monthly, ActiveFrom: jan,
	})
	if err != nil {
		t.Fatalf("CreateConcept(vps) unexpected error: %v", err)
	}
	if err := catalog.SetBaseAmount(db, vps.ID, jan, amount(5)); err != nil {
		t.Fatalf("SetBaseAmount(vps) unexpected error: %v", err)
	}

	salary, err := catalog.CreateConcept(db, catalog.Concept{
		Name: "Sueldo", CategoryID: ingresos.ID, Kind: catalog.Income,
		Currency: domain.ARS, MonthMask: domain.Monthly, ActiveFrom: jan,
	})
	if err != nil {
		t.Fatalf("CreateConcept(salary) unexpected error: %v", err)
	}
	if err := catalog.SetBaseAmount(db, salary.ID, jan, amount(1000000)); err != nil {
		t.Fatalf("SetBaseAmount(salary) unexpected error: %v", err)
	}

	got, err := LoadYear(db, 2026)
	if err != nil {
		t.Fatalf("LoadYear() unexpected error: %v", err)
	}

	if len(got.Categories) != 1 {
		t.Fatalf("Categories = %+v, want exactly Servicios (income and zero-total categories excluded)", got.Categories)
	}
	want := amount((100000 + 20000) * 12)
	if got.Categories[0].Category.ID != servicios.ID || !got.Categories[0].Total.Equal(want) {
		t.Errorf("Categories[0] = %+v, want Servicios at %s (rent+internet across 12 months, VPS excluded as USD)",
			got.Categories[0], want)
	}
}

func TestLoadYearNetWorthSeriesFoldsAllocationsAcrossTheYear(t *testing.T) {
	db := openTestStore(t)
	if _, err := catalog.CreateSavingAllocation(db, catalog.SavingAllocation{
		Period: domain.NewPeriod(2026, time.March), Destination: catalog.Invested, Amount: amount(100), Currency: domain.USD,
	}); err != nil {
		t.Fatalf("CreateSavingAllocation() unexpected error: %v", err)
	}

	got, err := LoadYear(db, 2026)
	if err != nil {
		t.Fatalf("LoadYear() unexpected error: %v", err)
	}

	if len(got.NetWorth) != 12 || len(got.Leftover) != 12 {
		t.Fatalf("NetWorth/Leftover series have %d/%d entries, want 12 each", len(got.NetWorth), len(got.Leftover))
	}
	if !got.NetWorth[1].Invested.IsZero() {
		t.Errorf("NetWorth[Feb].Invested = %s, want 0 (allocation not reached yet)", got.NetWorth[1].Invested)
	}
	if !got.NetWorth[2].Invested.Equal(amount(100)) {
		t.Errorf("NetWorth[Mar].Invested = %s, want 100 (allocation folded in)", got.NetWorth[2].Invested)
	}
	if !got.NetWorth[11].Invested.Equal(amount(100)) {
		t.Errorf("NetWorth[Dec].Invested = %s, want 100 (still folded in at year end)", got.NetWorth[11].Invested)
	}
}
