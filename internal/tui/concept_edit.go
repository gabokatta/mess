package tui

import (
	"database/sql"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

// fullShareFraction is the fraction a 100% share resolves to — comparing
// against it is how the edit form tells "never touched" apart from "set to
// something else that happens to be 100".
var fullShareFraction = decimal.NewFromInt(1)

// updateConcept resolves the category the same way createConcept does, then
// writes the concept's fields and, for a money concept, a base amount
// effective from amountEffective — a new dated row if that date is new, an
// in-place correction if it matches an existing one. A Chore has no base
// amount to write.
func updateConcept(db *sql.DB, c catalog.Concept, categoryID int64, newCategory string, amount decimal.Decimal, amountEffective domain.Period) tea.Cmd {
	return func() tea.Msg {
		if categoryID == newCategorySentinel {
			cat, err := catalog.FindOrCreateCategory(db, newCategory)
			if err != nil {
				return conceptSavedMsg{err: err}
			}
			categoryID = cat.ID
		}
		c.CategoryID = categoryID
		if err := catalog.UpdateConcept(db, c); err != nil {
			return conceptSavedMsg{err: err}
		}
		if c.Money == nil {
			return conceptSavedMsg{}
		}
		return conceptSavedMsg{err: catalog.SetBaseAmount(db, c.ID, amountEffective, amount)}
	}
}

// conceptEditFormValues mirrors conceptFormValues, prefilled from an
// existing concept and its latest base amount rather than starting blank.
type conceptEditFormValues struct {
	conceptID       int64
	sortOrder       int
	name            string
	categoryID      int64
	newCategory     string
	kind            catalog.ConceptKind
	currency        domain.Currency
	amount          string
	amountEffective string
	share           string
	months          []time.Month
	dueDay          string
	activeFrom      string
	activeUntil     string
}

type conceptEditFormState struct {
	form   *huh.Form
	values *conceptEditFormValues
}

func newConceptEditForm(theme Theme, width, height int, c catalog.Concept, categories []catalog.Category, latest catalog.BaseAmount, hasBase bool) *conceptEditFormState {
	v := &conceptEditFormValues{
		conceptID:  c.ID,
		sortOrder:  c.SortOrder,
		name:       c.Name,
		categoryID: c.CategoryID,
		kind:       c.Kind,
		months:     c.MonthMask.Months(),
		activeFrom: c.ActiveFrom.String(),
	}
	if c.Money != nil {
		v.currency = c.Money.Currency
		if !c.Money.Share.Fraction().Equal(fullShareFraction) {
			v.share = c.Money.Share.Fraction().Mul(decimal.NewFromInt(100)).String()
		}
	}
	if hasBase {
		v.amount = latest.Amount.StringFixed(2)
		v.amountEffective = latest.EffectiveFrom.String()
	} else {
		v.amountEffective = c.ActiveFrom.String()
	}
	if c.DueDay != 0 {
		v.dueDay = strconv.Itoa(c.DueDay)
	}
	if !c.ActiveUntil.IsZero() {
		v.activeUntil = c.ActiveUntil.String()
	}

	categoryOptions := buildCategoryOptions(categories)
	monthOptions := buildMonthOptions()

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Name").Value(&v.name).Validate(huh.ValidateNotEmpty()),
			huh.NewSelect[int64]().Title("Category").
				Options(categoryOptions...).Value(&v.categoryID),
			huh.NewSelect[catalog.ConceptKind]().Title("Kind").
				Options(conceptKindOptions...).Value(&v.kind),
		).Title("Edit concept"),
		moneyGroupWithEffectiveDate(&v.currency, &v.amount, &v.amountEffective, &v.share, func() bool { return v.kind == catalog.Chore }),
		huh.NewGroup(
			huh.NewMultiSelect[time.Month]().Title("Months").
				Description("deselect to skip months, e.g. only June + December").
				Options(monthOptions...).Value(&v.months),
			huh.NewInput().Title("Due day").Description("blank = none").
				Value(&v.dueDay).Validate(validateOptionalDueDay),
			huh.NewInput().Title("Active from").Value(&v.activeFrom).Validate(validateRequiredPeriod),
			huh.NewInput().Title("Active until").Description("blank = open-ended").
				Value(&v.activeUntil).Validate(validateOptionalPeriod),
		),
		huh.NewGroup(
			huh.NewInput().Title("New category name").Value(&v.newCategory).Validate(huh.ValidateNotEmpty()),
		).Title("New category").WithHideFunc(func() bool { return newCategoryStepHidden(v.categoryID) }),
	).WithTheme(themeFor(theme)).WithWidth(width - 6).WithHeight(formHeight(height))

	return &conceptEditFormState{form: form, values: v}
}

// moneyGroupWithEffectiveDate is moneyGroup plus the edit form's one extra
// field: the date an amount correction takes effect from.
func moneyGroupWithEffectiveDate(currency *domain.Currency, amount, amountEffective, share *string, hideFunc func() bool) *huh.Group {
	return huh.NewGroup(
		huh.NewSelect[domain.Currency]().Title("Currency").
			Options(huh.NewOption("ARS", domain.ARS), huh.NewOption("USD", domain.USD)).
			Value(currency),
		huh.NewInput().Title("Amount").Value(amount).Validate(validateRequiredDecimal),
		huh.NewInput().Title("Amount effective from").
			Description("same date corrects it in place; a new date adds a raise from there").
			Value(amountEffective).Validate(validateRequiredPeriod),
		huh.NewInput().Title("Share %").Description("blank = 100%").
			Value(share).Validate(validateOptionalWholePercent),
	).Title("Money").WithHideFunc(hideFunc)
}

func (m Model) startConceptEdit() (Model, tea.Cmd) {
	c, ok := m.cursorConcept()
	if !ok {
		return m, nil
	}
	latest, hasBase := catalog.LatestBaseAmount(m.baseAmounts[c.ID])
	m.conceptEditForm = newConceptEditForm(m.theme, m.width, m.height, c, m.categories, latest, hasBase)
	return m, m.conceptEditForm.form.Init()
}

func (m Model) updateConceptEditForm(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.conceptEditForm = nil
		return m, nil
	}
	return m.forwardConceptEditForm(msg)
}

// forwardConceptEditForm drives the form with any tea.Msg — see
// forwardConceptForm's comment for why.
func (m Model) forwardConceptEditForm(msg tea.Msg) (Model, tea.Cmd) {
	updated, cmd := m.conceptEditForm.form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.conceptEditForm.form = f
	}

	switch m.conceptEditForm.form.State {
	case huh.StateCompleted:
		c, categoryID, newCategory, amount, amountEffective := m.conceptEditForm.values.build()
		m.conceptEditForm = nil
		return m, tea.Batch(cmd, updateConcept(m.db, c, categoryID, newCategory, amount, amountEffective))
	case huh.StateAborted:
		m.conceptEditForm = nil
		return m, nil
	}
	return m, cmd
}

// build converts the form's validated strings into a Concept. Every parse
// here already passed the matching field's Validate func, so an error would
// mean a bug in that pairing rather than bad user input. Money stays nil for
// a Chore, whatever's left over in the hidden currency/amount/share fields.
func (v *conceptEditFormValues) build() (catalog.Concept, int64, string, decimal.Decimal, domain.Period) {
	dueDay := 0
	if v.dueDay != "" {
		dueDay, _ = strconv.Atoi(v.dueDay)
	}

	activeFrom, _ := domain.ParsePeriod(v.activeFrom)
	var activeUntil domain.Period
	if v.activeUntil != "" {
		activeUntil, _ = domain.ParsePeriod(v.activeUntil)
	}

	c := catalog.Concept{
		ID: v.conceptID, SortOrder: v.sortOrder, Name: v.name, Kind: v.kind,
		MonthMask: domain.NewCadence(v.months...), DueDay: dueDay,
		ActiveFrom: activeFrom, ActiveUntil: activeUntil,
	}
	if v.kind == catalog.Chore {
		return c, v.categoryID, v.newCategory, decimal.Decimal{}, domain.Period{}
	}

	amount, _ := decimal.NewFromString(v.amount)
	amountEffective, _ := domain.ParsePeriod(v.amountEffective)
	var share domain.Percent
	if v.share != "" {
		wholePercent, _ := strconv.ParseInt(v.share, 10, 64)
		share = domain.NewPercent(wholePercent)
	}
	c.Money = &catalog.MoneyDetails{Currency: v.currency, Share: share}
	return c, v.categoryID, v.newCategory, amount, amountEffective
}
