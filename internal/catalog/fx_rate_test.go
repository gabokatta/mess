package catalog_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
)

func TestSaveFxCloseInsertsWhenAbsent(t *testing.T) {
	db := fixture.DB(t)
	august := domain.NewPeriod(fixture.Year, time.August)

	if err := catalog.SaveFxClose(db, august, decimal.NewFromInt(1200)); err != nil {
		t.Fatalf("SaveFxClose() unexpected error: %v", err)
	}

	got, err := catalog.FxRates(db)
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	want := []catalog.FxRate{{Period: august, Value: decimal.NewFromInt(1200), Source: catalog.Close}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("FxRates() mismatch (-want +got):\n%s", diff)
	}
}

// Backfill never overwrites: a stored close is final, and a rate you set by
// hand is never replaced by an automatic one.
func TestSaveFxCloseNeverOverwrites(t *testing.T) {
	db := fixture.DB(t)
	august := domain.NewPeriod(fixture.Year, time.August)

	if err := catalog.SetManualFxRate(db, august, decimal.NewFromInt(1300)); err != nil {
		t.Fatalf("SetManualFxRate() unexpected error: %v", err)
	}
	if err := catalog.SaveFxClose(db, august, decimal.NewFromInt(1200)); err != nil {
		t.Fatalf("SaveFxClose() unexpected error: %v", err)
	}

	got, err := catalog.FxRates(db)
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	want := []catalog.FxRate{{Period: august, Value: decimal.NewFromInt(1300), Source: catalog.Manual}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("FxRates() mismatch (-want +got):\n%s", diff)
	}
}

func TestSetManualFxRateOverwritesAStoredClose(t *testing.T) {
	db := fixture.DB(t)
	august := domain.NewPeriod(fixture.Year, time.August)

	if err := catalog.SaveFxClose(db, august, decimal.NewFromInt(1200)); err != nil {
		t.Fatalf("SaveFxClose() unexpected error: %v", err)
	}
	if err := catalog.SetManualFxRate(db, august, decimal.NewFromInt(1450)); err != nil {
		t.Fatalf("SetManualFxRate() unexpected error: %v", err)
	}

	got, err := catalog.FxRates(db)
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	want := []catalog.FxRate{{Period: august, Value: decimal.NewFromInt(1450), Source: catalog.Manual}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("FxRates() mismatch (-want +got):\n%s", diff)
	}
}

func TestFxRatesComeBackInPeriodOrder(t *testing.T) {
	db := fixture.DB(t)
	for _, m := range []time.Month{time.March, time.January, time.February} {
		if err := catalog.SaveFxClose(db, domain.NewPeriod(fixture.Year, m), decimal.NewFromInt(1000)); err != nil {
			t.Fatalf("SaveFxClose(%s) unexpected error: %v", m, err)
		}
	}

	got, err := catalog.FxRates(db)
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	for i, want := range []time.Month{time.January, time.February, time.March} {
		if got[i].Period.Month() != want {
			t.Fatalf("FxRates()[%d] = %s, want %s", i, got[i].Period, want)
		}
	}
}
