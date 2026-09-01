package catalog

import (
	"database/sql"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
)

func TestMonthEntriesFiltersByPeriodAndParsesNullableAmount(t *testing.T) {
	db := openTestStore(t).DB()
	c := mustConcept(t, db)

	sept := domain.NewPeriod(2026, time.September)
	oct := domain.NewPeriod(2026, time.October)

	mustExec(t, db, `INSERT INTO month_entry (concept_id, period, amount, done) VALUES (?, ?, ?, ?)`,
		c.ID, sept.String(), "800000", 1)
	mustExec(t, db, `INSERT INTO month_entry (concept_id, period, amount, done) VALUES (?, ?, ?, ?)`,
		c.ID, oct.String(), nil, 0)

	got, err := MonthEntries(db, sept)
	if err != nil {
		t.Fatalf("MonthEntries() unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("MonthEntries(sept) returned %d rows, want 1", len(got))
	}
	if got[0].Amount == nil || !got[0].Amount.Equal(decimal.NewFromInt(800000)) {
		t.Errorf("MonthEntries(sept)[0].Amount = %v, want 800000", got[0].Amount)
	}
	if !got[0].Done {
		t.Error("MonthEntries(sept)[0].Done = false, want true")
	}

	got, err = MonthEntries(db, oct)
	if err != nil {
		t.Fatalf("MonthEntries() unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("MonthEntries(oct) returned %d rows, want 1", len(got))
	}
	if got[0].Amount != nil {
		t.Errorf("MonthEntries(oct)[0].Amount = %v, want nil (no override)", got[0].Amount)
	}
	if got[0].Done {
		t.Error("MonthEntries(oct)[0].Done = true, want false")
	}
}

func TestSetMonthEntryAmountInsertsWithDoneFalse(t *testing.T) {
	db := openTestStore(t).DB()
	c := mustConcept(t, db)
	sept := domain.NewPeriod(2026, time.September)
	amount := decimal.NewFromInt(800000)

	if err := SetMonthEntryAmount(db, c.ID, sept, &amount); err != nil {
		t.Fatalf("SetMonthEntryAmount() unexpected error: %v", err)
	}

	got, err := MonthEntries(db, sept)
	if err != nil {
		t.Fatalf("MonthEntries() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Amount == nil || !got[0].Amount.Equal(amount) {
		t.Fatalf("MonthEntries() = %+v, want a single 800000 row", got)
	}
	if got[0].Done {
		t.Error("MonthEntries()[0].Done = true, want false (a fresh row must not touch done)")
	}
}

func TestSetMonthEntryAmountPreservesExistingDone(t *testing.T) {
	db := openTestStore(t).DB()
	c := mustConcept(t, db)
	sept := domain.NewPeriod(2026, time.September)

	if err := SetMonthEntryDone(db, c.ID, sept, true); err != nil {
		t.Fatalf("SetMonthEntryDone() unexpected error: %v", err)
	}
	amount := decimal.NewFromInt(800000)
	if err := SetMonthEntryAmount(db, c.ID, sept, &amount); err != nil {
		t.Fatalf("SetMonthEntryAmount() unexpected error: %v", err)
	}

	got, err := MonthEntries(db, sept)
	if err != nil {
		t.Fatalf("MonthEntries() unexpected error: %v", err)
	}
	if len(got) != 1 || !got[0].Done {
		t.Fatalf("MonthEntries() = %+v, want done still true after setting the amount", got)
	}
}

func TestSetMonthEntryAmountNilClearsOverride(t *testing.T) {
	db := openTestStore(t).DB()
	c := mustConcept(t, db)
	sept := domain.NewPeriod(2026, time.September)
	amount := decimal.NewFromInt(800000)

	if err := SetMonthEntryAmount(db, c.ID, sept, &amount); err != nil {
		t.Fatalf("SetMonthEntryAmount() unexpected error: %v", err)
	}
	if err := SetMonthEntryAmount(db, c.ID, sept, nil); err != nil {
		t.Fatalf("SetMonthEntryAmount() unexpected error: %v", err)
	}

	got, err := MonthEntries(db, sept)
	if err != nil {
		t.Fatalf("MonthEntries() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Amount != nil {
		t.Fatalf("MonthEntries() = %+v, want the override cleared back to nil", got)
	}
}

func TestSetMonthEntryDoneInsertsWithNilAmount(t *testing.T) {
	db := openTestStore(t).DB()
	c := mustConcept(t, db)
	sept := domain.NewPeriod(2026, time.September)

	if err := SetMonthEntryDone(db, c.ID, sept, true); err != nil {
		t.Fatalf("SetMonthEntryDone() unexpected error: %v", err)
	}

	got, err := MonthEntries(db, sept)
	if err != nil {
		t.Fatalf("MonthEntries() unexpected error: %v", err)
	}
	if len(got) != 1 || !got[0].Done || got[0].Amount != nil {
		t.Fatalf("MonthEntries() = %+v, want done true with no amount", got)
	}
}

func TestSetMonthEntryDonePreservesExistingAmount(t *testing.T) {
	db := openTestStore(t).DB()
	c := mustConcept(t, db)
	sept := domain.NewPeriod(2026, time.September)
	amount := decimal.NewFromInt(800000)

	if err := SetMonthEntryAmount(db, c.ID, sept, &amount); err != nil {
		t.Fatalf("SetMonthEntryAmount() unexpected error: %v", err)
	}
	if err := SetMonthEntryDone(db, c.ID, sept, true); err != nil {
		t.Fatalf("SetMonthEntryDone() unexpected error: %v", err)
	}

	got, err := MonthEntries(db, sept)
	if err != nil {
		t.Fatalf("MonthEntries() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Amount == nil || !got[0].Amount.Equal(amount) || !got[0].Done {
		t.Fatalf("MonthEntries() = %+v, want amount preserved and done true", got)
	}
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}
