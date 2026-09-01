package catalog

import (
	"database/sql"
	"testing"
	"time"

	"github.com/gabokatta/mes/internal/domain"
)

func mustChore(t *testing.T, db *sql.DB) Chore {
	t.Helper()
	c, err := CreateChore(db, Chore{
		Name:       "Sacar la basura",
		MonthMask:  domain.Monthly,
		ActiveFrom: domain.NewPeriod(2026, time.January),
	})
	if err != nil {
		t.Fatalf("CreateChore() unexpected error: %v", err)
	}
	return c
}

func TestChoreEntriesFiltersByPeriod(t *testing.T) {
	db := openTestStore(t).DB()
	c := mustChore(t, db)
	sept := domain.NewPeriod(2026, time.September)
	oct := domain.NewPeriod(2026, time.October)

	if err := SetChoreEntryDone(db, c.ID, sept, true); err != nil {
		t.Fatalf("SetChoreEntryDone() unexpected error: %v", err)
	}

	got, err := ChoreEntries(db, sept)
	if err != nil {
		t.Fatalf("ChoreEntries() unexpected error: %v", err)
	}
	if len(got) != 1 || !got[0].Done {
		t.Fatalf("ChoreEntries(sept) = %+v, want a single done row", got)
	}

	got, err = ChoreEntries(db, oct)
	if err != nil {
		t.Fatalf("ChoreEntries() unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ChoreEntries(oct) = %+v, want no rows", got)
	}
}

func TestSetChoreEntryDoneTogglesExistingRow(t *testing.T) {
	db := openTestStore(t).DB()
	c := mustChore(t, db)
	sept := domain.NewPeriod(2026, time.September)

	if err := SetChoreEntryDone(db, c.ID, sept, true); err != nil {
		t.Fatalf("SetChoreEntryDone() unexpected error: %v", err)
	}
	if err := SetChoreEntryDone(db, c.ID, sept, false); err != nil {
		t.Fatalf("SetChoreEntryDone() unexpected error: %v", err)
	}

	got, err := ChoreEntries(db, sept)
	if err != nil {
		t.Fatalf("ChoreEntries() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Done {
		t.Fatalf("ChoreEntries() = %+v, want a single row with done=false", got)
	}
}
