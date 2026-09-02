package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/huh/v2"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func TestRKeyOpensFxOverrideFormPrefilledWithCurrentPeriod(t *testing.T) {
	db := openTestStore(t)
	m := settingsModel(t, db)
	m.period = domain.NewPeriod(2026, time.March)

	updated, cmd := m.Update(key("r"))
	m = updated.(Model)
	if m.fxOverrideForm == nil {
		t.Fatal("fxOverrideForm = nil, want a form opened")
	}
	if m.fxOverrideForm.values.period != "2026-03" {
		t.Errorf("values.period = %q, want prefilled with the shown period", m.fxOverrideForm.values.period)
	}
	m = settle(t, m, cmd)

	content := m.View().Content
	if !strings.Contains(content, "Set FX rate") {
		t.Errorf("content = %q, want the form's title", content)
	}
}

// completeFxOverrideForm fills the bound values as if the user had, then
// flips the form to StateCompleted directly — see completeSettingsForm.
func completeFxOverrideForm(m Model, mutate func(*fxOverrideFormValues)) Model {
	mutate(m.fxOverrideForm.values)
	m.fxOverrideForm.form.State = huh.StateCompleted
	return m
}

func TestCompletingFxOverrideFormWritesAManualRate(t *testing.T) {
	db := openTestStore(t)
	m := settingsModel(t, db)

	updated, _ := m.Update(key("r"))
	m = updated.(Model)
	m = completeFxOverrideForm(m, func(v *fxOverrideFormValues) {
		v.period = "2026-01"
		v.value = "1200"
	})

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	if m.fxOverrideForm != nil {
		t.Fatal("a completed form should close")
	}
	m = settle(t, m, cmd)

	rates, err := catalog.FxRates(db)
	if err != nil {
		t.Fatalf("catalog.FxRates() unexpected error: %v", err)
	}
	if len(rates) != 1 || !rates[0].Value.Equal(amountFor(t, "1200")) || rates[0].Source != catalog.Manual {
		t.Fatalf("FxRates() = %+v, want a single manual 1200 row", rates)
	}
}

func TestCompletingFxOverrideFormReplacesAFetchedRate(t *testing.T) {
	db := openTestStore(t)
	period := domain.NewPeriod(2026, time.January)
	if err := catalog.FillFetchedFxRate(db, period, amountFor(t, "1000")); err != nil {
		t.Fatalf("FillFetchedFxRate() unexpected error: %v", err)
	}
	m := settingsModel(t, db)

	updated, _ := m.Update(key("r"))
	m = updated.(Model)
	m = completeFxOverrideForm(m, func(v *fxOverrideFormValues) {
		v.period = "2026-01"
		v.value = "1500"
	})

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	m = settle(t, m, cmd)

	rates, err := catalog.FxRates(db)
	if err != nil {
		t.Fatalf("catalog.FxRates() unexpected error: %v", err)
	}
	if len(rates) != 1 || !rates[0].Value.Equal(amountFor(t, "1500")) {
		t.Fatalf("FxRates() = %+v, want the fetched rate replaced by the manual 1500", rates)
	}
}

func TestEscCancelsFxOverrideFormWithoutWriting(t *testing.T) {
	db := openTestStore(t)
	m := settingsModel(t, db)

	updated, _ := m.Update(key("r"))
	m = updated.(Model)
	m.fxOverrideForm.values.value = "9999"

	updated, cmd := m.Update(keyEsc())
	m = updated.(Model)
	if m.fxOverrideForm != nil {
		t.Error("esc should close the form")
	}
	if cmd != nil {
		t.Error("esc should not write anything")
	}

	rates, err := catalog.FxRates(db)
	if err != nil {
		t.Fatalf("catalog.FxRates() unexpected error: %v", err)
	}
	if len(rates) != 0 {
		t.Errorf("FxRates() = %+v, want none written", rates)
	}
}
