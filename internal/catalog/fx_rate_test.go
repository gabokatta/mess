package catalog_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
	"github.com/gabokatta/mess/internal/testutil"
)

func TestSaveFxCloseInsertsWhenAbsent(t *testing.T) {
	db := testutil.DB(t)
	august := domain.NewPeriod(fixture.Year, time.August)

	if err := catalog.SaveFxClose(db, august, decimal.NewFromInt(1200), domain.Blue); err != nil {
		t.Fatalf("SaveFxClose() unexpected error: %v", err)
	}

	got, err := catalog.FxRates(db)
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	blue := domain.Blue
	want := []catalog.FxRate{{Period: august, Value: decimal.NewFromInt(1200), Source: catalog.Close, House: &blue}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("FxRates() mismatch (-want +got):\n%s", diff)
	}
}

func TestSaveFxCloseNeverOverwrites(t *testing.T) {
	db := testutil.DB(t)
	august := domain.NewPeriod(fixture.Year, time.August)

	if err := catalog.SetManualFxRate(db, august, decimal.NewFromInt(1300)); err != nil {
		t.Fatalf("SetManualFxRate() unexpected error: %v", err)
	}
	if err := catalog.SaveFxClose(db, august, decimal.NewFromInt(1200), domain.Blue); err != nil {
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
	db := testutil.DB(t)
	august := domain.NewPeriod(fixture.Year, time.August)

	if err := catalog.SaveFxClose(db, august, decimal.NewFromInt(1200), domain.Blue); err != nil {
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
	db := testutil.DB(t)
	for _, m := range []time.Month{time.March, time.January, time.February} {
		if err := catalog.SaveFxClose(db, domain.NewPeriod(fixture.Year, m), decimal.NewFromInt(1000), domain.Blue); err != nil {
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

func TestSaveFxCloseRecordsTheHouseItFetchedAt(t *testing.T) {
	db := testutil.DB(t)
	august := domain.NewPeriod(fixture.Year, time.August)

	if err := catalog.SaveFxClose(db, august, decimal.NewFromInt(1200), domain.MEP); err != nil {
		t.Fatalf("SaveFxClose() unexpected error: %v", err)
	}

	got, err := catalog.FxRates(db)
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	mep := domain.MEP
	want := []catalog.FxRate{{Period: august, Value: decimal.NewFromInt(1200), Source: catalog.Close, House: &mep}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("FxRates() mismatch (-want +got):\n%s", diff)
	}
}

func TestSetManualFxRateComesFromNoHouse(t *testing.T) {
	db := testutil.DB(t)
	august := domain.NewPeriod(fixture.Year, time.August)

	if err := catalog.SaveFxClose(db, august, decimal.NewFromInt(1200), domain.Blue); err != nil {
		t.Fatalf("SaveFxClose() unexpected error: %v", err)
	}
	if err := catalog.SetManualFxRate(db, august, decimal.NewFromInt(1450)); err != nil {
		t.Fatalf("SetManualFxRate() unexpected error: %v", err)
	}

	got, err := catalog.FxRates(db)
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	if got[0].House != nil {
		t.Errorf("FxRates()[0].House = %v, want nil", *got[0].House)
	}
}

func TestClearFxRateLetsBackfillFillTheHoleAgain(t *testing.T) {
	db := testutil.DB(t)
	august := domain.NewPeriod(fixture.Year, time.August)

	if err := catalog.SetManualFxRate(db, august, decimal.NewFromInt(1450)); err != nil {
		t.Fatalf("SetManualFxRate() unexpected error: %v", err)
	}
	if err := catalog.ClearFxRate(db, august); err != nil {
		t.Fatalf("ClearFxRate() unexpected error: %v", err)
	}

	got, err := catalog.FxRates(db)
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("FxRates() = %v, want empty", got)
	}

	if err := catalog.SaveFxClose(db, august, decimal.NewFromInt(1200), domain.Official); err != nil {
		t.Fatalf("SaveFxClose() unexpected error: %v", err)
	}
	if got, err = catalog.FxRates(db); err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Source != catalog.Close || *got[0].House != domain.Official {
		t.Errorf("FxRates() = %v, want one Official close", got)
	}
}
