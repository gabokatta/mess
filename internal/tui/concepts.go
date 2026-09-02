package tui

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

// conceptsLoadedMsg is the result of loadConcepts' Cmd, delivered back to
// Update once the database read completes.
type conceptsLoadedMsg struct {
	concepts    []catalog.Concept
	categories  []catalog.Category
	baseAmounts map[int64][]catalog.BaseAmount
	err         error
}

// categoriesSeededMsg is the result of ensureDefaultCategories' Cmd. It
// always triggers a concepts reload, so a freshly seeded database shows the
// defaults on first render.
type categoriesSeededMsg struct {
	err error
}

// ensureDefaultCategories seeds catalog.DefaultCategoryNames once, on an
// empty category table. Run from Init rather than loadConcepts, so seeding
// happens before the read that would otherwise race it.
func ensureDefaultCategories(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		return categoriesSeededMsg{err: catalog.EnsureDefaultCategories(db)}
	}
}

func loadConcepts(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		concepts, err := catalog.Concepts(db)
		if err != nil {
			return conceptsLoadedMsg{err: err}
		}
		categories, err := catalog.Categories(db)
		if err != nil {
			return conceptsLoadedMsg{err: err}
		}
		baseAmounts, err := catalog.AllBaseAmounts(db)
		return conceptsLoadedMsg{concepts: concepts, categories: categories, baseAmounts: baseAmounts, err: err}
	}
}

// conceptSavedMsg is the result of a new-concept write, which always
// triggers a reload so the rendered list reflects it.
type conceptSavedMsg struct {
	err error
}

// createConcept finds or creates category by name, then writes the concept
// and its opening base amount as one Cmd.
func createConcept(db *sql.DB, c catalog.Concept, category string, amount decimal.Decimal) tea.Cmd {
	return func() tea.Msg {
		cat, err := catalog.FindOrCreateCategory(db, category)
		if err != nil {
			return conceptSavedMsg{err: err}
		}
		c.CategoryID = cat.ID
		created, err := catalog.CreateConcept(db, c)
		if err != nil {
			return conceptSavedMsg{err: err}
		}
		return conceptSavedMsg{err: catalog.SetBaseAmount(db, created.ID, c.ActiveFrom, amount)}
	}
}

// conceptFormValues are the huh-bound values for the new-concept form. Kind,
// currency and months are typed directly via Select/MultiSelect; everything
// else stays a string, parsed once the form completes.
type conceptFormValues struct {
	name        string
	category    string
	kind        catalog.ConceptKind
	currency    domain.Currency
	amount      string
	share       string
	months      []time.Month
	dueDay      string
	activeFrom  string
	activeUntil string
}

// conceptFormState is the new-concept form in progress. It only ever
// creates — editing an existing concept is a later slice.
type conceptFormState struct {
	form   *huh.Form
	values *conceptFormValues
}

func newConceptForm(theme Theme, width, height int, current domain.Period) *conceptFormState {
	v := &conceptFormValues{
		months:     allMonths(),
		activeFrom: current.String(),
	}

	monthOptions := make([]huh.Option[time.Month], 12)
	for i := range 12 {
		m := time.Month(i + 1)
		monthOptions[i] = huh.NewOption(m.String(), m)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Name").Value(&v.name).Validate(huh.ValidateNotEmpty()),
			huh.NewInput().Title("Category").
				Description("an existing name reuses it, a new one creates it").
				Value(&v.category).Validate(huh.ValidateNotEmpty()),
			huh.NewSelect[catalog.ConceptKind]().Title("Kind").
				Options(
					huh.NewOption("Income", catalog.Income),
					huh.NewOption("Fixed expense", catalog.FixedExpense),
					huh.NewOption("Variable expense", catalog.VariableExpense),
				).Value(&v.kind),
			huh.NewSelect[domain.Currency]().Title("Currency").
				Options(huh.NewOption("ARS", domain.ARS), huh.NewOption("USD", domain.USD)).
				Value(&v.currency),
			huh.NewInput().Title("Base amount").Value(&v.amount).Validate(validateRequiredDecimal),
			huh.NewInput().Title("Share %").Description("blank = 100%").
				Value(&v.share).Validate(validateOptionalWholePercent),
			huh.NewMultiSelect[time.Month]().Title("Months").
				Description("deselect to skip months, e.g. only June + December").
				Options(monthOptions...).Value(&v.months),
			huh.NewInput().Title("Due day").Description("blank = none").
				Value(&v.dueDay).Validate(validateOptionalDueDay),
			huh.NewInput().Title("Active from").Value(&v.activeFrom).Validate(validateRequiredPeriod),
			huh.NewInput().Title("Active until").Description("blank = open-ended").
				Value(&v.activeUntil).Validate(validateOptionalPeriod),
		).Title("New concept"),
	).WithTheme(themeFor(theme)).WithWidth(width - 6).WithHeight(formHeight(height)).WithShowHelp(true)

	return &conceptFormState{form: form, values: v}
}

func allMonths() []time.Month {
	months := make([]time.Month, 12)
	for i := range months {
		months[i] = time.Month(i + 1)
	}
	return months
}

func (m Model) startNewConcept() (Model, tea.Cmd) {
	m.conceptForm = newConceptForm(m.theme, m.width, m.height, m.period)
	return m, m.conceptForm.form.Init()
}

func (m Model) updateConceptForm(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.conceptForm = nil
		return m, nil
	}
	return m.forwardConceptForm(msg)
}

// forwardConceptForm drives the form with any tea.Msg: Huh advances fields
// and groups via its own internal messages, returned as a tea.Cmd rather
// than applied synchronously, so those need the same round trip a key does.
func (m Model) forwardConceptForm(msg tea.Msg) (Model, tea.Cmd) {
	updated, cmd := m.conceptForm.form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.conceptForm.form = f
	}

	switch m.conceptForm.form.State {
	case huh.StateCompleted:
		c, category, amount := m.conceptForm.values.build()
		m.conceptForm = nil
		return m, tea.Batch(cmd, createConcept(m.db, c, category, amount))
	case huh.StateAborted:
		m.conceptForm = nil
		return m, nil
	}
	return m, cmd
}

// build converts the form's validated strings into a Concept. Every parse
// here already passed the matching field's Validate func, so an error would
// mean a bug in that pairing rather than bad user input.
func (v *conceptFormValues) build() (catalog.Concept, string, decimal.Decimal) {
	amount, _ := decimal.NewFromString(v.amount)

	var share domain.Percent
	if v.share != "" {
		wholePercent, _ := strconv.ParseInt(v.share, 10, 64)
		share = domain.NewPercent(wholePercent)
	}

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
		Name: v.name, Kind: v.kind, Currency: v.currency, MonthMask: domain.NewCadence(v.months...),
		Share: share, DueDay: dueDay, ActiveFrom: activeFrom, ActiveUntil: activeUntil,
	}
	return c, v.category, amount
}

func validateRequiredDecimal(s string) error {
	if s == "" {
		return fmt.Errorf("required")
	}
	_, err := decimal.NewFromString(s)
	return err
}

func validateOptionalWholePercent(s string) error {
	if s == "" {
		return nil
	}
	_, err := strconv.ParseInt(s, 10, 64)
	return err
}

func validateOptionalDueDay(s string) error {
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return err
	}
	if n < 1 || n > 31 {
		return fmt.Errorf("must be 1-31")
	}
	return nil
}

func validateRequiredPeriod(s string) error {
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

func (m Model) renderConcepts() string {
	var b strings.Builder
	b.WriteString(m.theme.Muted.Render(m.view.String() + " · " + m.period.String()))

	if m.conceptsErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("failed to load: " + m.conceptsErr.Error()))
		return b.String()
	}

	if m.conceptForm != nil {
		b.WriteString("\n\n")
		b.WriteString(m.conceptForm.form.View())
		return b.String()
	}

	if len(m.concepts) == 0 {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("no concepts yet — press n to add one"))
		return b.String()
	}

	for _, cat := range m.categories {
		group := conceptsForCategory(m.concepts, cat.ID)
		if len(group) == 0 {
			continue
		}
		b.WriteString("\n\n")
		b.WriteString(m.theme.Title.Render(cat.Name))
		for _, c := range group {
			b.WriteString("\n")
			b.WriteString(m.renderConceptRow(c))
		}
	}

	if m.conceptSaveErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("failed to save: " + m.conceptSaveErr.Error()))
	}
	return b.String()
}

func conceptsForCategory(concepts []catalog.Concept, categoryID int64) []catalog.Concept {
	var out []catalog.Concept
	for _, c := range concepts {
		if c.CategoryID == categoryID {
			out = append(out, c)
		}
	}
	return out
}

func (m Model) renderConceptRow(c catalog.Concept) string {
	amount := "—"
	if latest, ok := catalog.LatestBaseAmount(m.baseAmounts[c.ID]); ok {
		amount = latest.Amount.StringFixed(2)
	}
	active := c.ActiveFrom.String()
	if !c.ActiveUntil.IsZero() {
		active += " – " + c.ActiveUntil.String()
	}
	return fmt.Sprintf("  %-20s %-14s %s %12s  %s", c.Name, c.Kind, c.Currency, amount, active)
}
