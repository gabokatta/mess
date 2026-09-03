package catalog_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
)

func TestCreateAndListConcepts(t *testing.T) {
	db := fixture.DB(t)
	cat, err := catalog.CreateCategory(db, "Home", 0)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}

	c := catalog.Concept{
		Name:       "Rent",
		CategoryID: cat.ID,
		Kind:       catalog.Expense,
		Money:      &catalog.MoneyDetails{Currency: domain.ARS, Base: decimal.NewFromInt(785000)},
		MonthMask:  domain.Monthly,
		ActiveFrom: domain.NewPeriod(fixture.Year, time.January),
	}

	created, err := catalog.CreateConcept(db, c)
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}
	if created.ID == 0 {
		t.Error("CreateConcept() should assign a non-zero ID")
	}

	got, err := catalog.Concepts(db)
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	if diff := cmp.Diff([]catalog.Concept{created}, got); diff != "" {
		t.Errorf("Concepts() mismatch (-want +got):\n%s", diff)
	}
}

func TestConceptsOrderByCategoryThenName(t *testing.T) {
	db := fixture.DB(t)
	home, err := catalog.CreateCategory(db, "Home", 0)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	utilities, err := catalog.CreateCategory(db, "Utilities", 1)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}

	for _, c := range []catalog.Concept{
		{Name: "Water", CategoryID: utilities.ID, Kind: catalog.Expense, Money: &catalog.MoneyDetails{}, MonthMask: domain.Monthly, ActiveFrom: domain.NewPeriod(fixture.Year, time.January)},
		{Name: "Rent", CategoryID: home.ID, Kind: catalog.Expense, Money: &catalog.MoneyDetails{}, MonthMask: domain.Monthly, ActiveFrom: domain.NewPeriod(fixture.Year, time.January)},
		{Name: "Gas", CategoryID: utilities.ID, Kind: catalog.Expense, Money: &catalog.MoneyDetails{}, MonthMask: domain.Monthly, ActiveFrom: domain.NewPeriod(fixture.Year, time.January)},
	} {
		if _, err := catalog.CreateConcept(db, c); err != nil {
			t.Fatalf("CreateConcept(%s) unexpected error: %v", c.Name, err)
		}
	}

	got, err := catalog.Concepts(db)
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	names := make([]string, len(got))
	for i, c := range got {
		names[i] = c.Name
	}
	if diff := cmp.Diff([]string{"Rent", "Gas", "Water"}, names); diff != "" {
		t.Errorf("Concepts() order mismatch (-want +got):\n%s", diff)
	}
}

func TestCreateConceptRequiresExistingCategory(t *testing.T) {
	db := fixture.DB(t)

	c := catalog.Concept{
		Name:       "Rent",
		CategoryID: 999,
		Kind:       catalog.Expense,
		Money:      &catalog.MoneyDetails{Currency: domain.ARS},
		MonthMask:  domain.Monthly,
		ActiveFrom: domain.NewPeriod(fixture.Year, time.January),
	}
	if _, err := catalog.CreateConcept(db, c); err == nil {
		t.Error("CreateConcept() with a dangling category ID should fail the foreign key check")
	}
}

func TestUpdateConceptRetiresViaActiveUntil(t *testing.T) {
	db := fixture.DB(t)
	loaded := fixture.MustLoad(t, db, fixture.World{
		Concepts: []fixture.Concept{{Name: "Netflix", Category: "Home", Kind: catalog.Expense}},
	})
	concept := loaded.Concepts["Netflix"]

	concept.ActiveUntil = domain.NewPeriod(fixture.Year, time.June)
	if err := catalog.UpdateConcept(db, concept); err != nil {
		t.Fatalf("UpdateConcept() unexpected error: %v", err)
	}

	got, err := catalog.Concepts(db)
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	if diff := cmp.Diff([]catalog.Concept{concept}, got); diff != "" {
		t.Errorf("Concepts() mismatch (-want +got):\n%s", diff)
	}
}

// A chore's currency and base amount are NULL at the SQL boundary, and past
// Concepts() a chore carrying money is a state that does not exist.
func TestChoreRoundTripsWithoutMoney(t *testing.T) {
	db := fixture.DB(t)
	loaded := fixture.MustLoad(t, db, fixture.World{
		Concepts: []fixture.Concept{{Name: "Wash the house", Category: "Home", Kind: catalog.Chore}},
	})
	if got := loaded.Concepts["Wash the house"].Money; got != nil {
		t.Errorf("created.Money = %+v, want nil for a Chore", got)
	}

	got, err := catalog.Concepts(db)
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	if diff := cmp.Diff([]catalog.Concept{loaded.Concepts["Wash the house"]}, got); diff != "" {
		t.Errorf("Concepts() mismatch (-want +got):\n%s", diff)
	}
}

func TestDeleteConceptTakesItsEntriesWithIt(t *testing.T) {
	db := fixture.DB(t)
	loaded := fixture.MustLoad(t, db, fixture.World{
		Concepts: []fixture.Concept{{Name: "Rent", Category: "Home", Kind: catalog.Expense}},
		Entries:  []fixture.Entry{{Concept: "Rent", Period: fixture.Period, Done: true}},
	})
	concept := loaded.Concepts["Rent"]

	if err := catalog.DeleteConcept(db, concept.ID); err != nil {
		t.Fatalf("DeleteConcept() unexpected error: %v", err)
	}

	concepts, err := catalog.Concepts(db)
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	if len(concepts) != 0 {
		t.Errorf("Concepts() = %+v, want empty", concepts)
	}
	entries, err := catalog.MonthEntries(db, fixture.Period)
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
		want    catalog.ConceptKind
		wantErr bool
	}{
		{"Income", catalog.Income, false},
		{"Expense", catalog.Expense, false},
		{"Saving", catalog.Saving, false},
		{"Chore", catalog.Chore, false},
		{"Savings", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := catalog.ParseConceptKind(tt.input)
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
