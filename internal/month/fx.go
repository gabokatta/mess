package month

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

// RateOrigin is where a period's rate came from, shown so a stale rate is
// visible rather than silently in effect.
type RateOrigin int

const (
	RateNone RateOrigin = iota
	RateLive
	RateClose
	RateManual
	RateInherited
)

// Rate is the dollar rate in effect for one period.
type Rate struct {
	Value  decimal.Decimal
	Origin RateOrigin
	From   domain.Period // where an inherited rate came from
}

func (r Rate) OK() bool { return r.Origin != RateNone }

func (r Rate) Label() string {
	switch r.Origin {
	case RateLive:
		return "live"
	case RateClose:
		return "close"
	case RateManual:
		return "manual"
	case RateInherited:
		return "inherited from " + r.From.String()
	default:
		return "no rate"
	}
}

// FxTable is every period's rate at once: stored closes plus today's quote
// for the running month. Conversion is read-time, so corrections cascade.
type FxTable struct {
	stored  []catalog.FxRate
	live    decimal.Decimal
	hasLive bool
	today   domain.Period
}

// NewFxTable takes stored rows in catalog.FxRates' ascending order plus
// today's quote; hasLive is false when the fetch failed or has not landed.
func NewFxTable(stored []catalog.FxRate, live decimal.Decimal, hasLive bool, today domain.Period) FxTable {
	return FxTable{stored: stored, live: live, hasLive: hasLive, today: today}
}

// At resolves period's rate: manual wins outright, the running month falls
// to today's quote, and a close-less completed month inherits the last one.
func (t FxTable) At(period domain.Period) Rate {
	var inherited Rate
	for _, r := range t.stored {
		if r.Period.After(period) {
			break
		}
		if r.Period.Equal(period) {
			if r.Source == catalog.Manual {
				return Rate{Value: r.Value, Origin: RateManual}
			}
			return Rate{Value: r.Value, Origin: RateClose}
		}
		inherited = Rate{Value: r.Value, Origin: RateInherited, From: r.Period}
	}
	if period.Equal(t.today) && t.hasLive {
		return Rate{Value: t.live, Origin: RateLive}
	}
	return inherited
}

// MissingCloses is the backfill list: every completed month of year with no
// stored rate, so a stored close is never refetched.
func MissingCloses(year int, today domain.Period, stored []catalog.FxRate) []domain.Period {
	have := make(map[domain.Period]bool, len(stored))
	for _, r := range stored {
		have[r.Period] = true
	}
	var missing []domain.Period
	for m := 1; m <= 12; m++ {
		p := domain.NewPeriod(year, time.Month(m))
		if p.Before(today) && !have[p] {
			missing = append(missing, p)
		}
	}
	return missing
}
