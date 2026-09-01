package catalog

import (
	"database/sql"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mes/internal/domain"
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

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}
