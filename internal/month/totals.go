package month

import (
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mes/internal/catalog"
	"github.com/gabokatta/mes/internal/domain"
)

// Net is a net amount in one currency at two altitudes: the full household
// cost or income, and your share of it.
type Net struct {
	Household decimal.Decimal
	Share     decimal.Decimal
}

// Totals is the month header's summary: net income minus expenses,
// projected (every resolved line) versus confirmed (only lines backed by an
// override), grouped by currency since an amount never sums across one.
type Totals struct {
	Projected map[domain.Currency]Net
	Confirmed map[domain.Currency]Net
}

// ResolveTotals folds lines into projected and confirmed net totals per
// currency. Income adds, expenses subtract; each line's share is computed
// with its own concept's Share so household and your-share stay separate.
func ResolveTotals(lines []Line) Totals {
	totals := Totals{Projected: map[domain.Currency]Net{}, Confirmed: map[domain.Currency]Net{}}
	for _, l := range lines {
		signed := l.Amount
		if l.Concept.Kind != catalog.Income {
			signed = signed.Neg()
		}
		share := domain.NewMoney(signed, l.Concept.Currency).Share(l.Concept.Share).Amount()
		delta := Net{Household: signed, Share: share}

		addNet(totals.Projected, l.Concept.Currency, delta)
		if l.Confirmed {
			addNet(totals.Confirmed, l.Concept.Currency, delta)
		}
	}
	return totals
}

func addNet(totals map[domain.Currency]Net, cur domain.Currency, delta Net) {
	n := totals[cur]
	n.Household = n.Household.Add(delta.Household)
	n.Share = n.Share.Add(delta.Share)
	totals[cur] = n
}
