package month

import (
	"database/sql"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

// CategoryTotal is one category's year total for the breakdown chart: ARS
// expense lines only, since a single figure can't mix currencies and income
// belongs in the net totals, not a spend breakdown.
type CategoryTotal struct {
	Category catalog.Category
	Total    decimal.Decimal
}

// Year is one calendar year's resolved data for the read-only Year view:
// each period's Month for the grid, category totals for the breakdown, and
// net worth / leftover pesos series for the sparklines.
type Year struct {
	Periods    []domain.Period
	Months     []Month
	Categories []CategoryTotal
	NetWorth   []NetWorth
	Leftover   []decimal.Decimal
}

// LoadYear resolves every period of year, plus the net worth and leftover
// pesos series folded from opening balances through each one.
func LoadYear(db *sql.DB, year int) (Year, error) {
	periods := make([]domain.Period, 12)
	for i := range periods {
		periods[i] = domain.NewPeriod(year, time.Month(i+1))
	}

	months := make([]Month, len(periods))
	for i, p := range periods {
		m, err := Load(db, p)
		if err != nil {
			return Year{}, err
		}
		months[i] = m
	}

	categories, err := catalog.Categories(db)
	if err != nil {
		return Year{}, err
	}
	opening, err := catalog.LoadOpeningBalances(db)
	if err != nil {
		return Year{}, err
	}
	allocations, err := catalog.AllSavingAllocations(db)
	if err != nil {
		return Year{}, err
	}
	rates, err := catalog.FxRates(db)
	if err != nil {
		return Year{}, err
	}
	monthlyNets, err := MonthlyNetsARS(db, opening.Period, periods[len(periods)-1])
	if err != nil {
		return Year{}, err
	}

	netWorth := make([]NetWorth, len(periods))
	leftover := make([]decimal.Decimal, len(periods))
	for i, p := range periods {
		if netWorth[i], err = ResolveNetWorth(p, opening, allocations, rates); err != nil {
			return Year{}, err
		}
		if leftover[i], err = ResolveLeftoverPesos(p, opening, monthlyNets, allocations, rates); err != nil {
			return Year{}, err
		}
	}

	return Year{
		Periods:    periods,
		Months:     months,
		Categories: categoryTotals(categories, months),
		NetWorth:   netWorth,
		Leftover:   leftover,
	}, nil
}

// categoryTotals sums each category's ARS expense lines across months,
// dropping categories with nothing to show — Income lines and other
// currencies never contribute, so an income-only category ends up empty.
func categoryTotals(categories []catalog.Category, months []Month) []CategoryTotal {
	var lines []Line
	for _, m := range months {
		lines = append(lines, m.Lines...)
	}
	return CategoryTotals(categories, lines)
}

// CategoryTotals sums lines' ARS expense amounts by category, dropping
// categories with nothing to show. Shared by the Year breakdown (every
// month's lines pooled together) and the Month view's own single-period
// breakdown, which passes just that one period's lines.
func CategoryTotals(categories []catalog.Category, lines []Line) []CategoryTotal {
	sums := make(map[int64]decimal.Decimal, len(categories))
	for _, l := range lines {
		if l.Concept.Kind == catalog.Income || l.Concept.Currency != domain.ARS {
			continue
		}
		sums[l.Concept.CategoryID] = sums[l.Concept.CategoryID].Add(l.Amount)
	}
	var totals []CategoryTotal
	for _, c := range categories {
		if sum, ok := sums[c.ID]; ok && !sum.IsZero() {
			totals = append(totals, CategoryTotal{Category: c, Total: sum})
		}
	}
	return totals
}
