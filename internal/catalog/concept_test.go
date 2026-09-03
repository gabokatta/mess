package catalog

import (
	"database/sql"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
)

func mustCategory(t *testing.T, db *sql.DB, name string) Category {
	t.Helper()
	cat, err := CreateCategory(db, name, 0)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	return cat
}

func TestCreateAndListConcepts(t *testing.T) {
	db := openTestStore(t).DB()
	cat := mustCategory(t, db, "Home")

	c := Concept{
		Name:       "Rent",
		CategoryID: cat.ID,
		Kind:       Expense,
		Money:      &MoneyDetails{Currency: domain.ARS, Base: decimal.NewFromInt(785000)},
		MonthMask:  domain.Monthly,
		ActiveFrom: domain.NewPeriod(2026, time.January),
	}

	created, err := CreateConcept(db, c)
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}
	if created.ID == 0 {
		t.Error("CreateConcept() should assign a non-zero ID")
	}

	got, err := Concepts(db)
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Concepts() returned %d rows, want 1", len(got))
	}

	g := got[0]
	if g.ID != created.ID || g.Name != c.Name || g.CategoryID != cat.ID || g.Kind != Expense ||
		g.MonthMask != c.MonthMask || !g.ActiveFrom.Equal(c.ActiveFrom) {
		t.Errorf("Concepts()[0] = %+v, want %+v", g, created)
	}
	if g.Money.Currency != domain.ARS || !g.Money.Base.Equal(decimal.NewFromInt(785000)) {
		t.Errorf("Concepts()[0].Money = %+v, want ARS 785000", g.Money)
	}
	if !g.ActiveUntil.IsZero() {
		t.Error("ActiveUntil should round-trip as zero (unbounded) when never set")
	}
}

func TestConceptsOrderByCategoryThenName(t *testing.T) {
	db := openTestStore(t).DB()
	home, err := CreateCategory(db, "Home", 0)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	utilities, err := CreateCategory(db, "Utilities", 1)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}

	for _, c := range []Concept{
		{Name: "Water", CategoryID: utilities.ID, Kind: Expense, Money: &MoneyDetails{}, MonthMask: domain.Monthly, ActiveFrom: domain.NewPeriod(2026, time.January)},
		{Name: "Rent", CategoryID: home.ID, Kind: Expense, Money: &MoneyDetails{}, MonthMask: domain.Monthly, ActiveFrom: domain.NewPeriod(2026, time.January)},
		{Name: "Gas", CategoryID: utilities.ID, Kind: Expense, Money: &MoneyDetails{}, MonthMask: domain.Monthly, ActiveFrom: domain.NewPeriod(2026, time.January)},
	} {
		if _, err := CreateConcept(db, c); err != nil {
			t.Fatalf("CreateConcept(%s) unexpected error: %v", c.Name, err)
		}
	}

	got, err := Concepts(db)
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	want := []string{"Rent", "Gas", "Water"}
	for i, name := range want {
		if got[i].Name != name {
			t.Fatalf("Concepts() order = %s, want %s", names(got), want)
		}
	}
}

func names(concepts []Concept) []string {
	out := make([]string, len(concepts))
	for i, c := range concepts {
		out[i] = c.Name
	}
	return out
}

func TestCreateConceptRequiresExistingCategory(t *testing.T) {
	db := openTestStore(t).DB()

	c := Concept{
		Name:       "Rent",
		CategoryID: 999,
		Kind:       Expense,
		Money:      &MoneyDetails{Currency: domain.ARS},
		MonthMask:  domain.Monthly,
		ActiveFrom: domain.NewPeriod(2026, time.January),
	}
	if _, err := CreateConcept(db, c); err == nil {
		t.Error("CreateConcept() with a dangling category ID should fail the foreign key check")
	}
}

func TestUpdateConceptRetiresViaActiveUntil(t *testing.T) {
	db := openTestStore(t).DB()
	cat := mustCategory(t, db, "Home")

	created, err := CreateConcept(db, Concept{
		Name:       "Netflix",
		CategoryID: cat.ID,
		Kind:       Expense,
		Money:      &MoneyDetails{Currency: domain.ARS},
		MonthMask:  domain.Monthly,
		ActiveFrom: domain.NewPeriod(2026, time.January),
	})
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}

	created.ActiveUntil = domain.NewPeriod(2026, time.June)
	if err := UpdateConcept(db, created); err != nil {
		t.Fatalf("UpdateConcept() unexpected error: %v", err)
	}

	got, err := Concepts(db)
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	if len(got) != 1 || !got[0].ActiveUntil.Equal(domain.NewPeriod(2026, time.June)) {
		t.Errorf("Concepts()[0].ActiveUntil = %v, want 2026-06", got[0].ActiveUntil)
	}
}

// A chore's currency and base amount are NULL at the SQL boundary, and past
// Concepts() a chore carrying money is a state that does not exist.
func TestChoreRoundTripsWithoutMoney(t *testing.T) {
	db := openTestStore(t).DB()
	cat := mustCategory(t, db, "Home")

	created, err := CreateConcept(db, Concept{
		Name:       "Wash the house",
		CategoryID: cat.ID,
		Kind:       Chore,
		MonthMask:  domain.Monthly,
		ActiveFrom: domain.NewPeriod(2026, time.January),
	})
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}
	if created.Money != nil {
		t.Errorf("created.Money = %+v, want nil for a Chore", created.Money)
	}

	got, err := Concepts(db)
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Money != nil {
		t.Errorf("Concepts()[0] = %+v, want a money-less Chore round-tripped", got)
	}
}

func TestDeleteConceptTakesItsEntriesWithIt(t *testing.T) {
	db := openTestStore(t).DB()
	cat := mustCategory(t, db, "Home")
	period := domain.NewPeriod(2026, time.September)

	created, err := CreateConcept(db, Concept{
		Name:       "Rent",
		CategoryID: cat.ID,
		Kind:       Expense,
		Money:      &MoneyDetails{Currency: domain.ARS},
		MonthMask:  domain.Monthly,
		ActiveFrom: domain.NewPeriod(2026, time.January),
	})
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}
	if err := SetMonthEntryDone(db, created.ID, period, true); err != nil {
		t.Fatalf("SetMonthEntryDone() unexpected error: %v", err)
	}

	if err := DeleteConcept(db, created.ID); err != nil {
		t.Fatalf("DeleteConcept() unexpected error: %v", err)
	}

	concepts, err := Concepts(db)
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	if len(concepts) != 0 {
		t.Errorf("Concepts() = %+v, want empty", concepts)
	}
	entries, err := MonthEntries(db, period)
	if err != nil {
		t.Fatalf("MonthEntries() unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("MonthEntries() = %+v, want empty after the concept was deleted", entries)
	}
}

func TestParseConceptKind(t *testing.T) {
	tests := []struct {
		input   string
		want    ConceptKind
		wantErr bool
	}{
		{"Income", Income, false},
		{"Expense", Expense, false},
		{"Saving", Saving, false},
		{"Chore", Chore, false},
		{"Savings", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseConceptKind(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseConceptKind(%q) = nil error, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseConceptKind(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseConceptKind(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func mustConcept(t *testing.T, db *sql.DB) Concept {
	t.Helper()
	cat := mustCategory(t, db, "Home")
	c, err := CreateConcept(db, Concept{
		Name:       "Rent",
		CategoryID: cat.ID,
		Kind:       Expense,
		Money:      &MoneyDetails{Currency: domain.ARS, Base: decimal.NewFromInt(785000)},
		MonthMask:  domain.Monthly,
		ActiveFrom: domain.NewPeriod(2026, time.January),
	})
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}
	return c
}
