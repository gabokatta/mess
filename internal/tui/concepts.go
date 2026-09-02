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

// newCategorySentinel is the Category select's "+ New category" option —
// the zero value, so a form built with no categories yet defaults there.
const newCategorySentinel int64 = 0

// createConcept resolves the category — an existing ID reuses it directly,
// the sentinel finds-or-creates newCategory by name — then writes the
// concept and its opening base amount as one Cmd.
func createConcept(db *sql.DB, c catalog.Concept, categoryID int64, newCategory string, amount decimal.Decimal) tea.Cmd {
	return func() tea.Msg {
		if categoryID == newCategorySentinel {
			cat, err := catalog.FindOrCreateCategory(db, newCategory)
			if err != nil {
				return conceptSavedMsg{err: err}
			}
			categoryID = cat.ID
		}
		c.CategoryID = categoryID
		created, err := catalog.CreateConcept(db, c)
		if err != nil {
			return conceptSavedMsg{err: err}
		}
		return conceptSavedMsg{err: catalog.SetBaseAmount(db, created.ID, c.ActiveFrom, amount)}
	}
}

// conceptFormValues are the huh-bound values for the new-concept form. Kind,
// currency, category and months are typed directly via Select/MultiSelect;
// everything else stays a string, parsed once the form completes.
type conceptFormValues struct {
	name        string
	categoryID  int64
	newCategory string
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

func newConceptForm(theme Theme, width, height int, current domain.Period, categories []catalog.Category) *conceptFormState {
	v := &conceptFormValues{
		months:     allMonths(),
		activeFrom: current.String(),
	}
	if len(categories) > 0 {
		v.categoryID = categories[0].ID
	}

	categoryOptions := buildCategoryOptions(categories)
	monthOptions := buildMonthOptions()

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Name").Value(&v.name).Validate(huh.ValidateNotEmpty()),
			huh.NewSelect[int64]().Title("Category").
				Options(categoryOptions...).Value(&v.categoryID),
			huh.NewSelect[catalog.ConceptKind]().Title("Kind").
				Options(
					huh.NewOption("Income", catalog.Income),
					huh.NewOption("Expense", catalog.Expense),
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
		huh.NewGroup(
			huh.NewInput().Title("New category name").Value(&v.newCategory).Validate(huh.ValidateNotEmpty()),
		).Title("New category").WithHideFunc(func() bool { return newCategoryStepHidden(v.categoryID) }),
	).WithTheme(themeFor(theme)).WithWidth(width - 6).WithHeight(formHeight(height)).WithShowHelp(true)

	return &conceptFormState{form: form, values: v}
}

// newCategoryStepHidden reports whether the "New category name" step should
// be skipped — true whenever Category picked an existing row rather than the
// sentinel, so the prompt only ever appears when it's actually needed.
func newCategoryStepHidden(categoryID int64) bool {
	return categoryID != newCategorySentinel
}

// buildCategoryOptions is the Category select's option list, shared by the
// new-concept and edit-concept forms: every existing category, plus the
// New category sentinel.
func buildCategoryOptions(categories []catalog.Category) []huh.Option[int64] {
	options := make([]huh.Option[int64], 0, len(categories)+1)
	for _, cat := range categories {
		options = append(options, huh.NewOption(cat.Name, cat.ID))
	}
	return append(options, huh.NewOption("New category", newCategorySentinel))
}

// buildMonthOptions is the Months multi-select's option list, shared by
// every form that picks a Cadence: concept, chore, and concept-edit.
func buildMonthOptions() []huh.Option[time.Month] {
	options := make([]huh.Option[time.Month], 12)
	for i := range 12 {
		m := time.Month(i + 1)
		options[i] = huh.NewOption(m.String(), m)
	}
	return options
}

func allMonths() []time.Month {
	months := make([]time.Month, 12)
	for i := range months {
		months[i] = time.Month(i + 1)
	}
	return months
}

func (m Model) startNewConcept() (Model, tea.Cmd) {
	m.conceptForm = newConceptForm(m.theme, m.width, m.height, m.period, m.categories)
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
		c, categoryID, newCategory, amount := m.conceptForm.values.build()
		m.conceptForm = nil
		return m, tea.Batch(cmd, createConcept(m.db, c, categoryID, newCategory, amount))
	case huh.StateAborted:
		m.conceptForm = nil
		return m, nil
	}
	return m, cmd
}

// build converts the form's validated strings into a Concept. Every parse
// here already passed the matching field's Validate func, so an error would
// mean a bug in that pairing rather than bad user input.
func (v *conceptFormValues) build() (catalog.Concept, int64, string, decimal.Decimal) {
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
	return c, v.categoryID, v.newCategory, amount
}

func validateRequiredDecimal(s string) error {
	if s == "" {
		return fmt.Errorf("required")
	}
	_, err := decimal.NewFromString(s)
	return err
}

func validateOptionalDecimal(s string) error {
	if s == "" {
		return nil
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

// orderedConcepts is m.concepts in the same grouped-by-category order the
// Concepts view renders, so cursor index and render index always agree.
func (m Model) orderedConcepts() []catalog.Concept {
	var out []catalog.Concept
	for _, cat := range m.categories {
		out = append(out, conceptsForCategory(m.concepts, cat.ID)...)
	}
	return out
}

func (m Model) moveConceptCursor(delta int) int {
	n := len(m.orderedConcepts())
	if n == 0 {
		return 0
	}
	cursor := m.conceptCursor + delta
	if cursor < 0 {
		return 0
	}
	if cursor >= n {
		return n - 1
	}
	return cursor
}

// cursorConcept reports the concept under the cursor, if the list isn't empty.
func (m Model) cursorConcept() (catalog.Concept, bool) {
	list := m.orderedConcepts()
	if m.conceptCursor >= len(list) {
		return catalog.Concept{}, false
	}
	return list[m.conceptCursor], true
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
	if m.conceptEditForm != nil {
		b.WriteString("\n\n")
		b.WriteString(m.conceptEditForm.form.View())
		return b.String()
	}

	if len(m.concepts) == 0 {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("no concepts yet — press n to add one"))
		return b.String()
	}

	idx := 0
	for _, cat := range m.categories {
		group := conceptsForCategory(m.concepts, cat.ID)
		if len(group) == 0 {
			continue
		}
		b.WriteString("\n\n")
		b.WriteString(m.theme.Title.Render(cat.Name))
		for _, c := range group {
			b.WriteString("\n")
			b.WriteString(m.renderConceptRow(c, idx == m.conceptCursor))
			idx++
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

func (m Model) renderConceptRow(c catalog.Concept, selected bool) string {
	cursor := " "
	if selected {
		cursor = ">"
	}
	amount := "—"
	if latest, ok := catalog.LatestBaseAmount(m.baseAmounts[c.ID]); ok {
		amount = latest.Amount.StringFixed(2)
	}
	active := c.ActiveFrom.String()
	if !c.ActiveUntil.IsZero() {
		active += " – " + c.ActiveUntil.String()
	}
	return fmt.Sprintf("%s %-20s %-14s %s %12s  %s", cursor, c.Name, c.Kind, c.Currency, amount, active)
}
