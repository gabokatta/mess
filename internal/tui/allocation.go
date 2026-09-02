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
	"github.com/gabokatta/mess/internal/month"
)

// allocationsLoadedMsg is the result of loadAllocations' Cmd, delivered back
// to Update once the database read completes.
type allocationsLoadedMsg struct {
	allocations []catalog.SavingAllocation
	rates       []catalog.FxRate
	err         error
}

// loadAllocations returns a Cmd that reads period's allocations and the
// full fx rate history — ResolveGap needs history, not just this period's
// own rate, to convert a USD allocation at the rate it was made under.
func loadAllocations(db *sql.DB, period domain.Period) tea.Cmd {
	return func() tea.Msg {
		allocations, err := catalog.SavingAllocations(db, period)
		if err != nil {
			return allocationsLoadedMsg{err: err}
		}
		rates, err := catalog.FxRates(db)
		return allocationsLoadedMsg{allocations: allocations, rates: rates, err: err}
	}
}

// allocationSavedMsg is the result of writing the panel's allocations, which
// always triggers a reload so the rendered gap reflects it.
type allocationSavedMsg struct {
	err error
}

func createAllocations(db *sql.DB, allocations []catalog.SavingAllocation) tea.Cmd {
	return func() tea.Msg {
		for _, a := range allocations {
			if _, err := catalog.CreateSavingAllocation(db, a); err != nil {
				return allocationSavedMsg{err: err}
			}
		}
		return allocationSavedMsg{}
	}
}

func deleteAllocation(db *sql.DB, id int64) tea.Cmd {
	return func() tea.Msg {
		return allocationSavedMsg{err: catalog.DeleteSavingAllocation(db, id)}
	}
}

// deleteCursorAllocation removes the allocation under the cursor, or is a
// no-op if the cursor is on a concept line or a chore instead.
func (m Model) deleteCursorAllocation() (Model, tea.Cmd) {
	a, ok := m.cursorAllocation()
	if !ok {
		return m, nil
	}
	return m, deleteAllocation(m.db, a.ID)
}

// allocationFormValues are the huh-bound values for the allocation panel.
// Each destination takes a percent of available or an exact figure — percent
// wins if both are set, and blank in both skips that destination entirely.
type allocationFormValues struct {
	investedPercent  string
	investedAmount   string
	investedCurrency domain.Currency
	cashPercent      string
	cashAmount       string
	cashCurrency     domain.Currency
}

type allocationFormState struct {
	form   *huh.Form
	values *allocationFormValues
}

func newAllocationForm(theme Theme, width, height int, available, availableUSD decimal.Decimal, hasUSD bool) *allocationFormState {
	v := &allocationFormValues{investedCurrency: domain.USD, cashCurrency: domain.USD}
	desc := available.StringFixed(2) + " ARS"
	if hasUSD {
		desc += fmt.Sprintf("  (%s USD)", availableUSD.StringFixed(2))
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title("Available to save").
				Description(desc),
			huh.NewInput().Title("Invested %").Description("blank = skip; always % of the ARS remainder above").
				Value(&v.investedPercent).Validate(validateOptionalWholePercent),
			huh.NewInput().Title("Invested amount").Description("blank = skip; ignored if % is set").
				Value(&v.investedAmount).Validate(validateOptionalDecimal),
			huh.NewSelect[domain.Currency]().Title("Invested amount currency").
				Description("currency for the exact figure above only — % is always ARS").
				Options(huh.NewOption("ARS", domain.ARS), huh.NewOption("USD", domain.USD)).
				Value(&v.investedCurrency),
			huh.NewInput().Title("Cash %").Description("blank = skip; always % of the ARS remainder above").
				Value(&v.cashPercent).Validate(validateOptionalWholePercent),
			huh.NewInput().Title("Cash amount").Description("blank = skip; ignored if % is set").
				Value(&v.cashAmount).Validate(validateOptionalDecimal),
			huh.NewSelect[domain.Currency]().Title("Cash amount currency").
				Description("currency for the exact figure above only — % is always ARS").
				Options(huh.NewOption("ARS", domain.ARS), huh.NewOption("USD", domain.USD)).
				Value(&v.cashCurrency),
		).Title("Allocate"),
	).WithTheme(themeFor(theme)).WithWidth(width - 6).WithHeight(formHeight(height))

	return &allocationFormState{form: form, values: v}
}

func (m Model) startAllocationPanel() (Model, tea.Cmd) {
	usd, ok := m.availableToSaveUSD()
	m.allocationForm = newAllocationForm(m.theme, m.width, m.height, m.available(), usd, ok)
	return m, m.allocationForm.form.Init()
}

func (m Model) updateAllocationForm(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.allocationForm = nil
		return m, nil
	}
	return m.forwardAllocationForm(msg)
}

// forwardAllocationForm drives the form with any tea.Msg — see
// forwardConceptForm's comment for why.
func (m Model) forwardAllocationForm(msg tea.Msg) (Model, tea.Cmd) {
	updated, cmd := m.allocationForm.form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.allocationForm.form = f
	}

	switch m.allocationForm.form.State {
	case huh.StateCompleted:
		allocations := m.allocationForm.values.build(m.period, m.available())
		m.allocationForm = nil
		return m, tea.Batch(cmd, createAllocations(m.db, allocations))
	case huh.StateAborted:
		m.allocationForm = nil
		return m, nil
	}
	return m, cmd
}

// build resolves each destination independently: percent of available wins
// over an exact figure when both are set, and a destination with neither is
// left out rather than written as a zero allocation.
func (v *allocationFormValues) build(period domain.Period, available decimal.Decimal) []catalog.SavingAllocation {
	var out []catalog.SavingAllocation
	if a, cur, ok := v.resolve(v.investedPercent, v.investedAmount, v.investedCurrency, available); ok {
		out = append(out, catalog.SavingAllocation{Period: period, Destination: catalog.Invested, Amount: a, Currency: cur})
	}
	if a, cur, ok := v.resolve(v.cashPercent, v.cashAmount, v.cashCurrency, available); ok {
		out = append(out, catalog.SavingAllocation{Period: period, Destination: catalog.Cash, Amount: a, Currency: cur})
	}
	return out
}

func (v *allocationFormValues) resolve(percent, exact string, currency domain.Currency, available decimal.Decimal) (decimal.Decimal, domain.Currency, bool) {
	if percent != "" {
		n, _ := decimal.NewFromString(percent)
		return available.Mul(n).Div(decimal.NewFromInt(100)), domain.ARS, true
	}
	if exact != "" {
		amt, _ := decimal.NewFromString(exact)
		return amt, currency, true
	}
	return decimal.Decimal{}, domain.ARS, false
}

// available is the confirmed ARS remainder the allocation panel starts
// from, computed from the month lines already in memory rather than a
// separate query.
func (m Model) available() decimal.Decimal {
	return month.AvailableToSave(month.ResolveTotals(m.lines))
}

// availableToSaveUSD converts available() at period's resolved fx rate,
// purely for display alongside the ARS figure — the allocation itself
// always stores whatever currency was actually typed, so this conversion is
// never persisted. ok is false when no rate is resolvable yet.
func (m Model) availableToSaveUSD() (decimal.Decimal, bool) {
	rate, ok := month.ResolveFxRate(m.period, m.rates)
	if !ok || rate.IsZero() {
		return decimal.Decimal{}, false
	}
	return m.available().Div(rate), true
}

// unallocated reports whether there's something to save and nothing decided
// yet this period — the moment pocket money and available-to-save read as
// the same number, which looks like "nothing to do here" rather than
// "you haven't decided yet." Gone the instant any allocation exists.
func (m Model) unallocated() bool {
	return m.available().IsPositive() && len(m.allocations) == 0
}

// renderAllocations renders the Savings section, marking whichever
// allocation the cursor sits on — startIdx is that row's position in the
// month view's shared cursor space, right after the line list.
func (m Model) renderAllocations(startIdx int) string {
	var b strings.Builder
	b.WriteString(m.theme.Title.Render("Savings"))
	fmt.Fprintf(&b, "\n  available to save  %12s ARS", m.available().StringFixed(2))
	if usd, ok := m.availableToSaveUSD(); ok {
		fmt.Fprintf(&b, "  %s", m.theme.Muted.Render("("+usd.StringFixed(2)+" USD)"))
	}
	if m.unallocated() {
		fmt.Fprintf(&b, "\n  %s", m.theme.Highlight.Render(m.available().StringFixed(2)+" unallocated · press a"))
	}
	for i, a := range m.allocations {
		cursor := " "
		if startIdx+i == m.cursor {
			cursor = ">"
		}
		fmt.Fprintf(&b, "\n%s %-10s          %12s %s", cursor, a.Destination, a.Amount.StringFixed(2), a.Currency)
	}
	if pocket, err := month.ResolveGap(m.available(), m.period, m.allocations, m.rates); err != nil {
		b.WriteString("\n  " + m.theme.Muted.Render("pocket money unavailable: "+err.Error()))
	} else {
		fmt.Fprintf(&b, "\n  pocket money         %12s ARS", pocket.StringFixed(2))
	}
	if m.allocationsErr != nil {
		b.WriteString("\n  " + m.theme.Muted.Render("failed to load: "+m.allocationsErr.Error()))
	}
	if m.allocationSaveErr != nil {
		b.WriteString("\n  " + m.theme.Muted.Render("failed to save: "+m.allocationSaveErr.Error()))
	}
	return b.String()
}
