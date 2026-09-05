package month

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
	"github.com/gabokatta/mess/internal/testutil"
)

func yearWorld() fixture.World {
	return fixture.World{
		Concepts: []fixture.Concept{
			{Name: "Salary", Category: "Earnings", Kind: catalog.Income},
			{Name: "Rent", Category: "Home", Kind: catalog.Expense},
			{Name: "Gas", Category: "Utilities", Kind: catalog.Expense},
			{Name: "Dollars", Category: "Home", Kind: catalog.Saving, Currency: domain.USD},
		},
		Entries: []fixture.Entry{
			{Concept: "Salary", Period: period(time.January), Amount: "1000000"},
			{Concept: "Rent", Period: period(time.January), Amount: "700000"},
			{Concept: "Gas", Period: period(time.January), Amount: "30000"},
			{Concept: "Salary", Period: period(time.February), Amount: "1200000"},
			{Concept: "Rent", Period: period(time.February), Amount: "750000"},
			{Concept: "Dollars", Period: period(time.February), Amount: "400"},
		},
	}
}

func loadYearWorld(t *testing.T) Year {
	t.Helper()
	db := testutil.DB(t)
	testutil.MustLoad(t, db, yearWorld())

	fx := NewFxTable([]catalog.FxRate{closeAt(time.January, 1000), closeAt(time.February, 1200)},
		decimal.Decimal{}, false, domain.NewPeriod(fixture.Year, time.December))

	y, err := LoadYear(db, fixture.Year, fx)
	if err != nil {
		t.Fatalf("LoadYear() unexpected error: %v", err)
	}
	return y
}

func TestLoadYearFoldsEachMonthAtItsOwnRate(t *testing.T) {
	y := loadYearWorld(t)

	if len(y.Months) != 12 {
		t.Fatalf("LoadYear() returned %d months, want 12", len(y.Months))
	}
	jan, feb := y.Months[0], y.Months[1]

	if !jan.Spent.Equal(decimal.NewFromInt(730000)) {
		t.Errorf("jan.Spent = %s, want 730000", jan.Spent)
	}
	if !feb.Saved.Equal(decimal.NewFromInt(480000)) {
		t.Errorf("feb.Saved = %s, want 480000 (400 USD at 1200)", feb.Saved)
	}
	// jan 1.000.000 - 730.000 = 270.000; feb 1.200.000 - 750.000 - 480.000 = -30.000.
	if !jan.Pocket().Equal(decimal.NewFromInt(270000)) {
		t.Errorf("jan.Pocket() = %s, want 270000", jan.Pocket())
	}
	if !feb.Pocket().Equal(decimal.NewFromInt(-30000)) {
		t.Errorf("feb.Pocket() = %s, want -30000 (february over-saved)", feb.Pocket())
	}
}

func TestYearUSDConvertsMonthByMonth(t *testing.T) {
	y := loadYearWorld(t)

	if !y.Spent.ARS.Equal(decimal.NewFromInt(1480000)) {
		t.Errorf("Spent.ARS = %s, want 1480000", y.Spent.ARS)
	}
	if !y.Spent.USD.Equal(decimal.NewFromInt(1355)) {
		t.Errorf("Spent.USD = %s, want 1355 (730 + 625)", y.Spent.USD)
	}
	if !y.Saved.USD.Equal(decimal.NewFromInt(400)) {
		t.Errorf("Saved.USD = %s, want 400", y.Saved.USD)
	}
	if !y.Earned.USD.Equal(decimal.NewFromInt(2000)) {
		t.Errorf("Earned.USD = %s, want 2000 (1000 + 1000)", y.Earned.USD)
	}
	// 2.200.000 - 1.480.000 - 480.000, and 2000 - 1355 - 400 in dollars.
	if !y.Pocket.ARS.Equal(decimal.NewFromInt(240000)) {
		t.Errorf("Pocket.ARS = %s, want 240000", y.Pocket.ARS)
	}
	if !y.Pocket.USD.Equal(decimal.NewFromInt(245)) {
		t.Errorf("Pocket.USD = %s, want 245", y.Pocket.USD)
	}
}

func TestYearConfirmedCountsOnlyMonthsWithLines(t *testing.T) {
	y := loadYearWorld(t)

	if got := y.Confirmed(); got != 2 {
		t.Errorf("Confirmed() = %d, want 2", got)
	}
	if y.Months[2].Confirmed {
		t.Errorf("march reports confirmed, want pending")
	}
	if got := y.Months[5].Period; !got.Equal(domain.NewPeriod(fixture.Year, time.June)) {
		t.Errorf("Months[5].Period = %s, want june", got)
	}
}

func TestYearCategoriesRankHighToLow(t *testing.T) {
	y := loadYearWorld(t)

	want := []struct {
		name  string
		total int64
	}{
		{"Home", 1450000},
		{"Utilities", 30000},
	}
	if len(y.Categories) != len(want) {
		t.Fatalf("Categories = %+v, want %d entries", y.Categories, len(want))
	}
	for i, w := range want {
		got := y.Categories[i]
		if got.Category.Name != w.name {
			t.Errorf("Categories[%d] = %q, want %q (ranked high to low)", i, got.Category.Name, w.name)
		}
		if !got.Total.Equal(decimal.NewFromInt(w.total)) {
			t.Errorf("%s total = %s, want %d", w.name, got.Total, w.total)
		}
	}
}

func TestLoadYearWithNothingConfirmed(t *testing.T) {
	db := testutil.DB(t)
	testutil.MustLoad(t, db, fixture.World{
		Concepts: []fixture.Concept{
			{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "785000"},
		},
	})

	y, err := LoadYear(db, fixture.Year, NewFxTable(nil, decimal.Decimal{}, false, fixture.Period))
	if err != nil {
		t.Fatalf("LoadYear() unexpected error: %v", err)
	}
	if got := y.Confirmed(); got != 0 {
		t.Errorf("Confirmed() = %d, want 0", got)
	}
	if !y.Spent.ARS.IsZero() || !y.Spent.USD.IsZero() {
		t.Errorf("Spent = %+v, want zero in both currencies", y.Spent)
	}
	if !y.Saved.ARS.IsZero() {
		t.Errorf("Saved.ARS = %s, want zero", y.Saved.ARS)
	}
	if len(y.Categories) != 0 {
		t.Errorf("Categories = %+v, want none", y.Categories)
	}
}

func TestLoadYearKeepsEntriesInTheirCalendarYear(t *testing.T) {
	db := testutil.DB(t)
	jan := domain.NewPeriod(fixture.Year, time.January)
	dec := domain.NewPeriod(fixture.Year, time.December)
	testutil.MustLoad(t, db, fixture.World{
		Concepts: []fixture.Concept{{Name: "Rent", Category: "Home", Kind: catalog.Expense, From: jan.AddMonths(-1)}},
		Entries: []fixture.Entry{
			{Concept: "Rent", Period: jan.AddMonths(-1), Amount: "9000"},
			{Concept: "Rent", Period: jan, Amount: "100"},
			{Concept: "Rent", Period: dec, Amount: "200"},
			{Concept: "Rent", Period: dec.AddMonths(1), Amount: "9000"},
		},
	})
	y, err := LoadYear(db, fixture.Year, FxTable{})
	if err != nil {
		t.Fatal(err)
	}
	if !y.Spent.ARS.Equal(decimal.NewFromInt(300)) || y.Confirmed() != 2 {
		t.Fatalf("year spent = %s across %d confirmed months, want 300 across 2", y.Spent.ARS, y.Confirmed())
	}
}
