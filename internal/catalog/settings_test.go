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
		INSERT INTO settings (id, fx_house, opening_period)
		VALUES (1, 'Official', '2026-01')`)

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

func TestLoadSettingsDefaultsWhenUnset(t *testing.T) {
	db := openTestStore(t).DB()

	got, err := LoadSettings(db)
	if err != nil {
		t.Fatalf("LoadSettings() unexpected error: %v", err)
	}
	if got.FxHouse != domain.Blue {
		t.Errorf("LoadSettings().FxHouse = %v, want Blue", got.FxHouse)
	}
	if !got.Opening.Period.IsZero() {
		t.Errorf("LoadSettings().Opening.Period = %v, want zero", got.Opening.Period)
	}
}

func TestSaveSettingsThenLoadSettingsRoundTrips(t *testing.T) {
	db := openTestStore(t).DB()
	want := Settings{
		FxHouse: domain.MEP,
		Opening: OpeningBalances{
			Period:        domain.NewPeriod(2026, 1),
			LeftoverPesos: decimal.NewFromInt(15000),
			CashUSD:       decimal.NewFromInt(2000),
			InvestedUSD:   decimal.NewFromInt(8000),
		},
	}

	if err := SaveSettings(db, want); err != nil {
		t.Fatalf("SaveSettings() unexpected error: %v", err)
	}
	got, err := LoadSettings(db)
	if err != nil {
		t.Fatalf("LoadSettings() unexpected error: %v", err)
	}
	if got.FxHouse != want.FxHouse {
		t.Errorf("LoadSettings().FxHouse = %v, want %v", got.FxHouse, want.FxHouse)
	}
	if !got.Opening.Period.Equal(want.Opening.Period) {
		t.Errorf("LoadSettings().Opening.Period = %s, want %s", got.Opening.Period, want.Opening.Period)
	}
	if !got.Opening.LeftoverPesos.Equal(want.Opening.LeftoverPesos) {
		t.Errorf("LoadSettings().Opening.LeftoverPesos = %s, want %s", got.Opening.LeftoverPesos, want.Opening.LeftoverPesos)
	}
	if !got.Opening.CashUSD.Equal(want.Opening.CashUSD) {
		t.Errorf("LoadSettings().Opening.CashUSD = %s, want %s", got.Opening.CashUSD, want.Opening.CashUSD)
	}
	if !got.Opening.InvestedUSD.Equal(want.Opening.InvestedUSD) {
		t.Errorf("LoadSettings().Opening.InvestedUSD = %s, want %s", got.Opening.InvestedUSD, want.Opening.InvestedUSD)
	}
}

func TestSaveSettingsOverwritesPreviousValues(t *testing.T) {
	db := openTestStore(t).DB()
	first := Settings{FxHouse: domain.Blue, Opening: OpeningBalances{Period: domain.NewPeriod(2026, 1)}}
	if err := SaveSettings(db, first); err != nil {
		t.Fatalf("SaveSettings() unexpected error: %v", err)
	}
	second := Settings{FxHouse: domain.Official, Opening: OpeningBalances{Period: domain.NewPeriod(2026, 2)}}
	if err := SaveSettings(db, second); err != nil {
		t.Fatalf("SaveSettings() unexpected error: %v", err)
	}

	got, err := LoadSettings(db)
	if err != nil {
		t.Fatalf("LoadSettings() unexpected error: %v", err)
	}
	if got.FxHouse != domain.Official || !got.Opening.Period.Equal(domain.NewPeriod(2026, 2)) {
		t.Errorf("LoadSettings() = %+v, want the second save to have overwritten the first", got)
	}
}

func TestLoadOpeningBalancesReadsConfiguredValues(t *testing.T) {
	db := openTestStore(t).DB()
	mustExec(t, db, `
		INSERT INTO settings (id, fx_house, opening_period, opening_leftover_pesos, opening_cash_usd, opening_invested_usd)
		VALUES (1, 'Blue', '2026-01', '15000', '2000', '8000')`)

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
