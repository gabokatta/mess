package month

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
)

func TestLoadResolvesTheCatalogAgainstTheStore(t *testing.T) {
	db := fixture.DB(t)
	fixture.MustLoad(t, db, fixture.World{
		Concepts: []fixture.Concept{
			{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "785000"},
			{Name: "Wash the house", Category: "Home", Kind: catalog.Chore},
		},
		Entries: []fixture.Entry{
			{Concept: "Rent", Period: fixture.Period, Amount: "812000"},
		},
	})

	loaded, err := Load(db, fixture.Period)
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
	want := LineMoney{Amount: domain.NewMoney(decimal.NewFromInt(812000), domain.ARS), Overridden: true}
	if diff := cmp.Diff(want, *byName["Rent"].Money); diff != "" {
		t.Errorf("Rent money mismatch (-want +got):\n%s", diff)
	}
	if got := byName["Wash the house"].Money; got != nil {
		t.Errorf("chore Money = %+v, want nil", got)
	}
}

// A month you never touched stores nothing and still resolves.
func TestLoadUntouchedPeriodStoresNothing(t *testing.T) {
	db := fixture.DB(t)
	fixture.MustLoad(t, db, fixture.World{
		Concepts: []fixture.Concept{
			{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "785000"},
		},
	})

	loaded, err := Load(db, domain.NewPeriod(fixture.Year, time.October))
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if len(loaded.Lines) != 1 {
		t.Fatalf("Load() returned %d lines, want 1", len(loaded.Lines))
	}
	if loaded.Lines[0].Money.Overridden {
		t.Errorf("Load() = %+v, want an unconfirmed line", loaded.Lines[0])
	}

	var entries int
	if err := db.QueryRow("SELECT count(*) FROM month_entry").Scan(&entries); err != nil {
		t.Fatalf("count month_entry: %v", err)
	}
	if entries != 0 {
		t.Errorf("month_entry has %d rows, want none for a month nobody touched", entries)
	}
}
