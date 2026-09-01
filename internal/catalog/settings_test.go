package catalog

import (
	"testing"

	"github.com/gabokatta/mes/internal/domain"
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
