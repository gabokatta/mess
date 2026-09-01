package month

import (
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

// NetWorth is opening USD balances plus every allocation up to and
// including a period, split by destination. It is USD-only by product
// decision: leftover pesos are reported separately and never blend in.
type NetWorth struct {
	Cash     decimal.Decimal
	Invested decimal.Decimal
}

func (n NetWorth) Total() decimal.Decimal { return n.Cash.Add(n.Invested) }

// ResolveNetWorth folds allocations from opening.Period through period, so
// nothing already reflected in the opening balance folds in twice,
// converting ARS to USD at each allocation's own period rate. A missing
// rate is an error, never treated as zero.
func ResolveNetWorth(period domain.Period, opening catalog.OpeningBalances, allocations []catalog.SavingAllocation, rates []catalog.FxRate) (NetWorth, error) {
	nw := NetWorth{Cash: opening.CashUSD, Invested: opening.InvestedUSD}
	for _, a := range allocations {
		if a.Period.After(period) {
			continue
		}
		if !opening.Period.IsZero() && a.Period.Before(opening.Period) {
			continue
		}
		usd, err := toUSD(a, rates)
		if err != nil {
			return NetWorth{}, err
		}
		switch a.Destination {
		case catalog.Cash:
			nw.Cash = nw.Cash.Add(usd)
		case catalog.Invested:
			nw.Invested = nw.Invested.Add(usd)
		}
	}
	return nw, nil
}

func toUSD(a catalog.SavingAllocation, rates []catalog.FxRate) (decimal.Decimal, error) {
	if a.Currency == domain.USD {
		return a.Amount, nil
	}
	rate, err := fxRateAt(a.Period, rates)
	if err != nil {
		return decimal.Decimal{}, err
	}
	return a.Amount.Div(rate), nil
}

// fxRateAt is ResolveFxRate with a resolution failure turned into an error,
// for callers that can't proceed without a rate.
func fxRateAt(period domain.Period, rates []catalog.FxRate) (decimal.Decimal, error) {
	rate, ok := ResolveFxRate(period, rates)
	if !ok {
		return decimal.Decimal{}, fmt.Errorf("month: no fx rate at or before %s to convert allocation", period)
	}
	return rate, nil
}
