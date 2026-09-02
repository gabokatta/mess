package tui

import (
	"database/sql"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

// choreSavedMsg is the result of a new-chore write, which always triggers a
// reload so the rendered chore list reflects it.
type choreSavedMsg struct {
	err error
}

func createChore(db *sql.DB, c catalog.Chore) tea.Cmd {
	return func() tea.Msg {
		_, err := catalog.CreateChore(db, c)
		return choreSavedMsg{err: err}
	}
}

// choreFormValues are the huh-bound values for the new-chore form. Months
// are typed directly via MultiSelect; everything else stays a string,
// parsed once the form completes — see conceptFormValues for the same shape.
type choreFormValues struct {
	name        string
	months      []time.Month
	dueDay      string
	activeFrom  string
	activeUntil string
}

type choreFormState struct {
	form   *huh.Form
	values *choreFormValues
}

func newChoreForm(theme Theme, width, height int, current domain.Period) *choreFormState {
	v := &choreFormValues{
		months:     allMonths(),
		activeFrom: current.String(),
	}

	monthOptions := buildMonthOptions()

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Name").Value(&v.name).Validate(huh.ValidateNotEmpty()),
			huh.NewMultiSelect[time.Month]().Title("Months").
				Description("deselect to skip months, e.g. only June + December").
				Options(monthOptions...).Value(&v.months),
			huh.NewInput().Title("Due day").Description("blank = none").
				Value(&v.dueDay).Validate(validateOptionalDueDay),
			huh.NewInput().Title("Active from").Value(&v.activeFrom).Validate(validateRequiredPeriod),
			huh.NewInput().Title("Active until").Description("blank = open-ended").
				Value(&v.activeUntil).Validate(validateOptionalPeriod),
		).Title("New chore"),
	).WithTheme(themeFor(theme)).WithWidth(width - 6).WithHeight(formHeight(height))

	return &choreFormState{form: form, values: v}
}

func (m Model) startNewChore() (Model, tea.Cmd) {
	m.choreForm = newChoreForm(m.theme, m.width, m.height, m.period)
	return m, m.choreForm.form.Init()
}

func (m Model) updateChoreForm(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.choreForm = nil
		return m, nil
	}
	return m.forwardChoreForm(msg)
}

// forwardChoreForm drives the form with any tea.Msg — see
// forwardConceptForm's comment for why.
func (m Model) forwardChoreForm(msg tea.Msg) (Model, tea.Cmd) {
	updated, cmd := m.choreForm.form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.choreForm.form = f
	}

	switch m.choreForm.form.State {
	case huh.StateCompleted:
		c := m.choreForm.values.build()
		m.choreForm = nil
		return m, tea.Batch(cmd, createChore(m.db, c))
	case huh.StateAborted:
		m.choreForm = nil
		return m, nil
	}
	return m, cmd
}

// build converts the form's validated strings into a Chore. Every parse
// here already passed the matching field's Validate func, so an error would
// mean a bug in that pairing rather than bad user input.
func (v *choreFormValues) build() catalog.Chore {
	dueDay := 0
	if v.dueDay != "" {
		dueDay, _ = strconv.Atoi(v.dueDay)
	}

	activeFrom, _ := domain.ParsePeriod(v.activeFrom)
	var activeUntil domain.Period
	if v.activeUntil != "" {
		activeUntil, _ = domain.ParsePeriod(v.activeUntil)
	}

	return catalog.Chore{
		Name: v.name, MonthMask: domain.NewCadence(v.months...),
		DueDay: dueDay, ActiveFrom: activeFrom, ActiveUntil: activeUntil,
	}
}
