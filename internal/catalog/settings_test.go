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

// A World that never declares LastExport leaves the settings row unmarked,
// the same "never exported" state a database that has never called Load at
// all reports.
func TestLoadWithoutLastExportLeavesSettingsUnmarked(t *testing.T) {
	db := fixture.DB(t)
	fixture.MustLoad(t, db, fixture.World{})

	got, err := catalog.LoadSettings(db)
	if err != nil {
		t.Fatalf("LoadSettings() unexpected error: %v", err)
	}
	if got.LastExport != nil {
		t.Errorf("LoadSettings().LastExport = %v, want nil", got.LastExport)
	}
}

// fixture.Load writes LastExport through catalog.MarkExported, so a World
// that declares one is the "Month status line shows an export date" case a
// World otherwise cannot reach.
func TestLoadWritesLastExport(t *testing.T) {
	db := fixture.DB(t)
	at := time.Date(fixture.Year, time.September, 2, 15, 4, 5, 0, time.UTC)
	fixture.MustLoad(t, db, fixture.World{LastExport: &at})

	got, err := catalog.LoadSettings(db)
	if err != nil {
		t.Fatalf("LoadSettings() unexpected error: %v", err)
	}
	if got.LastExport == nil || !got.LastExport.Equal(at) {
		t.Errorf("LoadSettings().LastExport = %v, want %v", got.LastExport, at)
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
