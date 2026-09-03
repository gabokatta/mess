package month

import (
	"database/sql"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func confirm(t *testing.T, db *sql.DB, conceptID int64, p domain.Period, amount int64) {
	t.Helper()
	value := decimal.NewFromInt(amount)
	if err := catalog.SetMonthEntryAmount(db, conceptID, p, &value); err != nil {
		t.Fatalf("SetMonthEntryAmount() unexpected error: %v", err)
	}
}

func TestLoadYear(t *testing.T) {
	db := openTestStore(t)

	rent := seedConcept(t, db, "Home", catalog.Concept{
		Name: "Rent", Kind: catalog.Expense,
		Money: &catalog.MoneyDetails{Currency: domain.ARS},
	})
	gas := seedConcept(t, db, "Utilities", catalog.Concept{
		Name: "Gas", Kind: catalog.Expense,
		Money: &catalog.MoneyDetails{Currency: domain.ARS},
	})
	dollars := seedConcept(t, db, "Home", catalog.Concept{
		Name: "Dollars", Kind: catalog.Saving,
		Money: &catalog.MoneyDetails{Currency: domain.USD},
	})

	january := domain.NewPeriod(2026, time.January)
	february := domain.NewPeriod(2026, time.February)
	confirm(t, db, rent.ID, january, 700000)
	confirm(t, db, gas.ID, january, 30000)
	confirm(t, db, rent.ID, february, 750000)
	confirm(t, db, dollars.ID, february, 400)

	fx := NewFxTable([]catalog.FxRate{closeAt(time.January, 1000), closeAt(time.February, 1200)},
		decimal.Decimal{}, false, domain.NewPeriod(2026, time.December))

	y, err := LoadYear(db, 2026, fx)
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
	if len(y.SavingConcepts) != 1 || y.SavingConcepts[0].ID != dollars.ID {
		t.Errorf("SavingConcepts = %+v, want just Dollars", y.SavingConcepts)
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

// Every figure resolves from the periods on screen; nothing accumulates
// from an opening anchor, so a year with nothing confirmed reads as zero.
func TestLoadYearWithNothingConfirmed(t *testing.T) {
	db := openTestStore(t)
	seedConcept(t, db, "Home", catalog.Concept{
		Name: "Rent", Kind: catalog.Expense,
		Money: &catalog.MoneyDetails{Currency: domain.ARS, Base: decimal.NewFromInt(785000)},
	})

	y, err := LoadYear(db, 2026, NewFxTable(nil, decimal.Decimal{}, false, domain.NewPeriod(2026, time.September)))
	if err != nil {
		t.Fatalf("LoadYear() unexpected error: %v", err)
	}
	if !y.SpentTotal.IsZero() || !y.SavedTotal.IsZero() || len(y.Categories) != 0 {
		t.Errorf("LoadYear() = %+v, want everything zero", y)
	}
}
