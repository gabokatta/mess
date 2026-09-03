package month

import (
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

// Totals is the Month header's arithmetic, every figure in ARS. Pocket may
// be negative: over-saving is legal and shown, not refused.
type Totals struct {
	Available domain.Money
	Saved     domain.Money
	Pocket    domain.Money

	// Excluded counts confirmed lines dropped for want of a rate; counting
	// them as zero would understate the month.
	Excluded int
}

// ResolveTotals folds the confirmed lines into one ARS figure each; an
// untyped baseline is a guess and does not count.
func ResolveTotals(lines []Line, rate Rate) Totals {
	var available, saved decimal.Decimal

	excluded := eachConfirmedARS(lines, rate, func(l Line, ars decimal.Decimal) {
		switch l.Concept.Kind {
		case catalog.Income:
			available = available.Add(ars)
		case catalog.Expense:
			available = available.Sub(ars)
		case catalog.Saving:
			saved = saved.Add(ars)
		}
	})

	return Totals{
		Available: domain.NewMoney(available, domain.ARS),
		Saved:     domain.NewMoney(saved, domain.ARS),
		Pocket:    domain.NewMoney(available.Sub(saved), domain.ARS),
		Excluded:  excluded,
	}
}

func eachConfirmedARS(lines []Line, rate Rate, fn func(Line, decimal.Decimal)) int {
	excluded := 0
	for _, l := range lines {
		if l.Money == nil || !l.Money.Confirmed {
			continue
		}
		ars, ok := l.Money.Amount.ToARS(rate.Value, rate.OK())
		if !ok {
			excluded++
			continue
		}
		fn(l, ars.Amount())
	}
	return excluded
}

// AvailableUSD is Available back in dollars, zero when there is no rate to
// divide by.
func (t Totals) AvailableUSD(rate Rate) decimal.Decimal { return usd(t.Available.Amount(), rate) }

// SavedUSD is Saved back in dollars, zero when there is no rate to divide by.
func (t Totals) SavedUSD(rate Rate) decimal.Decimal { return usd(t.Saved.Amount(), rate) }

// PocketUSD is Pocket back in dollars, zero when there is no rate to divide
// by. It carries Pocket's sign, so an over-saved month divides negative too.
func (t Totals) PocketUSD(rate Rate) decimal.Decimal { return usd(t.Pocket.Amount(), rate) }

func usd(amount decimal.Decimal, rate Rate) decimal.Decimal {
	if !rate.OK() || rate.Value.IsZero() {
		return decimal.Decimal{}
	}
	return amount.Div(rate.Value)
}
