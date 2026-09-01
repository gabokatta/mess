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

	lines, err := Load(db, domain.NewPeriod(2026, time.September))
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("Load() = %d lines, want 1", len(lines))
	}
	if lines[0].Concept.Name != "Alquiler" || !lines[0].Amount.Equal(amount(785000)) || lines[0].Confirmed {
		t.Errorf("Load()[0] = %+v, want Alquiler at 785000, projected", lines[0])
	}
}
