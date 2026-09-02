package tui

import (
	"database/sql"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
)

// incomeConfirmedMsg is the result of writing the income confirm prompt's
// entries, which always triggers a reload so the totals reflect them.
type incomeConfirmedMsg struct {
	err error
}

func confirmIncome(db *sql.DB, period domain.Period, conceptIDs []int64, amounts []string) tea.Cmd {
	return func() tea.Msg {
		for i, value := range amounts {
			if value == "" {
				continue
			}
			amt, err := decimal.NewFromString(value)
			if err != nil {
				return incomeConfirmedMsg{err: err}
			}
			if err := catalog.SetMonthEntryAmount(db, conceptIDs[i], period, &amt); err != nil {
				return incomeConfirmedMsg{err: err}
			}
		}
		return incomeConfirmedMsg{}
	}
}

// incomeConfirmFormValues are the huh-bound values for the income confirm
// prompt: one amount input per active Income concept, prefilled with its
// projected amount, index-aligned with conceptIDs.
type incomeConfirmFormValues struct {
	conceptIDs []int64
	amounts    []string
}

type incomeConfirmFormState struct {
	form   *huh.Form
	values *incomeConfirmFormValues
}

func newIncomeConfirmForm(theme Theme, width, height int, income []month.Line) *incomeConfirmFormState {
	v := &incomeConfirmFormValues{
		conceptIDs: make([]int64, len(income)),
		amounts:    make([]string, len(income)),
	}
	fields := make([]huh.Field, len(income))
	for i, l := range income {
		v.conceptIDs[i] = l.Concept.ID
		v.amounts[i] = l.Money.Amount.StringFixed(2)
		fields[i] = huh.NewInput().Title(l.Concept.Name).Value(&v.amounts[i]).Validate(validateOptionalDecimal)
	}

	form := huh.NewForm(
		huh.NewGroup(fields...).Title("Confirm this month's income").
			Description("esc to skip — nothing here is required"),
	).WithTheme(themeFor(theme)).WithWidth(width - 6).WithHeight(formHeight(height))

	return &incomeConfirmFormState{form: form, values: v}
}

// maybeIncomeConfirmForm builds the prompt when this period has active
// Income concepts but none confirmed yet, or reports nil when there's
// nothing to ask about.
func (m Model) maybeIncomeConfirmForm() *incomeConfirmFormState {
	income := linesForKind(m.lines, catalog.Income)
	if len(income) == 0 || hasConfirmedIncome(m.lines) {
		return nil
	}
	return newIncomeConfirmForm(m.theme, m.width, m.height, income)
}

func hasConfirmedIncome(lines []month.Line) bool {
	for _, l := range lines {
		if l.Concept.Kind == catalog.Income && l.Money != nil && l.Money.Confirmed {
			return true
		}
	}
	return false
}

func (m Model) updateIncomeConfirmForm(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.incomeConfirmForm = nil
		return m, nil
	}
	return m.forwardIncomeConfirmForm(msg)
}

// forwardIncomeConfirmForm drives the form with any tea.Msg — see
// forwardConceptForm's comment for why.
func (m Model) forwardIncomeConfirmForm(msg tea.Msg) (Model, tea.Cmd) {
	updated, cmd := m.incomeConfirmForm.form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.incomeConfirmForm.form = f
	}

	switch m.incomeConfirmForm.form.State {
	case huh.StateCompleted:
		v := m.incomeConfirmForm.values
		m.incomeConfirmForm = nil
		return m, tea.Batch(cmd, confirmIncome(m.db, m.period, v.conceptIDs, v.amounts))
	case huh.StateAborted:
		m.incomeConfirmForm = nil
		return m, nil
	}
	return m, cmd
}
