package catalog

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
)

func TestSaveFxCloseInsertsWhenAbsent(t *testing.T) {
	db := openTestStore(t).DB()
	august := domain.NewPeriod(2026, time.August)

	if err := SaveFxClose(db, august, decimal.NewFromInt(1200)); err != nil {
		t.Fatalf("SaveFxClose() unexpected error: %v", err)
	}

	got, err := FxRates(db)
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	if len(got) != 1 || !got[0].Value.Equal(decimal.NewFromInt(1200)) || got[0].Source != Close {
		t.Fatalf("FxRates() = %+v, want a single Close 1200 row", got)
	}
}

// Backfill never overwrites: a stored close is final, and a rate you set by
// hand is never replaced by an automatic one.
func TestSaveFxCloseNeverOverwrites(t *testing.T) {
	db := openTestStore(t).DB()
	august := domain.NewPeriod(2026, time.August)

	if err := SetManualFxRate(db, august, decimal.NewFromInt(1300)); err != nil {
		t.Fatalf("SetManualFxRate() unexpected error: %v", err)
	}
	if err := SaveFxClose(db, august, decimal.NewFromInt(1200)); err != nil {
		t.Fatalf("SaveFxClose() unexpected error: %v", err)
	}

	got, err := FxRates(db)
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	if len(got) != 1 || !got[0].Value.Equal(decimal.NewFromInt(1300)) || got[0].Source != Manual {
		t.Fatalf("FxRates() = %+v, want the manual 1300 row untouched", got)
	}
}

func TestSetManualFxRateOverwritesAStoredClose(t *testing.T) {
	db := openTestStore(t).DB()
	august := domain.NewPeriod(2026, time.August)

	if err := SaveFxClose(db, august, decimal.NewFromInt(1200)); err != nil {
		t.Fatalf("SaveFxClose() unexpected error: %v", err)
	}
	if err := SetManualFxRate(db, august, decimal.NewFromInt(1450)); err != nil {
		t.Fatalf("SetManualFxRate() unexpected error: %v", err)
	}

	got, err := FxRates(db)
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	if len(got) != 1 || !got[0].Value.Equal(decimal.NewFromInt(1450)) || got[0].Source != Manual {
		t.Fatalf("FxRates() = %+v, want a single Manual 1450 row", got)
	}
}

func TestFxRatesComeBackInPeriodOrder(t *testing.T) {
	db := openTestStore(t).DB()
	for _, m := range []time.Month{time.March, time.January, time.February} {
		if err := SaveFxClose(db, domain.NewPeriod(2026, m), decimal.NewFromInt(1000)); err != nil {
			t.Fatalf("SaveFxClose(%s) unexpected error: %v", m, err)
		}
	}

	got, err := FxRates(db)
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	for i, want := range []time.Month{time.January, time.February, time.March} {
		if got[i].Period.Month() != want {
			t.Fatalf("FxRates()[%d] = %s, want %s", i, got[i].Period, want)
		}
	}
}
