package month

import (
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
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
// A Chore line carries no Money, so it doesn't contribute.
func ResolveTotals(lines []Line) Totals {
	totals := Totals{Projected: map[domain.Currency]Net{}, Confirmed: map[domain.Currency]Net{}}
	for _, l := range lines {
		if l.Money == nil {
			continue
		}
		signed := l.Money.Amount
		if l.Concept.Kind != catalog.Income {
			signed = signed.Neg()
		}
		share := domain.NewMoney(signed, l.Concept.Money.Currency).Share(l.Concept.Money.Share).Amount()
		delta := Net{Household: signed, Share: share}

		addNet(totals.Projected, l.Concept.Money.Currency, delta)
		if l.Money.Confirmed {
			addNet(totals.Confirmed, l.Concept.Money.Currency, delta)
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

// HeaderNet is the month header's single confirmed figure: every confirmed
// line folded into one ARS net, plus how many of the month's lines are
// confirmed at all — what's left to check, not a projected total nobody
// acts on.
type HeaderNet struct {
	Net       Net
	Confirmed int
	Lines     int
}

// ResolveHeaderNet folds every confirmed money line into one ARS net,
// converting a non-ARS line at rate — the same read-time, never-persisted
// conversion the allocation panel already does. Lines/Confirmed count money
// lines only: a Chore has nothing to confirm, and gets its own "X of Y
// chores done" count instead. A non-ARS line is skipped from Net (not
// dropped from Lines/Confirmed) when hasRate is false, since there's nothing
// to convert it with yet.
func ResolveHeaderNet(lines []Line, rate decimal.Decimal, hasRate bool) HeaderNet {
	var h HeaderNet
	for _, l := range lines {
		if l.Money == nil {
			continue
		}
		h.Lines++
		if !l.Money.Confirmed {
			continue
		}
		h.Confirmed++

		signed := l.Money.Amount
		if l.Concept.Kind != catalog.Income {
			signed = signed.Neg()
		}
		ars := signed
		if l.Concept.Money.Currency != domain.ARS {
			if !hasRate {
				continue
			}
			ars = signed.Mul(rate)
		}
		share := domain.NewMoney(ars, domain.ARS).Share(l.Concept.Money.Share).Amount()
		h.Net.Household = h.Net.Household.Add(ars)
		h.Net.Share = h.Net.Share.Add(share)
	}
	return h
}
