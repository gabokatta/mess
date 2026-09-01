package month

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/gabokatta/mes/internal/catalog"
	"github.com/gabokatta/mes/internal/domain"
	"github.com/gabokatta/mes/internal/store"
)

func openTestStore(t *testing.T) *sql.DB {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "mes.db"))
	if err != nil {
		t.Fatalf("store.Open() unexpected error: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s.DB()
}

func TestLoadResolvesCatalogAgainstStore(t *testing.T) {
	db := openTestStore(t)

	cat, err := catalog.CreateCategory(db, "Hogar", 0)
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

	got, err := Load(db, domain.NewPeriod(2026, time.September))
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if len(got.Lines) != 1 {
		t.Fatalf("Load().Lines = %d lines, want 1", len(got.Lines))
	}
	if got.Lines[0].Concept.Name != "Alquiler" || !got.Lines[0].Amount.Equal(amount(785000)) || got.Lines[0].Confirmed {
		t.Errorf("Load().Lines[0] = %+v, want Alquiler at 785000, projected", got.Lines[0])
	}
}

func TestLoadResolvesChoresAgainstStore(t *testing.T) {
	db := openTestStore(t)

	trash, err := catalog.CreateChore(db, catalog.Chore{
		Name:       "Sacar la basura",
		MonthMask:  domain.Monthly,
		ActiveFrom: domain.NewPeriod(2026, time.January),
	})
	if err != nil {
		t.Fatalf("CreateChore() unexpected error: %v", err)
	}
	sept := domain.NewPeriod(2026, time.September)
	if err := catalog.SetChoreEntryDone(db, trash.ID, sept, true); err != nil {
		t.Fatalf("SetChoreEntryDone() unexpected error: %v", err)
	}

	got, err := Load(db, sept)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if len(got.Chores) != 1 || got.Chores[0].Chore.Name != "Sacar la basura" || !got.Chores[0].Done {
		t.Errorf("Load().Chores = %+v, want a single done Sacar la basura row", got.Chores)
	}
}
