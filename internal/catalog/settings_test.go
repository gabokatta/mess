package catalog

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
)

func TestFxHouseDefaultsToBlueWhenUnset(t *testing.T) {
	db := openTestStore(t).DB()

	got, err := FxHouse(db)
	if err != nil {
		t.Fatalf("FxHouse() unexpected error: %v", err)
	}
	if got != domain.Blue {
		t.Errorf("FxHouse() = %v, want Blue (no settings row yet)", got)
	}
}

func TestFxHouseReadsConfiguredValue(t *testing.T) {
	db := openTestStore(t).DB()
	mustExec(t, db, `
		INSERT INTO settings (id, allowance_cap, allowance_rate, fx_house, opening_period)
		VALUES (1, '310000', '0.7', 'Official', '2026-01')`)

	got, err := FxHouse(db)
	if err != nil {
		t.Fatalf("FxHouse() unexpected error: %v", err)
	}
	if got != domain.Official {
		t.Errorf("FxHouse() = %v, want Official", got)
	}
}

func TestLoadOpeningBalancesZeroWhenUnset(t *testing.T) {
	db := openTestStore(t).DB()

	got, err := LoadOpeningBalances(db)
	if err != nil {
		t.Fatalf("LoadOpeningBalances() unexpected error: %v", err)
	}
	if !got.Period.IsZero() || !got.LeftoverPesos.IsZero() || !got.CashUSD.IsZero() || !got.InvestedUSD.IsZero() {
		t.Errorf("LoadOpeningBalances() = %+v, want the zero value (no settings row yet)", got)
	}
}

func TestLoadOpeningBalancesReadsConfiguredValues(t *testing.T) {
	db := openTestStore(t).DB()
	mustExec(t, db, `
		INSERT INTO settings (id, allowance_cap, allowance_rate, fx_house, opening_period, opening_leftover_pesos, opening_cash_usd, opening_invested_usd)
		VALUES (1, '310000', '0.7', 'Blue', '2026-01', '15000', '2000', '8000')`)

	got, err := LoadOpeningBalances(db)
	if err != nil {
		t.Fatalf("LoadOpeningBalances() unexpected error: %v", err)
	}
	want := domain.NewPeriod(2026, 1)
	if !got.Period.Equal(want) {
		t.Errorf("LoadOpeningBalances().Period = %s, want %s", got.Period, want)
	}
	if !got.LeftoverPesos.Equal(decimal.NewFromInt(15000)) {
		t.Errorf("LoadOpeningBalances().LeftoverPesos = %s, want 15000", got.LeftoverPesos)
	}
	if !got.CashUSD.Equal(decimal.NewFromInt(2000)) {
		t.Errorf("LoadOpeningBalances().CashUSD = %s, want 2000", got.CashUSD)
	}
	if !got.InvestedUSD.Equal(decimal.NewFromInt(8000)) {
		t.Errorf("LoadOpeningBalances().InvestedUSD = %s, want 8000", got.InvestedUSD)
	}
}
