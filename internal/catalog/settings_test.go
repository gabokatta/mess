package catalog_test

import (
	"testing"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/testutil"
)

func TestLoadSettingsDefaultsWhenUnset(t *testing.T) {
	db := testutil.DB(t)

	got, err := catalog.LoadSettings(db)
	if err != nil {
		t.Fatalf("LoadSettings() unexpected error: %v", err)
	}
	if got.FxHouse != domain.Blue {
		t.Errorf("LoadSettings().FxHouse = %v, want Blue", got.FxHouse)
	}
}

func TestSetFxHouseRoundTrips(t *testing.T) {
	db := testutil.DB(t)

	if err := catalog.SetFxHouse(db, domain.MEP); err != nil {
		t.Fatalf("SetFxHouse() unexpected error: %v", err)
	}
	got, err := catalog.FxHouse(db)
	if err != nil {
		t.Fatalf("FxHouse() unexpected error: %v", err)
	}
	if got != domain.MEP {
		t.Errorf("FxHouse() = %v, want MEP", got)
	}
}
