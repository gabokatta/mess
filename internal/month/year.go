package month

import (
	"database/sql"
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

// Year is one calendar year on one screen: what each month saved, spent, and
// where it went. Confirmed lines only, folded to ARS at each period's rate.
type Year struct {
	Year    int
	Periods []domain.Period

	SavingConcepts []catalog.Concept
	Saved          []map[int64]decimal.Decimal
	SavedTotal     decimal.Decimal

	Spent      []decimal.Decimal
	SpentTotal decimal.Decimal
	Categories []CategoryTotal
}

func LoadYear(db *sql.DB, year int, fx FxTable) (Year, error) {
	categories, err := catalog.Categories(db)
	if err != nil {
		return Year{}, err
	}

	y := Year{
		Year:    year,
		Periods: make([]domain.Period, 12),
		Saved:   make([]map[int64]decimal.Decimal, 12),
		Spent:   make([]decimal.Decimal, 12),
	}
	spendByCategory := make(map[int64]decimal.Decimal)
	seenSaving := make(map[int64]bool)

	for i := range y.Periods {
		p := domain.NewPeriod(year, time.Month(i+1))
		y.Periods[i] = p

		loaded, err := Load(db, p)
		if err != nil {
			return Year{}, err
		}
		saved := make(map[int64]decimal.Decimal)
		eachConfirmedARS(loaded.Lines, fx.At(p), func(l Line, ars decimal.Decimal) {
			switch l.Concept.Kind {
			case catalog.Saving:
				saved[l.Concept.ID] = saved[l.Concept.ID].Add(ars)
				y.SavedTotal = y.SavedTotal.Add(ars)
				if !seenSaving[l.Concept.ID] {
					seenSaving[l.Concept.ID] = true
					y.SavingConcepts = append(y.SavingConcepts, l.Concept)
				}
			case catalog.Expense:
				y.Spent[i] = y.Spent[i].Add(ars)
				y.SpentTotal = y.SpentTotal.Add(ars)
				spendByCategory[l.Concept.CategoryID] = spendByCategory[l.Concept.CategoryID].Add(ars)
			}
		})
		y.Saved[i] = saved
	}

	for _, c := range categories {
		if total, ok := spendByCategory[c.ID]; ok && !total.IsZero() {
			y.Categories = append(y.Categories, CategoryTotal{Category: c, Total: total})
		}
	}
	return y, nil
}
