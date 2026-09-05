package tui

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

// Use Month's widths for the shared concept fields.
const (
	cadenceWidth = 9
	statusWidth  = 7

	conceptsTableWidth = gutterWidth + nameWidth + colGap + categoryWidth + colGap +
		currencyWidth + colGap + amountWidth + colGap + cadenceWidth + colGap + statusWidth
)

// The twelve-month strip needs 35 columns.
const (
	conceptPaneWidth  = 35
	conceptPaneLabel  = 10
	conceptsGap       = 6
	conceptsCardWidth = conceptsTableWidth + conceptsGap + conceptPaneWidth
)

type lifecycle string

const (
	statusActive  lifecycle = "active"
	statusFuture  lifecycle = "future"
	statusRetired lifecycle = "retired"
)

// An active-until month is inclusive.
func conceptStatus(c catalog.Concept, today domain.Period) lifecycle {
	switch {
	case !c.ActiveUntil.IsZero() && c.ActiveUntil.Before(today):
		return statusRetired
	case c.ActiveFrom.After(today):
		return statusFuture
	default:
		return statusActive
	}
}

func (m Model) cursorConcept() (catalog.Concept, bool) {
	ordered := m.orderedConcepts()
	if m.conceptsList.cursor >= len(ordered) {
		return catalog.Concept{}, false
	}
	return ordered[m.conceptsList.cursor], true
}

type conceptGroup struct {
	label    string
	concepts []catalog.Concept
}

func (m Model) conceptGroups() []conceptGroup {
	var groups []conceptGroup
	for _, kind := range monthGroups {
		if concepts := m.conceptsOfKind(kind); len(concepts) > 0 {
			groups = append(groups, conceptGroup{label: strings.ToUpper(kind.String()), concepts: concepts})
		}
	}
	if m.showRetired {
		if retired := m.retiredConcepts(); len(retired) > 0 {
			groups = append(groups, conceptGroup{label: "RETIRED", concepts: retired})
		}
	}
	return groups
}

func (m Model) retiredCount() int {
	count := 0
	for _, c := range m.concepts {
		if conceptStatus(c, m.today) == statusRetired {
			count++
		}
	}
	return count
}

// Retired concepts are ordered by end month, newest first.
func (m Model) retiredConcepts() []catalog.Concept {
	var out []catalog.Concept
	for _, c := range m.concepts {
		if conceptStatus(c, m.today) == statusRetired {
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[j].ActiveUntil.Before(out[i].ActiveUntil) })
	return out
}

// Stable sorting preserves concept-name order within each category.
func (m Model) conceptsOfKind(kind catalog.ConceptKind) []catalog.Concept {
	var out []catalog.Concept
	for _, c := range m.concepts {
		if c.Kind == kind && conceptStatus(c, m.today) != statusRetired {
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return categoryName(m.categories, out[i].CategoryID) < categoryName(m.categories, out[j].CategoryID)
	})
	return out
}

// Flatten the displayed groups in cursor order.
func (m Model) orderedConcepts() []catalog.Concept {
	var out []catalog.Concept
	for _, g := range m.conceptGroups() {
		out = append(out, g.concepts...)
	}
	return out
}

func (m Model) handleConceptsKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "n":
		return m.openModal(m.conceptForm(m.newConcept()))
	case "c":
		return m.openModal(m.categoryList())
	case "r":
		m.showRetired = !m.showRetired
		return m, nil
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
	if v.categoryID == 0 && len(m.categories) > 0 {
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
				Options(monthOptions()...).Value(&v.months).Validate(atLeastOneMonth),
		).WithHideFunc(func() bool { return v.preset != presetPicked }),
	}

	id := c.ID
	return newForm(m.theme, m.width, m.height, groups, func() tea.Cmd {
		return write(func() error { return v.save(m.db, id) })
	})
}

func (v *conceptValues) save(db *sql.DB, id int64) error {
	activeFrom, _ := domain.ParsePeriod(v.activeFrom)
	var activeUntil domain.Period
	if v.activeUntil != "" {
		activeUntil, _ = domain.ParsePeriod(v.activeUntil)
	}

	c := catalog.Concept{
		ID:          id,
		Name:        v.name,
		CategoryID:  v.categoryID,
		Kind:        v.kind,
		MonthMask:   v.cadence(),
		ActiveFrom:  activeFrom,
		ActiveUntil: activeUntil,
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
					Title("Delete " + c.Name + "?").
					Description("Its ticks and amounts go with it.").
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

const namedMonths = 2

// The table abbreviates cadences; the detail pane shows all twelve months.
func cadenceLabel(mask domain.Cadence) string {
	months := mask.Months()
	if mask == domain.Monthly {
		return "Monthly"
	}
	if len(months) > namedMonths {
		return fmt.Sprintf("%d months", len(months))
	}
	names := make([]string, len(months))
	for i, month := range months {
		names[i] = month.String()[:3]
	}
	return strings.Join(names, " · ")
}

// A cadence says which months of a year a concept fires in. How long it exists
// is the active window's question, and no preset here answers both.
type monthPreset int

const (
	presetMonthly monthPreset = iota
	presetPicked
)

var presetOptions = []huh.Option[monthPreset]{
	huh.NewOption("Every month", presetMonthly),
	huh.NewOption("Pick months", presetPicked),
}

func presetOf(mask domain.Cadence) monthPreset {
	if mask == domain.Monthly {
		return presetMonthly
	}
	return presetPicked
}

func (v *conceptValues) cadence() domain.Cadence {
	if v.preset == presetMonthly {
		return domain.Monthly
	}
	return domain.NewCadence(v.months...)
}

var kindOptions = []huh.Option[catalog.ConceptKind]{
	huh.NewOption("Income", catalog.Income),
	huh.NewOption("Expense", catalog.Expense),
	huh.NewOption("Saving", catalog.Saving),
	huh.NewOption("Chore", catalog.Chore),
}

func categoryOptions(categories []catalog.Category) []huh.Option[int64] {
	options := make([]huh.Option[int64], len(categories))
	for i, c := range categories {
		options[i] = huh.NewOption(c.Name, c.ID)
	}
	return options
}

func atLeastOneMonth(months []time.Month) error {
	if len(months) == 0 {
		return fmt.Errorf("pick at least one month")
	}
	return nil
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
	title := m.theme.Title.Render("CONCEPTS")
	c, ok := m.cursorConcept()
	if !ok {
		return title + "\n\n" + m.centerInBox(m.theme.Muted.Render(m.emptyCatalogLine()), 2)
	}

	table := m.conceptColumnHeader() + "\n" +
		m.conceptsList.View() + m.scrollHint(m.conceptsList, gutterWidth)
	sidebar := m.conceptPane(c) + "\n\n" + m.conceptMeta()
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		table, strings.Repeat(" ", conceptsGap), sidebar)

	card := title + "\n\n" + body
	top := max(0, (m.bodyHeight(0)-lipgloss.Height(card))/2)
	left := max(0, (m.contentWidth()-conceptsCardWidth)/2)
	return lipgloss.NewStyle().MarginLeft(left).Render(strings.Repeat("\n", top) + card)
}

func (m Model) emptyCatalogLine() string {
	if m.retiredCount() > 0 {
		return "everything here is retired — press r to see it"
	}
	return "no concepts yet — press n to add one"
}

func (m Model) conceptMeta() string {
	counts := make(map[lifecycle]int, 3)
	for _, c := range m.concepts {
		counts[conceptStatus(c, m.today)]++
	}

	lines := []string{fmt.Sprintf("%-8s%3d", statusActive, counts[statusActive])}
	for _, status := range []lifecycle{statusFuture, statusRetired} {
		if counts[status] > 0 {
			lines = append(lines, fmt.Sprintf("%-8s%3d", status, counts[status]))
		}
	}
	return m.theme.Muted.Render(strings.Join(lines, "\n"))
}

func (m Model) conceptPane(c catalog.Concept) string {
	base := "—"
	if c.Money != nil {
		base = c.Money.Currency.String() + " " + formatAmount(c.Money.Base)
	}

	lines := []string{
		m.theme.Title.Render(ansi.Truncate(strings.ToUpper(c.Name), conceptPaneWidth, "…")),
		m.theme.Muted.Render(strings.Join([]string{
			c.Kind.String(), categoryName(m.categories, c.CategoryID),
			string(conceptStatus(c, m.today)),
		}, " · ")),
		"",
		m.paneField("Base", base),
		m.paneField("Window", activeWindow(c)),
		"",
		m.monthStripHeader(),
		m.monthStrip(c.MonthMask),
	}
	return lipgloss.NewStyle().Width(conceptPaneWidth).Render(strings.Join(lines, "\n"))
}

func (m Model) paneField(label, value string) string {
	return m.theme.Muted.Width(conceptPaneLabel).Render(label) + m.theme.Bright.Render(value)
}

func (m Model) monthStripHeader() string {
	initials := make([]string, 12)
	for i := range initials {
		initials[i] = time.Month(i + 1).String()[:1]
	}
	return m.theme.Muted.Render(strings.Join(initials, "  "))
}

func (m Model) monthStrip(mask domain.Cadence) string {
	lit := make(map[time.Month]bool, 12)
	for _, month := range mask.Months() {
		lit[month] = true
	}

	cells := make([]string, 12)
	for i := range cells {
		if lit[time.Month(i+1)] {
			cells[i] = m.theme.Bright.Render("●")
			continue
		}
		cells[i] = m.theme.Muted.Render("·")
	}
	return strings.Join(cells, "  ")
}

func (m Model) conceptColumnHeader() string {
	row := strings.Repeat(" ", gutterWidth) +
		leftCol(nameWidth, "CONCEPT") + leftCol(categoryWidth, "CATEGORY") + leftCol(currencyWidth, "CUR") +
		lipgloss.NewStyle().Width(amountWidth).Align(lipgloss.Right).Render("BASE") +
		strings.Repeat(" ", colGap) + leftCol(cadenceWidth, "CADENCE") +
		lipgloss.NewStyle().Width(statusWidth).Render("STATUS")
	return m.theme.Muted.Render(row)
}

func (m Model) conceptRows() ([]string, []int) {
	var groups []group
	index := 0
	for _, cg := range m.conceptGroups() {
		rendered := make([]string, len(cg.concepts))
		for i, c := range cg.concepts {
			rendered[i] = m.renderConceptRow(c, index == m.conceptsList.cursor)
			index++
		}
		groups = append(groups, group{label: m.ruleHeader(cg.label, conceptsTableWidth), rows: rendered})
	}
	return groupedRows(groups)
}

func (m Model) renderConceptRow(c catalog.Concept, selected bool) string {
	cursor := "  "
	if selected {
		cursor = m.theme.Accent.Render("> ")
	}
	// Truncated before Style.Width sees it: Width wraps what overflows onto a
	// second line, which desyncs the scroller's one-line-per-row cursor math.
	name := lipgloss.NewStyle().Width(nameWidth).Render(ansi.Truncate(c.Name, nameWidth, "…"))
	category := categoryStyle(m.categories, c.CategoryID).Width(categoryWidth).
		Render(ansi.Truncate(categoryName(m.categories, c.CategoryID), categoryWidth, "…"))
	row := cursor + name + strings.Repeat(" ", colGap) + category + strings.Repeat(" ", colGap)

	// A Chore has no money at all, so its cells are blank rather than zeroed.
	if c.Money == nil {
		row += strings.Repeat(" ", currencyWidth+colGap+amountWidth)
	} else {
		row += m.theme.Muted.Width(currencyWidth).Render(c.Money.Currency.String()) +
			strings.Repeat(" ", colGap) +
			m.theme.Bright.Width(amountWidth).Align(lipgloss.Right).Render(formatAmount(c.Money.Base))
	}

	cadence := m.theme.Muted.Width(cadenceWidth).Render(cadenceLabel(c.MonthMask))

	status := conceptStatus(c, m.today)
	style := m.theme.Muted
	if status == statusActive {
		style = m.theme.Bright
	}
	return row + strings.Repeat(" ", colGap) + cadence + strings.Repeat(" ", colGap) +
		style.Width(statusWidth).Render(string(status))
}

func activeWindow(c catalog.Concept) string {
	if c.ActiveUntil.IsZero() {
		return c.ActiveFrom.String() + " → open-ended"
	}
	return c.ActiveFrom.String() + " → " + c.ActiveUntil.String()
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
