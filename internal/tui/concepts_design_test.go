package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
)

// A world holding one concept of every kind, with two categories inside the
// expense block so the sort inside a block is observable.
func catalogWorld() fixture.World {
	return fixture.World{
		Concepts: []fixture.Concept{
			{Name: "Salary", Category: "Earnings", Kind: catalog.Income, Base: "4704000"},
			{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "1428000"},
			{Name: "Groceries", Category: "Food", Kind: catalog.Expense, Base: "369600"},
			{Name: "Dollars", Category: "Savings", Kind: catalog.Saving, Base: "300"},
			{Name: "Take out the trash", Category: "Home", Kind: catalog.Chore},
		},
	}
}

func TestConceptGroupsAreKindsInMonthsOrder(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)

	groups := m.conceptGroups()
	want := []string{"INCOME", "EXPENSE", "SAVING", "CHORE"}
	if len(groups) != len(want) {
		t.Fatalf("conceptGroups() = %d groups, want %v", len(groups), want)
	}
	for i, label := range want {
		if groups[i].label != label {
			t.Errorf("group %d = %q, want %q", i, groups[i].label, label)
		}
	}
}

func TestConceptGroupsSkipKindsWithNoConcepts(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "1"}},
	}, minUsableWidth, 32)

	groups := m.conceptGroups()
	if len(groups) != 1 || groups[0].label != "EXPENSE" {
		t.Fatalf("conceptGroups() = %+v, want only EXPENSE", groups)
	}
}

func TestConceptRowsSortByCategoryInsideAKind(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)

	var expense []catalog.Concept
	for _, g := range m.conceptGroups() {
		if g.label == "EXPENSE" {
			expense = g.concepts
		}
	}
	if len(expense) != 2 {
		t.Fatalf("EXPENSE = %+v, want two concepts", expense)
	}
	if expense[0].Name != "Groceries" || expense[1].Name != "Rent" {
		t.Errorf("EXPENSE = %q, %q; want Food before Home", expense[0].Name, expense[1].Name)
	}
}

func TestConceptCursorWalksTheGroupedOrder(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)
	m.view = viewConcepts
	m = m.sync()

	for i, want := range []string{"Salary", "Groceries", "Rent", "Dollars", "Take out the trash"} {
		if got, ok := m.cursorConcept(); !ok || got.Name != want {
			t.Fatalf("cursor at %d = %q, want %q", i, got.Name, want)
		}
		m, _ = send(t, m, key("down"))
	}
}

func TestConceptColumnHeaderNamesTheColumns(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)

	header := stripANSI(m.conceptColumnHeader())
	for _, want := range []string{"CONCEPT", "CATEGORY", "CUR", "BASE", "CADENCE"} {
		if !strings.Contains(header, want) {
			t.Errorf("column header is missing %q:\n%q", want, header)
		}
	}
	if strings.Contains(header, "AMOUNT") {
		t.Errorf("column header says AMOUNT, which is Month's column:\n%q", header)
	}
}

func TestConceptKindHeadersCarryNoPaletteHue(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)
	m.view = viewConcepts
	m = m.sync()

	rows, _ := m.conceptRows()
	want := m.ruleHeader("INCOME", conceptsTableWidth)
	if rows[0] != want {
		t.Errorf("first row = %q, want the structural INCOME header %q", rows[0], want)
	}
	for _, c := range m.categories {
		hue := categoryStyle(m.categories, c.ID).Render("INCOME")
		if strings.Contains(rows[0], hue) {
			t.Fatalf("the INCOME header took %s's hue:\n%q", c.Name, rows[0])
		}
	}
}

func TestLongConceptNameTruncatesOnTheCatalogScreen(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{
			{Name: "Refacción Completa del Techo de la Casa", Category: "Home", Kind: catalog.Expense, Base: "159600"},
		},
	}, minUsableWidth, 32)

	c, ok := m.cursorConcept()
	if !ok {
		t.Fatal("cursorConcept() found nothing")
	}
	styled := m.renderConceptRow(c, false)
	rendered := stripANSI(styled)
	if strings.Contains(rendered, "\n") {
		t.Fatalf("renderConceptRow produced more than one line:\n%q", rendered)
	}
	if !strings.Contains(rendered, "…") {
		t.Errorf("name cell = %q, want it to end in an ellipsis", rendered)
	}
	if !strings.Contains(rendered, "159.600") {
		t.Errorf("the base fell out of its column:\n%q", rendered)
	}
	if got := lineWidth(styled); got != conceptsTableWidth {
		t.Errorf("row width = %d, want %d", got, conceptsTableWidth)
	}
}

func TestChoreLeavesCurrencyAndBaseBlank(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{{Name: "Take out the trash", Category: "Home", Kind: catalog.Chore}},
	}, minUsableWidth, 32)

	c, _ := m.cursorConcept()
	rendered := stripANSI(m.renderConceptRow(c, false))
	if strings.Contains(rendered, "ARS") || strings.Contains(rendered, "0,00") {
		t.Errorf("a chore row shows money:\n%q", rendered)
	}
	if got := lineWidth(m.renderConceptRow(c, false)); got != conceptsTableWidth {
		t.Errorf("row width = %d, want %d — blank cells still hold their columns", got, conceptsTableWidth)
	}
}

func TestDemoWorldCarriesANameLongerThanTheColumn(t *testing.T) {
	longest := 0
	for _, c := range fixture.Demo(fixture.Period).Concepts {
		if n := len([]rune(c.Name)); n > longest {
			longest = n
		}
	}
	if longest <= nameWidth {
		t.Errorf("longest demo concept name is %d runes, want more than %d so truncation is seeded", longest, nameWidth)
	}
}

func TestCadenceLabelNamesFewMonthsAndCountsMany(t *testing.T) {
	tests := []struct {
		mask domain.Cadence
		want string
	}{
		{domain.Monthly, "Monthly"},
		{domain.NewCadence(time.March), "Mar"},
		{domain.NewCadence(time.June, time.December), "Jun · Dec"},
		{domain.NewCadence(time.February, time.June, time.October), "3 months"},
		{domain.NewCadence(time.March, time.June, time.September, time.December), "4 months"},
	}
	for _, tt := range tests {
		if got := cadenceLabel(tt.mask); got != tt.want {
			t.Errorf("cadenceLabel(%012b) = %q, want %q", tt.mask, got, tt.want)
		}
		if got := lipgloss.Width(cadenceLabel(tt.mask)); got > cadenceWidth {
			t.Errorf("cadenceLabel(%012b) is %d columns, want at most %d", tt.mask, got, cadenceWidth)
		}
	}
}

func TestAStoredSparseCadenceOpensThePicker(t *testing.T) {
	mask := domain.NewCadence(time.June, time.December)
	if got := presetOf(mask); got != presetPicked {
		t.Errorf("presetOf(June|December) = %d, want presetPicked", got)
	}

	v := &conceptValues{preset: presetPicked, months: mask.Months()}
	if got := v.cadence(); got != mask {
		t.Errorf("cadence() = %012b, want %012b — the picker round-trips the mask", got, mask)
	}
}

func TestDemoWorldCarriesBothSidesOfTheCadenceCutoff(t *testing.T) {
	var named, counted bool
	for _, c := range fixture.Demo(fixture.Period).Concepts {
		switch n := len(c.Months.Months()); {
		case c.Months != 0 && c.Months != domain.Monthly && n <= namedMonths:
			named = true
		case n > namedMonths && c.Months != domain.Monthly:
			counted = true
		}
	}
	if !named {
		t.Error("no demo concept fires in one or two months, so no row spells its cadence")
	}
	if !counted {
		t.Error("no demo concept fires in three or more months, so no row counts its cadence")
	}
}

// A world with one concept of each lifecycle state, so the status column and
// the retired block are both observable.
func lifecycleWorld() fixture.World {
	return fixture.World{
		Concepts: []fixture.Concept{
			{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "1", From: september.AddMonths(-6)},
			{Name: "New Lease", Category: "Home", Kind: catalog.Expense, Base: "1", From: september.AddMonths(4)},
			{Name: "Old Gym", Category: "Health", Kind: catalog.Expense, Base: "1",
				From: september.AddMonths(-12), Until: september.AddMonths(-8)},
			{Name: "Personal Loan", Category: "Debt", Kind: catalog.Expense, Base: "1",
				From: september.AddMonths(-12), Until: september.AddMonths(-2)},
		},
	}
}

func TestConceptStatusReadsTheWindowAgainstToday(t *testing.T) {
	tests := []struct {
		name string
		from int
		till int
		want lifecycle
	}{
		{"open-ended and running", -6, 0, statusActive},
		{"ends later this year", -6, 3, statusActive},
		{"window has not opened", 4, 0, statusFuture},
		{"window has closed", -12, -2, statusRetired},
	}
	for _, tt := range tests {
		c := catalog.Concept{ActiveFrom: september.AddMonths(tt.from)}
		if tt.till != 0 {
			c.ActiveUntil = september.AddMonths(tt.till)
		}
		if got := conceptStatus(c, september); got != tt.want {
			t.Errorf("%s: conceptStatus() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestRetiredConceptsCollectInTheirOwnBlock(t *testing.T) {
	m := modelFor(t, lifecycleWorld(), minUsableWidth, 32)
	m.showRetired = true

	groups := m.conceptGroups()
	last := groups[len(groups)-1]
	if last.label != "RETIRED" {
		t.Fatalf("last group = %q, want RETIRED after the kind blocks", last.label)
	}
	if len(last.concepts) != 2 {
		t.Fatalf("RETIRED = %+v, want the two closed windows", last.concepts)
	}
	if last.concepts[0].Name != "Personal Loan" || last.concepts[1].Name != "Old Gym" {
		t.Errorf("RETIRED = %q, %q; want most recently ended first",
			last.concepts[0].Name, last.concepts[1].Name)
	}

	for _, g := range groups[:len(groups)-1] {
		for _, c := range g.concepts {
			if conceptStatus(c, m.today) == statusRetired {
				t.Errorf("%q is retired and still in the %s block", c.Name, g.label)
			}
		}
	}
}

func TestFutureConceptStaysInItsKindBlock(t *testing.T) {
	m := modelFor(t, lifecycleWorld(), minUsableWidth, 32)
	m.view = viewConcepts
	m = m.sync()

	var found bool
	for _, g := range m.conceptGroups() {
		for _, c := range g.concepts {
			if c.Name != "New Lease" {
				continue
			}
			found = true
			if g.label != "EXPENSE" {
				t.Errorf("New Lease is in %q, want EXPENSE", g.label)
			}
		}
	}
	if !found {
		t.Fatal("New Lease is not in any group")
	}
	if !strings.Contains(stripANSI(m.renderConcepts()), string(statusFuture)) {
		t.Errorf("no row reads %q:\n%s", statusFuture, stripANSI(m.renderConcepts()))
	}
}

func TestRetiredBlockIsAbsentWhenNothingIsRetired(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)
	m.showRetired = true
	for _, g := range m.conceptGroups() {
		if g.label == "RETIRED" {
			t.Error("a catalog with nothing retired should carry no RETIRED block")
		}
	}
}

func TestRetiredConceptsAreHiddenUntilAskedFor(t *testing.T) {
	m := modelFor(t, lifecycleWorld(), minUsableWidth, 32)
	m.view = viewConcepts
	m = m.sync()

	content := stripANSI(m.renderConcepts())
	if strings.Contains(content, "RETIRED") || strings.Contains(content, "Old Gym") {
		t.Errorf("retired concepts show before r is pressed:\n%s", content)
	}
	if !strings.Contains(content, "retired   2") {
		t.Errorf("the count should say what is hidden:\n%s", content)
	}
	if !strings.Contains(m.help(), "r retired") {
		t.Errorf("help = %q, want it to name the key", m.help())
	}

	m, _ = send(t, m, key("r"))
	shown := stripANSI(m.renderConcepts())
	for _, want := range []string{"RETIRED", "Old Gym", "Personal Loan"} {
		if !strings.Contains(shown, want) {
			t.Errorf("r did not reveal %q:\n%s", want, shown)
		}
	}

	m, _ = send(t, m, key("r"))
	if strings.Contains(stripANSI(m.renderConcepts()), "RETIRED") {
		t.Error("r should hide the block again")
	}
}

func TestHidingRetiredMovesTheCursorBackIntoTheList(t *testing.T) {
	m := modelFor(t, lifecycleWorld(), minUsableWidth, 32)
	m.view = viewConcepts
	m, _ = send(t, m, key("r"))

	for range m.orderedConcepts() {
		m, _ = send(t, m, key("down"))
	}
	last, _ := m.cursorConcept()
	if conceptStatus(last, m.today) != statusRetired {
		t.Fatalf("cursor is on %q, want a retired concept at the end of the list", last.Name)
	}

	m, _ = send(t, m, key("r"))
	c, ok := m.cursorConcept()
	if !ok {
		t.Fatal("the cursor found nothing after the block closed")
	}
	if conceptStatus(c, m.today) == statusRetired {
		t.Errorf("cursor is still on the retired %q", c.Name)
	}
}

func TestAnAllRetiredCatalogSaysSoRatherThanLookingEmpty(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{{
			Name: "Old Gym", Category: "Health", Kind: catalog.Expense, Base: "1",
			From: september.AddMonths(-12), Until: september.AddMonths(-8),
		}},
	}, minUsableWidth, 32)
	m.view = viewConcepts
	m = m.sync()

	content := stripANSI(m.renderConcepts())
	if strings.Contains(content, "no concepts yet") {
		t.Errorf("a catalog of retired concepts should not read as empty:\n%s", content)
	}
	if !strings.Contains(content, "press r") {
		t.Errorf("the line should say how to see them:\n%s", content)
	}
}

func TestRetiredRowsAreMutedAndActiveRowsAreNot(t *testing.T) {
	m := modelFor(t, lifecycleWorld(), minUsableWidth, 32)

	var active, retired catalog.Concept
	for _, c := range m.concepts {
		switch c.Name {
		case "Rent":
			active = c
		case "Old Gym":
			retired = c
		}
	}
	if !strings.Contains(m.renderConceptRow(active, false), m.theme.Bright.Width(statusWidth).Render(string(statusActive))) {
		t.Error("an active row should carry its status in plain foreground")
	}
	if !strings.Contains(m.renderConceptRow(retired, false), m.theme.Muted.Width(statusWidth).Render(string(statusRetired))) {
		t.Error("a retired row should carry its status muted")
	}
}

func TestDemoWorldCarriesEveryLifecycleState(t *testing.T) {
	world := fixture.Demo(fixture.Period)
	seen := map[lifecycle]int{}
	for _, c := range world.Concepts {
		seen[conceptStatus(catalog.Concept{ActiveFrom: c.From, ActiveUntil: c.Until}, fixture.Period)]++
	}
	if seen[statusRetired] < 2 {
		t.Errorf("demo world has %d retired concepts, want at least two so the block has an order", seen[statusRetired])
	}
	if seen[statusFuture] < 1 {
		t.Error("demo world has no future concept, so the seeded app never shows that word")
	}
}

func TestConceptPaneCarriesTheDefinition(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)
	m.view = viewConcepts
	m = m.sync()

	c, _ := m.cursorConcept()
	pane := stripANSI(m.conceptPane(c))
	for _, want := range []string{"SALARY", "Income · Earnings · active", "Base", "4.704.000", "Window", "open-ended"} {
		if !strings.Contains(pane, want) {
			t.Errorf("pane is missing %q:\n%s", want, pane)
		}
	}
	for _, line := range strings.Split(pane, "\n") {
		if got := lipgloss.Width(line); got > conceptPaneWidth {
			t.Errorf("pane line is %d columns, want at most %d:\n%q", got, conceptPaneWidth, line)
		}
	}
}

func TestConceptPaneDrawsTheMonthStrip(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{{
			Name: "School Fee", Category: "Home", Kind: catalog.Expense, Base: "1",
			Months: domain.NewCadence(time.June, time.December),
		}},
	}, minUsableWidth, 32)

	c, _ := m.cursorConcept()
	lines := strings.Split(stripANSI(m.conceptPane(c)), "\n")
	header, strip := strings.TrimRight(lines[6], " "), strings.TrimRight(lines[7], " ")
	if header != "J  F  M  A  M  J  J  A  S  O  N  D" {
		t.Errorf("strip header = %q", header)
	}
	if strip != "·  ·  ·  ·  ·  ●  ·  ·  ·  ·  ·  ●" {
		t.Errorf("strip = %q, want June and December lit", strip)
	}
}

func TestConceptPaneHeightDoesNotMoveWithTheCursor(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)
	m.view = viewConcepts
	m = m.sync()

	heights := map[int]bool{}
	for range m.orderedConcepts() {
		c, _ := m.cursorConcept()
		heights[lipgloss.Height(m.conceptPane(c))] = true
		if c.Money == nil && !strings.Contains(stripANSI(m.conceptPane(c)), "Base      —") {
			t.Errorf("a chore's base should read as absent:\n%s", stripANSI(m.conceptPane(c)))
		}
		m, _ = send(t, m, key("down"))
	}
	if len(heights) != 1 {
		t.Errorf("pane heights = %v, want one height for every concept", heights)
	}
}

func TestConceptCardIsCentredAndFitsTheFloor(t *testing.T) {
	if conceptsCardWidth > minUsableWidth-6 {
		t.Fatalf("card is %d columns, wider than contentWidth at the %d floor",
			conceptsCardWidth, minUsableWidth)
	}

	for _, width := range []int{minUsableWidth, 160} {
		m := modelFor(t, catalogWorld(), width, 32)
		m.view = viewConcepts
		m = m.sync()

		content := m.renderConcepts()
		for _, line := range strings.Split(content, "\n") {
			if got := lineWidth(line); got > m.contentWidth() {
				t.Fatalf("at %d columns a line is %d wide, want at most %d:\n%q",
					width, got, m.contentWidth(), stripANSI(line))
			}
		}
	}
}

func TestEmptyCatalogRendersNoPane(t *testing.T) {
	m := modelFor(t, fixture.World{}, minUsableWidth, 32)
	m.view = viewConcepts
	m = m.sync()

	content := stripANSI(m.renderConcepts())
	if !strings.Contains(content, "press n to add one") {
		t.Errorf("an empty catalog should say what to press:\n%s", content)
	}
	if strings.Contains(content, "Window") || strings.Contains(content, "CONCEPT ") {
		t.Errorf("an empty catalog should render neither the table nor the pane:\n%s", content)
	}
}

func TestConceptsTitleIsBoldUppercase(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)
	m.view = viewConcepts
	m = m.sync()

	if !strings.Contains(m.renderConcepts(), m.theme.Title.Render("CONCEPTS")) {
		t.Errorf("the title should be bold uppercase:\n%s", stripANSI(m.renderConcepts()))
	}
}

func TestConceptMetaSitsUnderThePane(t *testing.T) {
	m := modelFor(t, lifecycleWorld(), minUsableWidth, 32)
	m.view = viewConcepts
	m = m.sync()

	lines := strings.Split(stripANSI(m.renderConcepts()), "\n")
	var paneColumn, metaColumn int
	for _, line := range lines {
		if i := strings.Index(line, "Window"); i > 0 && paneColumn == 0 {
			paneColumn = i
		}
		if i := strings.Index(line, "active "); i > 0 && strings.Contains(line, "active   ") {
			metaColumn = i
		}
	}
	if paneColumn == 0 || metaColumn == 0 {
		t.Fatalf("could not find the pane and the meta cluster:\n%s", stripANSI(m.renderConcepts()))
	}
	if metaColumn != paneColumn {
		t.Errorf("meta starts at column %d and the pane at %d, want them flush", metaColumn, paneColumn)
	}
}

func TestConceptMetaCountsByTheSameWordTheRowsUse(t *testing.T) {
	m := modelFor(t, lifecycleWorld(), minUsableWidth, 32)

	meta := stripANSI(m.conceptMeta())
	for _, want := range []string{"active    1", "future    1", "retired   2"} {
		if !strings.Contains(meta, want) {
			t.Errorf("meta = %q, want a line reading %q", meta, want)
		}
	}

	clean := modelFor(t, catalogWorld(), minUsableWidth, 32)
	got := stripANSI(clean.conceptMeta())
	for _, unwanted := range []lifecycle{statusFuture, statusRetired} {
		if strings.Contains(got, string(unwanted)) {
			t.Errorf("meta = %q, want no %q line when the count is zero", got, unwanted)
		}
	}
}

func TestEveryLifecycleWordFitsTheStatusColumn(t *testing.T) {
	for _, status := range []lifecycle{statusActive, statusFuture, statusRetired} {
		if got := len(status); got > statusWidth {
			t.Errorf("%q is %d columns, want at most %d", status, got, statusWidth)
		}
	}
}

func TestConceptScrollHintAppearsWhenTheListIsCut(t *testing.T) {
	world := fixture.World{}
	for i := range 40 {
		world.Concepts = append(world.Concepts, fixture.Concept{
			Name: fmt.Sprintf("Concept %02d", i), Category: "Home", Kind: catalog.Expense, Base: "1",
		})
	}
	m := modelFor(t, world, minUsableWidth, minUsableHeight)
	m.view = viewConcepts
	m = m.sync()

	if !strings.Contains(stripANSI(m.renderConcepts()), "more") {
		t.Errorf("a cut list should carry a scroll hint:\n%s", stripANSI(m.renderConcepts()))
	}
}

func TestEnterIsUnboundOnTheCatalogScreen(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)
	m.view = viewConcepts
	m = m.sync()

	m, cmd := send(t, m, key("enter"))
	if m.topModal() != nil {
		t.Errorf("enter opened %T, want it unbound", m.topModal())
	}
	if cmd != nil {
		t.Error("enter produced a command, want it unbound")
	}
}
