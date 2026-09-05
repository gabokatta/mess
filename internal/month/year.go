package month

import (
	"database/sql"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

// CategoryTotal is one category's confirmed spend across a year, in ARS.
type CategoryTotal struct {
	Category catalog.Category
	Total    decimal.Decimal
}

// USD sums monthly conversions at each month's rate, not one conversion of
// the annual ARS total.
type Figure struct {
	ARS decimal.Decimal
	USD decimal.Decimal
}

func (f Figure) add(ars decimal.Decimal, rate Rate) Figure {
	return Figure{ARS: f.ARS.Add(ars), USD: f.USD.Add(usd(ars, rate))}
}

func (f Figure) sub(g Figure) Figure {
	return Figure{ARS: f.ARS.Sub(g.ARS), USD: f.USD.Sub(g.USD)}
}

// MonthTotals is one month of the year, every figure folded to ARS at that
// month's rate. Confirmed says whether the month holds anything at all, so a
// month nobody has ticked yet reads as pending rather than as a real zero.
type MonthTotals struct {
	Period    domain.Period
	Earned    decimal.Decimal
	Spent     decimal.Decimal
	Saved     decimal.Decimal
	Confirmed bool
}

func (t MonthTotals) Pocket() decimal.Decimal {
	return t.Earned.Sub(t.Spent).Sub(t.Saved)
}

// Year is one calendar year on one screen: what each month earned, spent and
// saved, and where the spending went. Confirmed lines only.
type Year struct {
	Year   int
	Months []MonthTotals // always twelve, in calendar order

	Earned Figure
	Spent  Figure
	Saved  Figure
	Pocket Figure

	// Categories is the year's spend ranked high to low, so the screen reads
	// as a ranking without re-sorting it.
	Categories []CategoryTotal

	// Excluded counts confirmed lines dropped across the year for want of a
	// rate; counting them as zero would understate it.
	Excluded int
}

// Overspent reports that a negative Pocket came from spending past what was
// earned, rather than from saving more than the year had left over.
func (y Year) Overspent() bool { return y.Earned.ARS.LessThan(y.Spent.ARS) }

// Confirmed is how many of the twelve months hold anything.
func (y Year) Confirmed() int {
	n := 0
	for _, t := range y.Months {
		if t.Confirmed {
			n++
		}
	}
	return n
}

func LoadYear(db *sql.DB, year int, fx FxTable) (Year, error) {
	categories, err := catalog.Categories(db)
	if err != nil {
		return Year{}, err
	}
	concepts, err := catalog.Concepts(db)
	if err != nil {
		return Year{}, err
	}
	entries, err := catalog.MonthEntriesBetween(db,
		domain.NewPeriod(year, time.January), domain.NewPeriod(year, time.December))
	if err != nil {
		return Year{}, err
	}
	byPeriod := make(map[domain.Period]map[int64]catalog.MonthEntry, 12)
	for _, entry := range entries {
		if byPeriod[entry.Period] == nil {
			byPeriod[entry.Period] = make(map[int64]catalog.MonthEntry)
		}
		byPeriod[entry.Period][entry.ConceptID] = entry
	}

	y := Year{Year: year, Months: make([]MonthTotals, 12)}
	spendByCategory := make(map[int64]decimal.Decimal)

	for i := range y.Months {
		p := domain.NewPeriod(year, time.Month(i+1))
		rate := fx.At(p)

		t := MonthTotals{Period: p}
		y.Excluded += eachConfirmedARS(Resolve(p, concepts, byPeriod[p]), rate, func(l Line, ars decimal.Decimal) {
			t.Confirmed = true
			switch l.Concept.Kind {
			case catalog.Income:
				t.Earned = t.Earned.Add(ars)
			case catalog.Expense:
				t.Spent = t.Spent.Add(ars)
				spendByCategory[l.Concept.CategoryID] = spendByCategory[l.Concept.CategoryID].Add(ars)
			case catalog.Saving:
				t.Saved = t.Saved.Add(ars)
			}
		})

		y.Months[i] = t
		y.Earned = y.Earned.add(t.Earned, rate)
		y.Spent = y.Spent.add(t.Spent, rate)
		y.Saved = y.Saved.add(t.Saved, rate)
	}
	y.Pocket = y.Earned.sub(y.Spent).sub(y.Saved)

	for _, c := range categories {
		if total, ok := spendByCategory[c.ID]; ok && !total.IsZero() {
			y.Categories = append(y.Categories, CategoryTotal{Category: c, Total: total})
		}
	}
	sort.SliceStable(y.Categories, func(i, j int) bool {
		return y.Categories[i].Total.GreaterThan(y.Categories[j].Total)
	})
	return y, nil
}
