package tui

import (
	"database/sql"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

// fxOverrideMsg is the result of a manual FX rate write. Nothing in the
// Settings view caches rates, so there's nothing to reload — the Month
// view's own rate load already runs fresh on every tab switch.
type fxOverrideMsg struct {
	err error
}

func setFxRateOverride(db *sql.DB, period domain.Period, value decimal.Decimal) tea.Cmd {
	return func() tea.Msg {
		return fxOverrideMsg{err: catalog.SetFxRate(db, period, value)}
	}
}

type fxOverrideFormValues struct {
	period string
	value  string
}

type fxOverrideFormState struct {
	form   *huh.Form
	values *fxOverrideFormValues
}

func newFxOverrideForm(theme Theme, width, height int, current domain.Period) *fxOverrideFormState {
	v := &fxOverrideFormValues{period: current.String()}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Period").Value(&v.period).Validate(validateRequiredPeriod),
			huh.NewInput().Title("Rate").Description("ARS per USD; replaces whatever is stored, fetched or manual").
				Value(&v.value).Validate(validateRequiredDecimal),
		).Title("Set FX rate"),
	).WithTheme(themeFor(theme)).WithWidth(width - 6).WithHeight(formHeight(height))

	return &fxOverrideFormState{form: form, values: v}
}

func (m Model) startFxOverride() (Model, tea.Cmd) {
	m.fxOverrideForm = newFxOverrideForm(m.theme, m.width, m.height, m.period)
	return m, m.fxOverrideForm.form.Init()
}

func (m Model) updateFxOverrideForm(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.fxOverrideForm = nil
		return m, nil
	}
	return m.forwardFxOverrideForm(msg)
}

// forwardFxOverrideForm drives the form with any tea.Msg — see
// forwardConceptForm's comment for why.
func (m Model) forwardFxOverrideForm(msg tea.Msg) (Model, tea.Cmd) {
	updated, cmd := m.fxOverrideForm.form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.fxOverrideForm.form = f
	}

	switch m.fxOverrideForm.form.State {
	case huh.StateCompleted:
		period, value := m.fxOverrideForm.values.build()
		m.fxOverrideForm = nil
		return m, tea.Batch(cmd, setFxRateOverride(m.db, period, value))
	case huh.StateAborted:
		m.fxOverrideForm = nil
		return m, nil
	}
	return m, cmd
}

// build converts the form's validated strings into a period and value.
// Every parse here already passed the matching field's Validate func, so an
// error would mean a bug in that pairing rather than bad user input.
func (v *fxOverrideFormValues) build() (domain.Period, decimal.Decimal) {
	period, _ := domain.ParsePeriod(v.period)
	value, _ := decimal.NewFromString(v.value)
	return period, value
}
