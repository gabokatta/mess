package month

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/store"
)

func openTestStore(t *testing.T) *sql.DB {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "mess.db"))
	if err != nil {
		t.Fatalf("store.Open() unexpected error: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s.DB()
}

func seedConcept(t *testing.T, db *sql.DB, category string, c catalog.Concept) catalog.Concept {
	t.Helper()
	cat, err := catalog.FindOrCreateCategory(db, category)
	if err != nil {
		t.Fatalf("FindOrCreateCategory() unexpected error: %v", err)
	}
	c.CategoryID = cat.ID
	if c.MonthMask == 0 {
		c.MonthMask = domain.Monthly
	}
	if c.ActiveFrom.IsZero() {
		c.ActiveFrom = domain.NewPeriod(2026, time.January)
	}
	created, err := catalog.CreateConcept(db, c)
	if err != nil {
		t.Fatalf("CreateConcept(%s) unexpected error: %v", c.Name, err)
	}
	return created
}

func TestLoadResolvesTheCatalogAgainstTheStore(t *testing.T) {
	db := openTestStore(t)
	september := domain.NewPeriod(2026, time.September)

	rent := seedConcept(t, db, "Home", catalog.Concept{
		Name: "Rent", Kind: catalog.Expense,
		Money: &catalog.MoneyDetails{Currency: domain.ARS, Base: decimal.NewFromInt(785000)},
	})
	seedConcept(t, db, "Home", catalog.Concept{Name: "Wash the house", Kind: catalog.Chore})

	typed := decimal.NewFromInt(812000)
	if err := catalog.SetMonthEntryAmount(db, rent.ID, september, &typed); err != nil {
		t.Fatalf("SetMonthEntryAmount() unexpected error: %v", err)
	}

	loaded, err := Load(db, september)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if len(loaded.Lines) != 2 {
		t.Fatalf("Load() returned %d lines, want 2", len(loaded.Lines))
	}

	byName := map[string]Line{}
	for _, l := range loaded.Lines {
		byName[l.Concept.Name] = l
	}
	if got := byName["Rent"]; !got.Money.Confirmed || !got.Money.Amount.Amount().Equal(typed) {
		t.Errorf("Rent = %+v, want the confirmed 812000 override", got.Money)
	}
	if got := byName["Wash the house"]; got.Money != nil {
		t.Errorf("chore Money = %+v, want nil", got.Money)
	}
}

// A month you never touched stores nothing and still resolves.
func TestLoadUntouchedPeriodStoresNothing(t *testing.T) {
	db := openTestStore(t)
	seedConcept(t, db, "Home", catalog.Concept{
		Name: "Rent", Kind: catalog.Expense,
		Money: &catalog.MoneyDetails{Currency: domain.ARS, Base: decimal.NewFromInt(785000)},
	})

	loaded, err := Load(db, domain.NewPeriod(2026, time.October))
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if len(loaded.Lines) != 1 || loaded.Lines[0].Money.Confirmed {
		t.Errorf("Load() = %+v, want one unconfirmed line", loaded.Lines)
	}

	var entries int
	if err := db.QueryRow("SELECT count(*) FROM month_entry").Scan(&entries); err != nil {
		t.Fatalf("count month_entry: %v", err)
	}
	if entries != 0 {
		t.Errorf("month_entry has %d rows, want none for a month nobody touched", entries)
	}
}
