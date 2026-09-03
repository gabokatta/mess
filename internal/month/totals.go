package month

import (
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

// Totals is the Month header's arithmetic, every figure in ARS: what came
// in minus what went out, what the Saving lines put away, and the
// remainder. Pocket may be negative — over-saving is legal and shown, not
// refused.
type Totals struct {
	Available domain.Money
	Saved     domain.Money
	Pocket    domain.Money

	// Excluded counts confirmed lines left out of the roll-up for want of a
	// rate to convert them with. Dropping them is honest; counting them as
	// zero would understate the month.
	Excluded int
}

// ResolveTotals folds the confirmed lines into one ARS figure each. Only
// confirmed lines count: a baseline you have not typed over is a guess, and
// the header states what you actually did.
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

// eachConfirmedARS is the fold every figure in mess runs on: confirmed
// lines only, converted at the period's rate, and a line that cannot be
// converted dropped rather than counted as zero. It returns how many.
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

// SavedUSD is Saved back in dollars, the figure the header shows beside it.
// Zero when there is no rate to divide by.
func (t Totals) SavedUSD(rate Rate) decimal.Decimal {
	if !rate.OK() || rate.Value.IsZero() {
		return decimal.Decimal{}
	}
	return t.Saved.Amount().Div(rate.Value)
}
