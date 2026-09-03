package catalog_test

import (
	"testing"
	"time"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
)

func TestLoadSettingsDefaultsWhenUnset(t *testing.T) {
	db := fixture.DB(t)

	got, err := catalog.LoadSettings(db)
	if err != nil {
		t.Fatalf("LoadSettings() unexpected error: %v", err)
	}
	if got.FxHouse != domain.Blue {
		t.Errorf("LoadSettings().FxHouse = %v, want Blue", got.FxHouse)
	}
	if got.LastExport != nil {
		t.Errorf("LoadSettings().LastExport = %v, want nil", got.LastExport)
	}
}

func TestSetFxHouseRoundTrips(t *testing.T) {
	db := fixture.DB(t)

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

// MarkExported creates the settings row on a database that never had one,
// and leaves an already-chosen house alone.
func TestMarkExportedKeepsTheChosenHouse(t *testing.T) {
	db := fixture.DB(t)
	if err := catalog.SetFxHouse(db, domain.Official); err != nil {
		t.Fatalf("SetFxHouse() unexpected error: %v", err)
	}

	at := time.Date(fixture.Year, time.September, 2, 15, 4, 5, 0, time.UTC)
	if err := catalog.MarkExported(db, at); err != nil {
		t.Fatalf("MarkExported() unexpected error: %v", err)
	}

	got, err := catalog.LoadSettings(db)
	if err != nil {
		t.Fatalf("LoadSettings() unexpected error: %v", err)
	}
	if got.FxHouse != domain.Official {
		t.Errorf("LoadSettings().FxHouse = %v, want Official", got.FxHouse)
	}
	if got.LastExport == nil {
		t.Fatalf("LoadSettings().LastExport = nil, want %v", at)
	}
	if !got.LastExport.Equal(at) {
		t.Errorf("LoadSettings().LastExport = %v, want %v", got.LastExport, at)
	}
}
