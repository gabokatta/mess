package month

import (
	"database/sql"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

// AvailableToSave is your confirmed share of period's net — income minus
// expenses in ARS — the figure the allocation panel starts from.
func AvailableToSave(totals Totals) decimal.Decimal {
	return totals.Confirmed[domain.ARS].Share
}

// MonthlyNetsARS resolves each period's confirmed ARS net from opening
// through period, for ResolveLeftoverPesos to fold over.
func MonthlyNetsARS(db *sql.DB, opening, period domain.Period) (map[domain.Period]decimal.Decimal, error) {
	nets := make(map[domain.Period]decimal.Decimal)
	if opening.IsZero() || opening.After(period) {
		return nets, nil
	}
	for p := opening; !p.After(period); p = p.AddMonths(1) {
		m, err := Load(db, p)
		if err != nil {
			return nil, err
		}
		nets[p] = ResolveTotals(m.Lines).Confirmed[domain.ARS].Share
	}
	return nets, nil
}

// ResolveLeftoverPesos folds (net minus that period's allocations) from
// opening.Period through period, anchored at opening.LeftoverPesos. Stays
// in ARS — it never blends into NetWorth.
func ResolveLeftoverPesos(period domain.Period, opening catalog.OpeningBalances, monthlyNets map[domain.Period]decimal.Decimal, allocations []catalog.SavingAllocation, rates []catalog.FxRate) (decimal.Decimal, error) {
	if opening.Period.IsZero() {
		return opening.LeftoverPesos, nil
	}
	total := opening.LeftoverPesos
	for p := opening.Period; !p.After(period); p = p.AddMonths(1) {
		total = total.Add(monthlyNets[p])
		spent, err := allocatedARS(p, allocations, rates)
		if err != nil {
			return decimal.Decimal{}, err
		}
		total = total.Sub(spent)
	}
	return total, nil
}

// ResolveGap is available-to-save minus what period has actually allocated,
// in ARS. Positive means unallocated remainder; negative means allocations
// exceeded it — the allocation panel shows this rather than blocking it.
func ResolveGap(available decimal.Decimal, period domain.Period, allocations []catalog.SavingAllocation, rates []catalog.FxRate) (decimal.Decimal, error) {
	allocated, err := allocatedARS(period, allocations, rates)
	if err != nil {
		return decimal.Decimal{}, err
	}
	return available.Sub(allocated), nil
}

// allocatedARS sums the ARS-equivalent of every allocation recorded in
// period, converting USD ones at that period's own rate.
func allocatedARS(period domain.Period, allocations []catalog.SavingAllocation, rates []catalog.FxRate) (decimal.Decimal, error) {
	var total decimal.Decimal
	for _, a := range allocations {
		if !a.Period.Equal(period) {
			continue
		}
		if a.Currency == domain.ARS {
			total = total.Add(a.Amount)
			continue
		}
		rate, err := fxRateAt(period, rates)
		if err != nil {
			return decimal.Decimal{}, err
		}
		total = total.Add(a.Amount.Mul(rate))
	}
	return total, nil
}
