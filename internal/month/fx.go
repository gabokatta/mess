package month

import (
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

// ResolveFxRate picks period's own rate if one was recorded, else the
// latest known rate before it. rates must be sorted ascending by Period, the
// order catalog.FxRates already returns them in. ok is false when no rate at
// or before period exists — there is no hardcoded fallback constant.
func ResolveFxRate(period domain.Period, rates []catalog.FxRate) (decimal.Decimal, bool) {
	var value decimal.Decimal
	found := false
	for _, r := range rates {
		if r.Period.After(period) {
			break
		}
		value = r.Value
		found = true
	}
	return value, found
}
