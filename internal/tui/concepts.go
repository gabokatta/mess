package tui

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

const newCategory int64 = 0

func (m Model) cursorConcept() (catalog.Concept, bool) {
	if m.conceptsList.cursor >= len(m.concepts) {
		return catalog.Concept{}, false
	}
	return m.concepts[m.conceptsList.cursor], true
}

func (m Model) handleConceptsKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.String() == "n" {
		return m.openModal(m.conceptForm(m.newConcept()))
	}

	c, ok := m.cursorConcept()
	if !ok {
		return m, nil
	}
	switch msg.String() {
	case "e":
		return m.openModal(m.conceptForm(c))
	case "d":
		return m.openModal(m.deleteConceptForm(c))
	}
	return m, nil
}

func (m Model) newConcept() catalog.Concept {
	return catalog.Concept{
		Kind:       catalog.Expense,
		MonthMask:  domain.Monthly,
		ActiveFrom: m.today,
		Money:      &catalog.MoneyDetails{},
	}
}

type conceptValues struct {
	name        string
	categoryID  int64
	newName     string
	kind        catalog.ConceptKind
	currency    domain.Currency
	base        string
	preset      monthPreset
	months      []time.Month
	activeFrom  string
	activeUntil string
}

func (m Model) conceptForm(c catalog.Concept) *form {
	v := &conceptValues{
		name:        c.Name,
		categoryID:  c.CategoryID,
		kind:        c.Kind,
		months:      c.MonthMask.Months(),
		activeFrom:  c.ActiveFrom.String(),
		activeUntil: periodOrBlank(c.ActiveUntil),
	}
	if c.Money != nil {
		v.currency = c.Money.Currency
		v.base = c.Money.Base.StringFixed(2)
	}
	v.preset = presetOf(c.MonthMask)
	if v.categoryID == newCategory && len(m.categories) > 0 {
		v.categoryID = m.categories[0].ID
	}

	title := "New concept"
	if c.ID != 0 {
		title = "Edit " + c.Name
	}

	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewInput().Title("Name").Value(&v.name).Validate(huh.ValidateNotEmpty()),
			huh.NewSelect[int64]().Title("Category").Options(categoryOptions(m.categories)...).Value(&v.categoryID),
			huh.NewSelect[catalog.ConceptKind]().Title("Kind").Options(kindOptions...).Value(&v.kind),
		).Title(title),
		huh.NewGroup(
			huh.NewSelect[domain.Currency]().Title("Currency").
				Options(huh.NewOption("ARS", domain.ARS), huh.NewOption("USD", domain.USD)).
				Value(&v.currency),
			huh.NewInput().Title("Base amount").Description("what the edit box opens with").
				Value(&v.base).Validate(validateDecimal),
		).Title("Money").WithHideFunc(func() bool { return v.kind == catalog.Chore }),
		huh.NewGroup(
			huh.NewSelect[monthPreset]().Title("Months").
				Options(presetOptions...).Value(&v.preset),
			huh.NewInput().Title("Active from").Value(&v.activeFrom).Validate(validatePeriod),
			huh.NewInput().Title("Active until").Description("blank = open-ended").
				Value(&v.activeUntil).Validate(validateOptionalPeriod),
		),
		huh.NewGroup(
			huh.NewMultiSelect[time.Month]().Title("Pick months").
				Options(monthOptions()...).Value(&v.months),
		).WithHideFunc(func() bool { return v.preset != presetPicked }),
		huh.NewGroup(
			huh.NewInput().Title("New category name").Value(&v.newName).Validate(huh.ValidateNotEmpty()),
		).WithHideFunc(func() bool { return v.categoryID != newCategory }),
	}

	id := c.ID
	return newForm(m.theme, m.width, m.height, groups, func() tea.Cmd {
		return write(func() error { return v.save(m.db, id) })
	})
}

func (v *conceptValues) save(db *sql.DB, id int64) error {
	categoryID := v.categoryID
	if categoryID == newCategory {
		cat, err := catalog.FindOrCreateCategory(db, v.newName)
		if err != nil {
			return err
		}
		categoryID = cat.ID
	}

	activeFrom, _ := domain.ParsePeriod(v.activeFrom)
	var activeUntil domain.Period
	if v.activeUntil != "" {
		activeUntil, _ = domain.ParsePeriod(v.activeUntil)
	}

	c := catalog.Concept{
		ID:          id,
		Name:        v.name,
		CategoryID:  categoryID,
		Kind:        v.kind,
		MonthMask:   v.cadence(),
		ActiveFrom:  activeFrom,
		ActiveUntil: activeUntil,
	}
	if v.preset == presetOnce {
		c.ActiveUntil = activeFrom
	}
	if v.kind != catalog.Chore {
		base, _ := decimal.NewFromString(v.base)
		c.Money = &catalog.MoneyDetails{Currency: v.currency, Base: base}
	}

	if id == 0 {
		_, err := catalog.CreateConcept(db, c)
		return err
	}
	return catalog.UpdateConcept(db, c)
}

func (m Model) deleteConceptForm(c catalog.Concept) *form {
	var confirmed bool
	f := newForm(m.theme, m.width, m.height,
		[]*huh.Group{
			huh.NewGroup(
				huh.NewConfirm().
					Title("Delete " + c.Name + "? Its ticks and amounts go with it.").
					Affirmative("Delete").Negative("Keep").Value(&confirmed),
			),
		},
		func() tea.Cmd {
			if !confirmed {
				return nil
			}
			return write(func() error { return catalog.DeleteConcept(m.db, c.ID) })
		})
	f.help = "←/→ choose · enter confirm · esc cancel"
	return f
}

type monthPreset int

const (
	presetMonthly monthPreset = iota
	presetAguinaldo
	presetOnce
	presetPicked
)

var presetOptions = []huh.Option[monthPreset]{
	huh.NewOption("Every month", presetMonthly),
	huh.NewOption("June and December", presetAguinaldo),
	huh.NewOption("This month only", presetOnce),
	huh.NewOption("Pick months", presetPicked),
}

func presetOf(mask domain.Cadence) monthPreset {
	switch mask {
	case domain.Monthly:
		return presetMonthly
	case domain.Aguinaldo:
		return presetAguinaldo
	default:
		return presetPicked
	}
}

func (v *conceptValues) cadence() domain.Cadence {
	switch v.preset {
	case presetMonthly:
		return domain.Monthly
	case presetAguinaldo:
		return domain.Aguinaldo
	case presetOnce:
		from, _ := domain.ParsePeriod(v.activeFrom)
		return domain.NewCadence(from.Month())
	default:
		return domain.NewCadence(v.months...)
	}
}

var kindOptions = []huh.Option[catalog.ConceptKind]{
	huh.NewOption("Income", catalog.Income),
	huh.NewOption("Expense", catalog.Expense),
	huh.NewOption("Saving", catalog.Saving),
	huh.NewOption("Chore", catalog.Chore),
}

func categoryOptions(categories []catalog.Category) []huh.Option[int64] {
	options := make([]huh.Option[int64], 0, len(categories)+1)
	for _, c := range categories {
		options = append(options, huh.NewOption(c.Name, c.ID))
	}
	return append(options, huh.NewOption("New category", newCategory))
}

func monthOptions() []huh.Option[time.Month] {
	options := make([]huh.Option[time.Month], 12)
	for i := range options {
		m := time.Month(i + 1)
		options[i] = huh.NewOption(m.String(), m)
	}
	return options
}

func (m Model) renderConcepts() string {
	title := m.theme.Muted.Render("Concepts")
	if len(m.concepts) == 0 {
		return title + "\n\n" + m.centerInBox(m.theme.Muted.Render("no concepts yet — press n to add one"), 2)
	}
	return title + "\n\n" + m.conceptsList.View()
}

func (m Model) conceptRows() ([]string, []int) {
	var groups []group
	var current int64
	for i, c := range m.concepts {
		if c.CategoryID != current || i == 0 {
			current = c.CategoryID
			groups = append(groups, group{label: categoryStyle(m.categories, c.CategoryID).Bold(true).
				Render(strings.ToUpper(categoryName(m.categories, c.CategoryID)))})
		}
		last := &groups[len(groups)-1]
		last.rows = append(last.rows, m.renderConceptRow(c, i == m.conceptsList.cursor))
	}
	return groupedRows(groups)
}

func (m Model) renderConceptRow(c catalog.Concept, selected bool) string {
	cursor := "  "
	if selected {
		cursor = m.theme.Accent.Render("> ")
	}
	name := m.theme.Bright.Width(nameWidth).MaxWidth(nameWidth).Render(c.Name)
	kind := m.theme.Muted.Width(9).Render(c.Kind.String())

	money := strings.Repeat(" ", amountWidth+4)
	if c.Money != nil {
		money = m.theme.Muted.Render(c.Money.Currency.String()) + " " +
			m.theme.Bright.Width(amountWidth).Align(lipgloss.Right).Render(formatAmount(c.Money.Base))
	}

	active := c.ActiveFrom.String()
	if !c.ActiveUntil.IsZero() {
		active += " – " + c.ActiveUntil.String()
	}
	cadence := m.theme.Muted.Width(7).Render(fmt.Sprintf("%d/12", len(c.MonthMask.Months())))

	return cursor + name + kind + money + "  " + cadence + m.theme.Muted.Render(active)
}

func categoryName(categories []catalog.Category, id int64) string {
	for _, c := range categories {
		if c.ID == id {
			return c.Name
		}
	}
	return "?"
}

func periodOrBlank(p domain.Period) string {
	if p.IsZero() {
		return ""
	}
	return p.String()
}

func validateDecimal(s string) error {
	if s == "" {
		return fmt.Errorf("required")
	}
	_, err := decimal.NewFromString(s)
	return err
}

func validatePeriod(s string) error {
	if s == "" {
		return fmt.Errorf("required")
	}
	_, err := domain.ParsePeriod(s)
	return err
}

func validateOptionalPeriod(s string) error {
	if s == "" {
		return nil
	}
	_, err := domain.ParsePeriod(s)
	return err
}
