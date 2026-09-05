package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/fixture"
	"github.com/gabokatta/mess/internal/testutil"
)

func openCategories(t *testing.T, world fixture.World) (Model, *categoryList) {
	t.Helper()
	m := modelFor(t, world, minUsableWidth, 32)
	m.view = viewConcepts
	m = m.sync()

	m, _ = send(t, m, key("c"))
	list, ok := m.topModal().(*categoryList)
	if !ok {
		t.Fatalf("modal = %T, want the category list", m.topModal())
	}
	return m, list
}

func TestCategoryListOpensFromTheCatalogScreen(t *testing.T) {
	m, list := openCategories(t, catalogWorld())

	view := stripANSI(list.View())
	for _, want := range []string{"CATEGORY", "COLOUR", "CONCEPTS", "Earnings", "Home"} {
		if !strings.Contains(view, want) {
			t.Errorf("category list is missing %q:\n%s", want, view)
		}
	}

	m, _ = send(t, m, key("esc"))
	if m.topModal() != nil {
		t.Errorf("esc left %T open", m.topModal())
	}
}

func TestCategoryListCountsConcepts(t *testing.T) {
	_, list := openCategories(t, catalogWorld())

	counts := map[string]int{}
	for _, c := range list.categories {
		counts[c.Name] = list.counts[c.ID]
	}
	if counts["Home"] != 2 {
		t.Errorf("Home holds %d concepts, want 2", counts["Home"])
	}
	if counts["Earnings"] != 1 {
		t.Errorf("Earnings holds %d concepts, want 1", counts["Earnings"])
	}
}

func TestArrowsCycleTheColourAndWriteIt(t *testing.T) {
	m, list := openCategories(t, catalogWorld())

	before := list.categories[list.list.cursor]
	order := make([]string, len(list.categories))
	for i, c := range list.categories {
		order[i] = c.Name
	}

	m, cmd := send(t, m, key("right"))
	m, _ = pump(t, m, cmd)

	stored, err := catalog.Categories(m.db)
	if err != nil {
		t.Fatalf("Categories() unexpected error: %v", err)
	}
	var found catalog.Category
	for _, c := range stored {
		if c.ID == before.ID {
			found = c
		}
	}
	want := (before.ColorIndex + 1) % catalog.PaletteSize
	if found.ColorIndex != want {
		t.Errorf("%s colour = %d, want %d", before.Name, found.ColorIndex, want)
	}
	for i, c := range stored {
		if c.Name != order[i] {
			t.Fatalf("list order changed to %q, want %v — colour is not position", c.Name, order)
		}
	}
}

func TestColourCyclingWrapsAtBothEnds(t *testing.T) {
	tests := []struct {
		from int
		key  string
		want int
	}{
		{0, "left", catalog.PaletteSize - 1},
		{catalog.PaletteSize - 1, "right", 0},
	}
	for _, tt := range tests {
		m, list := openCategories(t, catalogWorld())
		target := list.categories[list.list.cursor]

		if err := catalog.SetCategoryColor(m.db, target.ID, tt.from); err != nil {
			t.Fatalf("SetCategoryColor() unexpected error: %v", err)
		}
		m, _ = send(t, m, runCmd(t, loadCatalog(m.db)))
		m, cmd := send(t, m, key(tt.key))
		m, _ = pump(t, m, cmd)

		stored, err := catalog.Categories(m.db)
		if err != nil {
			t.Fatalf("Categories() unexpected error: %v", err)
		}
		for _, c := range stored {
			if c.ID == target.ID && c.ColorIndex != tt.want {
				t.Errorf("%s from %d, %s gave %d, want %d", c.Name, tt.from, tt.key, c.ColorIndex, tt.want)
			}
		}
	}
}

func TestTwoCategoriesMayShareAColour(t *testing.T) {
	world := fixture.World{}
	for _, name := range []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"} {
		world.Concepts = append(world.Concepts, fixture.Concept{
			Name: "Concept " + name, Category: name, Kind: catalog.Expense, Base: "1",
		})
	}
	m := modelFor(t, world, minUsableWidth, 32)

	if len(m.categories) < 9 {
		t.Fatalf("world has %d categories, want at least nine so one wraps", len(m.categories))
	}
	first, ninth := m.categories[0], m.categories[8]
	if categoryColor(m.categories, first.ID) != categoryColor(m.categories, ninth.ID) {
		t.Errorf("the ninth category should wrap onto the first's slot; got %d and %d",
			first.ColorIndex, ninth.ColorIndex)
	}
}

func TestCategoryRowShowsTheColourIndex(t *testing.T) {
	_, list := openCategories(t, catalogWorld())

	c := list.categories[0]
	row := stripANSI(list.row(c, false))
	if !strings.Contains(row, "●") {
		t.Errorf("row has no swatch:\n%q", row)
	}
	if !strings.Contains(row, string(rune('0'+c.ColorIndex+1))) {
		t.Errorf("row = %q, want the colour index %d in text", row, c.ColorIndex+1)
	}
}

func TestRenameFormOpensOverTheListAndReturnsToIt(t *testing.T) {
	m, list := openCategories(t, catalogWorld())
	before := list.categories[list.list.cursor]

	m, cmd := send(t, m, key("r"))
	m, _ = pump(t, m, cmd)

	renaming, ok := m.topModal().(*form)
	if !ok {
		t.Fatalf("top modal = %T, want the rename form", m.topModal())
	}
	if !strings.Contains(renaming.View(), before.Name) {
		t.Errorf("the form does not name the category:\n%s", renaming.View())
	}
	if len(m.modals) != 2 {
		t.Fatalf("stack is %d deep, want the list with the form over it", len(m.modals))
	}

	m, cmd = send(t, m, key("esc"))
	m, _ = pump(t, m, cmd)
	if _, ok := m.topModal().(*categoryList); !ok {
		t.Errorf("esc left %T on top, want the category list back", m.topModal())
	}
}

func TestRenameKeepsConceptsAttached(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)
	var home catalog.Category
	for _, c := range m.categories {
		if c.Name == "Home" {
			home = c
		}
	}

	if err := catalog.RenameCategory(m.db, home.ID, "Household"); err != nil {
		t.Fatalf("RenameCategory() unexpected error: %v", err)
	}

	concepts, err := catalog.Concepts(m.db)
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	var attached int
	for _, c := range concepts {
		if c.CategoryID == home.ID {
			attached++
		}
	}
	if attached != 2 {
		t.Errorf("%d concepts still point at the renamed category, want 2", attached)
	}
}

func TestRenameToATakenNameIsRefusedWithASentence(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)
	var home catalog.Category
	for _, c := range m.categories {
		if c.Name == "Home" {
			home = c
		}
	}

	err := catalog.RenameCategory(m.db, home.ID, "Earnings")
	if err == nil {
		t.Fatal("renaming onto a taken name should be refused")
	}
	if !strings.Contains(err.Error(), "Earnings") || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want it to name the clash in plain words", err)
	}

	stored, _ := catalog.Categories(m.db)
	for _, c := range stored {
		if c.ID == home.ID && c.Name != "Home" {
			t.Errorf("the refused rename went through: %q", c.Name)
		}
	}
}

func TestRenameSucceedsForAFreeName(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)
	first := m.categories[0]

	for _, name := range []string{first.Name, "Utilties", "Utilities"} {
		if err := catalog.RenameCategory(m.db, first.ID, name); err != nil {
			t.Errorf("RenameCategory(%q) unexpected error: %v", name, err)
		}
	}
}

func TestCategoryListRefreshesAfterAWrite(t *testing.T) {
	m, list := openCategories(t, catalogWorld())
	target := list.categories[list.list.cursor]

	m, cmd := send(t, m, runCmd(t, write(func() error {
		return catalog.RenameCategory(m.db, target.ID, "Renamed")
	})))
	m, _ = pump(t, m, cmd)

	refreshed, ok := m.topModal().(*categoryList)
	if !ok {
		t.Fatalf("top modal = %T, want the list", m.topModal())
	}
	var found bool
	for _, c := range refreshed.categories {
		if c.ID == target.ID && c.Name == "Renamed" {
			found = true
		}
	}
	if !found {
		t.Errorf("the list still shows the old name:\n%s", stripANSI(refreshed.View()))
	}
}

func TestCreatingACategoryThroughTheForm(t *testing.T) {
	m, list := openCategories(t, catalogWorld())
	before := len(list.categories)

	m, cmd := send(t, m, key("n"))
	m, _ = pump(t, m, cmd)
	if _, ok := m.topModal().(*form); !ok {
		t.Fatalf("top modal = %T, want the create form", m.topModal())
	}

	m, cmd = typeInto(t, m, "Pets")
	m, _ = pump(t, m, cmd)
	m, cmd = send(t, m, key("enter"))
	m, _ = pump(t, m, cmd)

	stored, err := catalog.Categories(m.db)
	if err != nil {
		t.Fatalf("Categories() unexpected error: %v", err)
	}
	if len(stored) != before+1 {
		t.Fatalf("catalog has %d categories, want %d", len(stored), before+1)
	}

	created := stored[len(stored)-1]
	if created.Name != "Pets" {
		t.Errorf("created %q, want Pets", created.Name)
	}
	if created.SortOrder != before {
		t.Errorf("new category sorts at %d, want appended at %d", created.SortOrder, before)
	}
	for _, c := range stored[:len(stored)-1] {
		if c.ColorIndex == created.ColorIndex {
			t.Errorf("Pets took colour %d, which %s already holds", created.ColorIndex, c.Name)
		}
	}

	if _, ok := m.topModal().(*categoryList); !ok {
		t.Errorf("submitting left %T on top, want the list back", m.topModal())
	}
}

// typeInto sends one key press per rune, which is how a person fills a huh
// input and the only way a test reaches its submit path.
func typeInto(t *testing.T, m Model, text string) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, r := range text {
		m, cmd = send(t, m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return m, cmd
}

func TestRenamingThroughTheForm(t *testing.T) {
	m, list := openCategories(t, catalogWorld())
	target := list.categories[list.list.cursor]

	m, cmd := send(t, m, key("r"))
	m, _ = pump(t, m, cmd)
	for range target.Name {
		m, cmd = send(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	}
	m, cmd = typeInto(t, m, "Wages")
	m, _ = pump(t, m, cmd)
	m, cmd = send(t, m, key("enter"))
	m, _ = pump(t, m, cmd)

	stored, _ := catalog.Categories(m.db)
	for _, c := range stored {
		if c.ID == target.ID && c.Name != "Wages" {
			t.Errorf("category is named %q, want Wages", c.Name)
		}
	}
}

func TestNextColorIndexFillsGapsThenSpreads(t *testing.T) {
	tests := []struct {
		name string
		used []int
		want int
	}{
		{"empty catalog", nil, 0},
		{"a gap in the middle", []int{0, 1, 3}, 2},
		{"appending in order", []int{0, 1, 2}, 3},
		{"every slot taken once", []int{0, 1, 2, 3, 4, 5, 6, 7}, 0},
		{"every slot taken, first doubled", []int{0, 0, 1, 2, 3, 4, 5, 6, 7}, 1},
	}
	for _, tt := range tests {
		categories := make([]catalog.Category, len(tt.used))
		for i, index := range tt.used {
			categories[i] = catalog.Category{ColorIndex: index}
		}
		if got := catalog.NextColorIndex(categories); got != tt.want {
			t.Errorf("%s: NextColorIndex() = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestCreatingADuplicateNameIsRefusedWithASentence(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)

	_, err := catalog.AppendCategory(m.db, "Home")
	if err == nil {
		t.Fatal("creating a category that already exists should be refused")
	}
	if !strings.Contains(err.Error(), "Home") || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want it to name the clash in plain words", err)
	}
}

func TestConceptFormPickerHasNoCreateOption(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)

	options := categoryOptions(m.categories)
	if len(options) != len(m.categories) {
		t.Fatalf("picker has %d options for %d categories", len(options), len(m.categories))
	}
	for _, o := range options {
		if o.Value == 0 {
			t.Errorf("picker still carries a create sentinel: %+v", o)
		}
	}

	m.view = viewConcepts
	m = m.sync()
	m, cmd := send(t, m, key("e"))
	m, _ = pump(t, m, cmd)
	if strings.Contains(m.topModal().View(), "New category") {
		t.Errorf("the concept form still offers to create a category:\n%s", m.topModal().View())
	}
}

func TestDeleteAsksBeforeItActs(t *testing.T) {
	m, list := openCategories(t, catalogWorld())
	target := list.categories[list.list.cursor]

	m, cmd := send(t, m, key("d"))
	m, _ = pump(t, m, cmd)
	if _, ok := m.topModal().(*form); !ok {
		t.Fatalf("top modal = %T, want the delete confirm", m.topModal())
	}
	if !strings.Contains(stripANSI(m.topModal().View()), target.Name) {
		t.Errorf("the confirm does not name the category:\n%s", stripANSI(m.topModal().View()))
	}

	// The confirm defaults to Keep, so answering it straight away writes
	// nothing and returns to the list.
	m, cmd = send(t, m, key("enter"))
	m, writes := pump(t, m, cmd)
	if len(writes) != 0 {
		t.Errorf("writes = %+v, want none", writes)
	}
	if _, ok := m.topModal().(*categoryList); !ok {
		t.Errorf("answering the confirm left %T on top, want the list", m.topModal())
	}
}

func TestDeletingACategoryHoldingConceptsIsRefused(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)
	var home catalog.Category
	for _, c := range m.categories {
		if c.Name == "Home" {
			home = c
		}
	}

	err := catalog.DeleteCategory(m.db, home.ID)
	if err == nil {
		t.Fatal("deleting a category that holds concepts should be refused")
	}
	if !strings.Contains(err.Error(), "Home holds 2 concepts") {
		t.Errorf("error = %q, want it to name the category and the count", err)
	}

	concepts, _ := catalog.Concepts(m.db)
	if len(concepts) != len(catalogWorld().Concepts) {
		t.Errorf("a refused delete took %d concepts with it", len(catalogWorld().Concepts)-len(concepts))
	}
}

func TestDeletingAnEmptyCategorySucceeds(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)
	spare, err := catalog.AppendCategory(m.db, "Pets")
	if err != nil {
		t.Fatalf("AppendCategory() unexpected error: %v", err)
	}

	if err := catalog.DeleteCategory(m.db, spare.ID); err != nil {
		t.Fatalf("DeleteCategory() unexpected error: %v", err)
	}
	stored, _ := catalog.Categories(m.db)
	for _, c := range stored {
		if c.ID == spare.ID {
			t.Error("the category is still there")
		}
	}
}

func TestTheLastCategoryCannotBeDeleted(t *testing.T) {
	db := testutil.DB(t)
	only, err := catalog.AppendCategory(db, "Home")
	if err != nil {
		t.Fatalf("AppendCategory() unexpected error: %v", err)
	}

	err = catalog.DeleteCategory(db, only.ID)
	if err == nil {
		t.Fatal("deleting the last category should be refused")
	}
	if !strings.Contains(err.Error(), "last category") {
		t.Errorf("error = %q, want it to say this is the last one", err)
	}

	stored, _ := catalog.Categories(db)
	if len(stored) != 1 {
		t.Errorf("catalog has %d categories, want the last one kept", len(stored))
	}
}

func TestARestoreMayPassThroughAnEmptyCatalog(t *testing.T) {
	db := testutil.DB(t)
	if _, err := catalog.AppendCategory(db, "Home"); err != nil {
		t.Fatalf("AppendCategory() unexpected error: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin() unexpected error: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM concept`); err != nil {
		t.Fatalf("wipe concepts: %v", err)
	}
	if _, err := tx.Exec(`DELETE FROM category`); err != nil {
		t.Fatalf("a bulk wipe should be allowed to empty the table: %v", err)
	}
	if _, err := tx.Exec(`INSERT INTO category (name, sort_order, color_index) VALUES ('Home', 0, 0)`); err != nil {
		t.Fatalf("refill: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() unexpected error: %v", err)
	}
}

func TestCategoryBoxDoesNotWrapItsRows(t *testing.T) {
	_, list := openCategories(t, catalogWorld())

	lines := strings.Split(stripANSI(list.View()), "\n")
	const chrome = 2 + 2 + 1 // border, padding, column header
	if got, want := len(lines), len(list.categories)+chrome; got != want {
		t.Fatalf("box is %d lines for %d categories, want %d — a row wrapped:\n%s",
			got, len(list.categories), want, stripANSI(list.View()))
	}

	width := lipgloss.Width(lines[0])
	for i, line := range lines {
		if got := lipgloss.Width(line); got != width {
			t.Errorf("line %d is %d columns, want %d:\n%q", i, got, width, line)
		}
	}
	if want := categoriesWidth + 6; width != want {
		t.Errorf("box is %d columns, want %d (%d of rows, plus padding and border)",
			width, want, categoriesWidth)
	}
}

func TestCategoryBoxIsCentred(t *testing.T) {
	m, _ := openCategories(t, catalogWorld())

	var boxed []string
	for _, line := range strings.Split(stripANSI(m.renderBody()), "\n") {
		if strings.ContainsAny(line, "│╭╰") {
			boxed = append(boxed, line)
		}
	}
	if len(boxed) == 0 {
		t.Fatalf("no box in the body:\n%s", stripANSI(m.renderBody()))
	}

	for _, line := range boxed {
		left := len(line) - len(strings.TrimLeft(line, " "))
		right := len(line) - len(strings.TrimRight(line, " "))
		if diff := left - right; diff > 1 || diff < -1 {
			t.Errorf("box sits %d columns from the left and %d from the right:\n%q", left, right, line)
		}
	}
}
