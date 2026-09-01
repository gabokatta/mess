package catalog

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
)

func TestFillFetchedFxRateInsertsWhenAbsent(t *testing.T) {
	db := openTestStore(t).DB()
	sept := domain.NewPeriod(2026, time.September)
	value := decimal.NewFromInt(1200)

	if err := FillFetchedFxRate(db, sept, value); err != nil {
		t.Fatalf("FillFetchedFxRate() unexpected error: %v", err)
	}

	got, err := FxRates(db)
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	if len(got) != 1 || !got[0].Value.Equal(value) || got[0].Source != Fetched {
		t.Fatalf("FxRates() = %+v, want a single Fetched 1200 row", got)
	}
}

func TestFillFetchedFxRateNeverOverwritesExistingRow(t *testing.T) {
	db := openTestStore(t).DB()
	sept := domain.NewPeriod(2026, time.September)
	manual := decimal.NewFromInt(1300)

	if err := SetFxRate(db, sept, manual); err != nil {
		t.Fatalf("SetFxRate() unexpected error: %v", err)
	}
	if err := FillFetchedFxRate(db, sept, decimal.NewFromInt(1200)); err != nil {
		t.Fatalf("FillFetchedFxRate() unexpected error: %v", err)
	}

	got, err := FxRates(db)
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	if len(got) != 1 || !got[0].Value.Equal(manual) || got[0].Source != Manual {
		t.Fatalf("FxRates() = %+v, want the manual 1300 row untouched", got)
	}
}

func TestSetFxRateOverwritesAnyExistingRow(t *testing.T) {
	db := openTestStore(t).DB()
	sept := domain.NewPeriod(2026, time.September)

	if err := FillFetchedFxRate(db, sept, decimal.NewFromInt(1200)); err != nil {
		t.Fatalf("FillFetchedFxRate() unexpected error: %v", err)
	}
	manual := decimal.NewFromInt(1300)
	if err := SetFxRate(db, sept, manual); err != nil {
		t.Fatalf("SetFxRate() unexpected error: %v", err)
	}

	got, err := FxRates(db)
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	if len(got) != 1 || !got[0].Value.Equal(manual) || got[0].Source != Manual {
		t.Fatalf("FxRates() = %+v, want the fetched row replaced by the 1300 manual one", got)
	}
}

func TestFxRatesOrdersByPeriod(t *testing.T) {
	db := openTestStore(t).DB()
	oct := domain.NewPeriod(2026, time.October)
	sept := domain.NewPeriod(2026, time.September)

	if err := SetFxRate(db, oct, decimal.NewFromInt(1250)); err != nil {
		t.Fatalf("SetFxRate() unexpected error: %v", err)
	}
	if err := SetFxRate(db, sept, decimal.NewFromInt(1200)); err != nil {
		t.Fatalf("SetFxRate() unexpected error: %v", err)
	}

	got, err := FxRates(db)
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	if len(got) != 2 || !got[0].Period.Equal(sept) || !got[1].Period.Equal(oct) {
		t.Fatalf("FxRates() = %+v, want September before October", got)
	}
}
