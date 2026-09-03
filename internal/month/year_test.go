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

func TestLoadYear(t *testing.T) {
	db := fixture.DB(t)
	loaded := fixture.MustLoad(t, db, fixture.World{
		Concepts: []fixture.Concept{
			{Name: "Rent", Category: "Home", Kind: catalog.Expense},
			{Name: "Gas", Category: "Utilities", Kind: catalog.Expense},
			{Name: "Dollars", Category: "Home", Kind: catalog.Saving, Currency: domain.USD},
		},
		Entries: []fixture.Entry{
			{Concept: "Rent", Period: period(time.January), Amount: "700000"},
			{Concept: "Gas", Period: period(time.January), Amount: "30000"},
			{Concept: "Rent", Period: period(time.February), Amount: "750000"},
			{Concept: "Dollars", Period: period(time.February), Amount: "400"},
		},
	})
	dollars := loaded.Concepts["Dollars"]

	fx := NewFxTable([]catalog.FxRate{closeAt(time.January, 1000), closeAt(time.February, 1200)},
		decimal.Decimal{}, false, domain.NewPeriod(fixture.Year, time.December))

	y, err := LoadYear(db, fixture.Year, fx)
	if err != nil {
		t.Fatalf("LoadYear() unexpected error: %v", err)
	}

	if len(y.Periods) != 12 {
		t.Fatalf("LoadYear() returned %d periods, want 12", len(y.Periods))
	}
	if !y.Spent[0].Equal(decimal.NewFromInt(730000)) {
		t.Errorf("Spent[jan] = %s, want 730000", y.Spent[0])
	}
	if !y.Spent[1].Equal(decimal.NewFromInt(750000)) {
		t.Errorf("Spent[feb] = %s, want 750000", y.Spent[1])
	}
	if !y.SpentTotal.Equal(decimal.NewFromInt(1480000)) {
		t.Errorf("SpentTotal = %s, want 1480000", y.SpentTotal)
	}
	if !y.Saved[1][dollars.ID].Equal(decimal.NewFromInt(480000)) {
		t.Errorf("Saved[feb][Dollars] = %s, want 400 USD at 1200", y.Saved[1][dollars.ID])
	}
	if !y.SavedTotal.Equal(decimal.NewFromInt(480000)) {
		t.Errorf("SavedTotal = %s, want 480000", y.SavedTotal)
	}
	if diff := cmp.Diff([]catalog.Concept{dollars}, y.SavingConcepts); diff != "" {
		t.Errorf("SavingConcepts mismatch (-want +got):\n%s", diff)
	}

	byCategory := map[string]decimal.Decimal{}
	for _, c := range y.Categories {
		byCategory[c.Category.Name] = c.Total
	}
	if !byCategory["Home"].Equal(decimal.NewFromInt(1450000)) {
		t.Errorf("Home total = %s, want 1450000", byCategory["Home"])
	}
	if !byCategory["Utilities"].Equal(decimal.NewFromInt(30000)) {
		t.Errorf("Utilities total = %s, want 30000", byCategory["Utilities"])
	}
}

// Every figure resolves from the periods on screen; nothing accumulates from
// an opening anchor.
func TestLoadYearWithNothingConfirmed(t *testing.T) {
	db := fixture.DB(t)
	fixture.MustLoad(t, db, fixture.World{
		Concepts: []fixture.Concept{
			{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "785000"},
		},
	})

	y, err := LoadYear(db, fixture.Year, NewFxTable(nil, decimal.Decimal{}, false, fixture.Period))
	if err != nil {
		t.Fatalf("LoadYear() unexpected error: %v", err)
	}
	if !y.SpentTotal.IsZero() {
		t.Errorf("SpentTotal = %s, want zero", y.SpentTotal)
	}
	if !y.SavedTotal.IsZero() {
		t.Errorf("SavedTotal = %s, want zero", y.SavedTotal)
	}
	if len(y.Categories) != 0 {
		t.Errorf("Categories = %+v, want none", y.Categories)
	}
}
