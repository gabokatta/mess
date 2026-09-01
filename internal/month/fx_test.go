package month

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mes/internal/catalog"
	"github.com/gabokatta/mes/internal/domain"
)

func fxRate(period domain.Period, value int64, source catalog.FxSource) catalog.FxRate {
	return catalog.FxRate{Period: period, Value: decimal.NewFromInt(value), Source: source}
}

func TestResolveFxRateUsesExactPeriodMatch(t *testing.T) {
	sept := domain.NewPeriod(2026, time.September)
	rates := []catalog.FxRate{
		fxRate(domain.NewPeriod(2026, time.August), 1150, catalog.Fetched),
		fxRate(sept, 1200, catalog.Fetched),
	}

	got, ok := ResolveFxRate(sept, rates)

	if !ok {
		t.Fatal("ResolveFxRate() ok = false, want true")
	}
	if !got.Equal(decimal.NewFromInt(1200)) {
		t.Errorf("ResolveFxRate() = %s, want 1200 (September's own rate)", got)
	}
}

func TestResolveFxRateFallsBackToLastKnownBefore(t *testing.T) {
	sept := domain.NewPeriod(2026, time.September)
	rates := []catalog.FxRate{
		fxRate(domain.NewPeriod(2026, time.June), 1100, catalog.Fetched),
		fxRate(domain.NewPeriod(2026, time.August), 1150, catalog.Fetched),
	}

	got, ok := ResolveFxRate(sept, rates)

	if !ok {
		t.Fatal("ResolveFxRate() ok = false, want true")
	}
	if !got.Equal(decimal.NewFromInt(1150)) {
		t.Errorf("ResolveFxRate() = %s, want 1150 (August, the latest before September)", got)
	}
}

func TestResolveFxRateIgnoresFutureRates(t *testing.T) {
	march := domain.NewPeriod(2026, time.March)
	rates := []catalog.FxRate{fxRate(domain.NewPeriod(2026, time.June), 1100, catalog.Fetched)}

	_, ok := ResolveFxRate(march, rates)

	if ok {
		t.Error("ResolveFxRate() ok = true, want false (only rate is in the future)")
	}
}

func TestResolveFxRateNoRatesIsNotFound(t *testing.T) {
	_, ok := ResolveFxRate(domain.NewPeriod(2026, time.September), nil)
	if ok {
		t.Error("ResolveFxRate() ok = true, want false (no rates at all)")
	}
}
