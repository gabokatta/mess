package tui

import (
	"database/sql"
	"strings"
	"testing"

	"charm.land/huh/v2"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func settingsModel(t *testing.T, db *sql.DB) Model {
	t.Helper()
	m := New(db)
	m.width, m.height = 100, 40
	m.view = viewSettings
	s, err := catalog.LoadSettings(db)
	if err != nil {
		t.Fatalf("catalog.LoadSettings() unexpected error: %v", err)
	}
	updated, _ := m.Update(settingsLoadedMsg{settings: s})
	return updated.(Model)
}

func TestSettingsViewRendersDefaults(t *testing.T) {
	db := openTestStore(t)
	m := settingsModel(t, db)
	content := m.View().Content
	if !strings.Contains(content, "Blue") {
		t.Errorf("content = %q, want the default fx house Blue", content)
	}
}

func TestEKeyOpensSettingsFormPrefilledWithCurrentValues(t *testing.T) {
	db := openTestStore(t)
	if err := catalog.SaveSettings(db, catalog.Settings{
		FxHouse: domain.MEP,
		Opening: catalog.OpeningBalances{Period: domain.NewPeriod(2026, 1)},
	}); err != nil {
		t.Fatalf("SaveSettings() unexpected error: %v", err)
	}
	m := settingsModel(t, db)

	updated, cmd := m.Update(key("e"))
	m = updated.(Model)
	if m.settingsForm == nil {
		t.Fatal("settingsForm = nil, want a form opened")
	}
	if m.settingsForm.values.fxHouse != domain.MEP {
		t.Errorf("values.fxHouse = %v, want prefilled with MEP", m.settingsForm.values.fxHouse)
	}
	if m.settingsForm.values.openingPeriod != "2026-01" {
		t.Errorf("values.openingPeriod = %q, want prefilled with 2026-01", m.settingsForm.values.openingPeriod)
	}
	m = settle(t, m, cmd)

	content := m.View().Content
	if !strings.Contains(content, "Settings") || !strings.Contains(content, "FX house") {
		t.Errorf("content = %q, want the form's title and FX house field", content)
	}
}

// completeSettingsForm mutates the bound values as if the user had, then
// flips the form to StateCompleted directly.
func completeSettingsForm(m Model, mutate func(*settingsFormValues)) Model {
	mutate(m.settingsForm.values)
	m.settingsForm.form.State = huh.StateCompleted
	return m
}

func TestCompletingSettingsFormPersists(t *testing.T) {
	db := openTestStore(t)
	m := settingsModel(t, db)

	updated, _ := m.Update(key("e"))
	m = updated.(Model)
	m = completeSettingsForm(m, func(v *settingsFormValues) {
		v.fxHouse = domain.Official
		v.openingPeriod = "2026-03"
	})

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	if m.settingsForm != nil {
		t.Fatal("a completed form should close")
	}
	m = settle(t, m, cmd)

	got, err := catalog.LoadSettings(db)
	if err != nil {
		t.Fatalf("catalog.LoadSettings() unexpected error: %v", err)
	}
	if got.FxHouse != domain.Official {
		t.Errorf("LoadSettings().FxHouse = %v, want Official", got.FxHouse)
	}
	if !got.Opening.Period.Equal(domain.NewPeriod(2026, 3)) {
		t.Errorf("LoadSettings().Opening.Period = %v, want 2026-03", got.Opening.Period)
	}
}

func TestEscCancelsSettingsFormWithoutWriting(t *testing.T) {
	db := openTestStore(t)
	m := settingsModel(t, db)

	updated, _ := m.Update(key("e"))
	m = updated.(Model)
	m.settingsForm.values.fxHouse = domain.Official

	updated, cmd := m.Update(keyEsc())
	m = updated.(Model)
	if m.settingsForm != nil {
		t.Error("esc should close the form")
	}
	if cmd != nil {
		t.Error("esc should not write anything")
	}

	got, err := catalog.LoadSettings(db)
	if err != nil {
		t.Fatalf("catalog.LoadSettings() unexpected error: %v", err)
	}
	if got.FxHouse != domain.Blue {
		t.Errorf("LoadSettings().FxHouse = %v, want unchanged Blue", got.FxHouse)
	}
}
