package tui

import (
	"database/sql"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

// settingsLoadedMsg is the result of loadSettings' Cmd, delivered back to
// Update once the database read completes.
type settingsLoadedMsg struct {
	settings catalog.Settings
	err      error
}

func loadSettings(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		s, err := catalog.LoadSettings(db)
		return settingsLoadedMsg{settings: s, err: err}
	}
}

// settingsSavedMsg is the result of a settings write, which always triggers
// a reload so the rendered fields reflect it.
type settingsSavedMsg struct {
	err error
}

func saveSettings(db *sql.DB, s catalog.Settings) tea.Cmd {
	return func() tea.Msg {
		return settingsSavedMsg{err: catalog.SaveSettings(db, s)}
	}
}

// settingsFormValues are the huh-bound values for the settings form.
// FxHouse is typed directly via Select; the rest stay strings, parsed once
// the form completes.
type settingsFormValues struct {
	fxHouse         domain.FxHouse
	openingPeriod   string
	openingLeftover string
	openingCash     string
	openingInvested string
}

type settingsFormState struct {
	form   *huh.Form
	values *settingsFormValues
}

func newSettingsForm(theme Theme, width, height int, current catalog.Settings) *settingsFormState {
	v := &settingsFormValues{
		fxHouse:         current.FxHouse,
		openingPeriod:   periodOrBlank(current.Opening.Period),
		openingLeftover: current.Opening.LeftoverPesos.StringFixed(2),
		openingCash:     current.Opening.CashUSD.StringFixed(2),
		openingInvested: current.Opening.InvestedUSD.StringFixed(2),
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[domain.FxHouse]().Title("FX house").
				Options(
					huh.NewOption("Blue", domain.Blue),
					huh.NewOption("Official", domain.Official),
					huh.NewOption("MEP", domain.MEP),
				).Value(&v.fxHouse),
			huh.NewInput().Title("Opening period").Description("blank = unset (YYYY-MM)").
				Value(&v.openingPeriod).Validate(validateOptionalPeriod),
			huh.NewInput().Title("Opening leftover pesos (ARS)").
				Value(&v.openingLeftover).Validate(validateRequiredDecimal),
			huh.NewInput().Title("Opening cash (USD)").
				Value(&v.openingCash).Validate(validateRequiredDecimal),
			huh.NewInput().Title("Opening invested (USD)").
				Value(&v.openingInvested).Validate(validateRequiredDecimal),
		).Title("Settings"),
	).WithTheme(themeFor(theme)).WithWidth(width - 6).WithHeight(formHeight(height)).WithShowHelp(true)

	return &settingsFormState{form: form, values: v}
}

func periodOrBlank(p domain.Period) string {
	if p.IsZero() {
		return ""
	}
	return p.String()
}

func (m Model) startSettingsEdit() (Model, tea.Cmd) {
	m.settingsForm = newSettingsForm(m.theme, m.width, m.height, m.settings)
	return m, m.settingsForm.form.Init()
}

func (m Model) updateSettingsForm(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.settingsForm = nil
		return m, nil
	}
	return m.forwardSettingsForm(msg)
}

// forwardSettingsForm drives the form with any tea.Msg, not just key
// presses — see forwardConceptForm's comment for why.
func (m Model) forwardSettingsForm(msg tea.Msg) (Model, tea.Cmd) {
	updated, cmd := m.settingsForm.form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.settingsForm.form = f
	}

	switch m.settingsForm.form.State {
	case huh.StateCompleted:
		settings := m.settingsForm.values.build()
		m.settingsForm = nil
		return m, tea.Batch(cmd, saveSettings(m.db, settings))
	case huh.StateAborted:
		m.settingsForm = nil
		return m, nil
	}
	return m, cmd
}

// build converts the form's validated strings into Settings. Every parse
// here already passed the matching field's Validate func, so an error would
// mean a bug in that pairing rather than bad user input.
func (v *settingsFormValues) build() catalog.Settings {
	var period domain.Period
	if v.openingPeriod != "" {
		period, _ = domain.ParsePeriod(v.openingPeriod)
	}
	leftover, _ := decimal.NewFromString(v.openingLeftover)
	cash, _ := decimal.NewFromString(v.openingCash)
	invested, _ := decimal.NewFromString(v.openingInvested)

	return catalog.Settings{
		FxHouse: v.fxHouse,
		Opening: catalog.OpeningBalances{
			Period: period, LeftoverPesos: leftover, CashUSD: cash, InvestedUSD: invested,
		},
	}
}

func (m Model) renderSettings() string {
	var b strings.Builder
	b.WriteString(m.theme.Muted.Render(m.view.String()))

	if m.settingsErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("failed to load: " + m.settingsErr.Error()))
		return b.String()
	}

	if m.settingsForm != nil {
		b.WriteString("\n\n")
		b.WriteString(m.settingsForm.form.View())
		return b.String()
	}
	if m.exportForm != nil {
		b.WriteString("\n\n")
		b.WriteString(m.exportForm.form.View())
		return b.String()
	}
	if m.importForm != nil {
		b.WriteString("\n\n")
		b.WriteString(m.importForm.form.View())
		return b.String()
	}
	if m.fxOverrideForm != nil {
		b.WriteString("\n\n")
		b.WriteString(m.fxOverrideForm.form.View())
		return b.String()
	}

	fmt.Fprintf(&b, "\n\n  fx house                       %s", m.settings.FxHouse)
	fmt.Fprintf(&b, "\n  opening period                 %s", periodDisplay(m.settings.Opening.Period))
	fmt.Fprintf(&b, "\n  opening leftover pesos (ARS)   %s", m.settings.Opening.LeftoverPesos.StringFixed(2))
	fmt.Fprintf(&b, "\n  opening cash (USD)             %s", m.settings.Opening.CashUSD.StringFixed(2))
	fmt.Fprintf(&b, "\n  opening invested (USD)         %s", m.settings.Opening.InvestedUSD.StringFixed(2))

	if m.settingsSaveErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("failed to save: " + m.settingsSaveErr.Error()))
	}
	if m.fxOverrideErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("failed to set fx rate: " + m.fxOverrideErr.Error()))
	}
	b.WriteString(m.renderBackupStatus())
	return b.String()
}

func periodDisplay(p domain.Period) string {
	if p.IsZero() {
		return "—"
	}
	return p.String()
}
