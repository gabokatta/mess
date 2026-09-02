package catalog

import (
	"database/sql"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
)

func mustConcept(t *testing.T, db *sql.DB) Concept {
	t.Helper()
	cat := mustCategory(t, db)
	c, err := CreateConcept(db, Concept{
		Name:       "Alquiler",
		CategoryID: cat.ID,
		Kind:       Expense,
		Currency:   domain.ARS,
		MonthMask:  domain.Monthly,
		Share:      domain.NewPercent(50),
		ActiveFrom: domain.NewPeriod(2026, time.January),
	})
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}
	return c
}

func TestSetAndListBaseAmounts(t *testing.T) {
	db := openTestStore(t).DB()
	c := mustConcept(t, db)

	if err := SetBaseAmount(db, c.ID, domain.NewPeriod(2026, time.January), decimal.NewFromInt(785000)); err != nil {
		t.Fatalf("SetBaseAmount() unexpected error: %v", err)
	}
	if err := SetBaseAmount(db, c.ID, domain.NewPeriod(2026, time.June), decimal.NewFromInt(850000)); err != nil {
		t.Fatalf("SetBaseAmount() unexpected error: %v", err)
	}

	got, err := BaseAmounts(db, c.ID)
	if err != nil {
		t.Fatalf("BaseAmounts() unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("BaseAmounts() returned %d rows, want 2", len(got))
	}
	if !got[0].EffectiveFrom.Equal(domain.NewPeriod(2026, time.January)) || !got[0].Amount.Equal(decimal.NewFromInt(785000)) {
		t.Errorf("BaseAmounts()[0] = %+v, want 2026-01 785000", got[0])
	}
	if !got[1].EffectiveFrom.Equal(domain.NewPeriod(2026, time.June)) || !got[1].Amount.Equal(decimal.NewFromInt(850000)) {
		t.Errorf("BaseAmounts()[1] = %+v, want 2026-06 850000", got[1])
	}
}

func TestSetBaseAmountUpsertsSameEffectiveFrom(t *testing.T) {
	db := openTestStore(t).DB()
	c := mustConcept(t, db)

	if err := SetBaseAmount(db, c.ID, domain.NewPeriod(2026, time.January), decimal.NewFromInt(785000)); err != nil {
		t.Fatalf("SetBaseAmount() unexpected error: %v", err)
	}
	if err := SetBaseAmount(db, c.ID, domain.NewPeriod(2026, time.January), decimal.NewFromInt(800000)); err != nil {
		t.Fatalf("SetBaseAmount() unexpected error: %v", err)
	}

	got, err := BaseAmounts(db, c.ID)
	if err != nil {
		t.Fatalf("BaseAmounts() unexpected error: %v", err)
	}
	if len(got) != 1 || !got[0].Amount.Equal(decimal.NewFromInt(800000)) {
		t.Errorf("BaseAmounts() = %+v, want a single 800000 row (correction, not a new row)", got)
	}
}

func TestAllBaseAmountsGroupsByConcept(t *testing.T) {
	db := openTestStore(t).DB()
	rent := mustConcept(t, db)
	cat, err := CreateCategory(db, "Otros", 1)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	internet, err := CreateConcept(db, Concept{
		Name:       "Internet",
		CategoryID: cat.ID,
		Kind:       Expense,
		Currency:   domain.ARS,
		MonthMask:  domain.Monthly,
		ActiveFrom: domain.NewPeriod(2026, time.January),
	})
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}

	if err := SetBaseAmount(db, rent.ID, domain.NewPeriod(2026, time.January), decimal.NewFromInt(785000)); err != nil {
		t.Fatalf("SetBaseAmount() unexpected error: %v", err)
	}
	if err := SetBaseAmount(db, rent.ID, domain.NewPeriod(2026, time.June), decimal.NewFromInt(850000)); err != nil {
		t.Fatalf("SetBaseAmount() unexpected error: %v", err)
	}
	if err := SetBaseAmount(db, internet.ID, domain.NewPeriod(2026, time.January), decimal.NewFromInt(15000)); err != nil {
		t.Fatalf("SetBaseAmount() unexpected error: %v", err)
	}

	got, err := AllBaseAmounts(db)
	if err != nil {
		t.Fatalf("AllBaseAmounts() unexpected error: %v", err)
	}
	if len(got[rent.ID]) != 2 {
		t.Errorf("AllBaseAmounts()[rent] = %+v, want 2 rows", got[rent.ID])
	}
	if len(got[internet.ID]) != 1 || !got[internet.ID][0].Amount.Equal(decimal.NewFromInt(15000)) {
		t.Errorf("AllBaseAmounts()[internet] = %+v, want a single 15000 row", got[internet.ID])
	}
}

func TestLatestBaseAmountReturnsMostRecentlyDated(t *testing.T) {
	amounts := []BaseAmount{
		{EffectiveFrom: domain.NewPeriod(2026, time.January), Amount: decimal.NewFromInt(785000)},
		{EffectiveFrom: domain.NewPeriod(2026, time.June), Amount: decimal.NewFromInt(850000)},
	}
	got, ok := LatestBaseAmount(amounts)
	if !ok {
		t.Fatal("LatestBaseAmount() ok = false, want true")
	}
	if !got.Amount.Equal(decimal.NewFromInt(850000)) {
		t.Errorf("LatestBaseAmount() = %+v, want the June 850000 row", got)
	}
}

func TestLatestBaseAmountEmptyReportsNotOK(t *testing.T) {
	if _, ok := LatestBaseAmount(nil); ok {
		t.Error("LatestBaseAmount(nil) ok = true, want false")
	}
}

func TestSetBaseAmountRequiresExistingConcept(t *testing.T) {
	db := openTestStore(t).DB()

	err := SetBaseAmount(db, 999, domain.NewPeriod(2026, time.January), decimal.NewFromInt(785000))
	if err == nil {
		t.Error("SetBaseAmount() with a dangling concept ID should fail the foreign key check")
	}
}
