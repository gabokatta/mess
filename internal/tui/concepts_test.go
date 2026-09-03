package tui

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func conceptsModel(t *testing.T, db *sql.DB) Model {
	t.Helper()
	concepts, err := catalog.Concepts(db)
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	categories, err := catalog.Categories(db)
	if err != nil {
		t.Fatalf("Categories() unexpected error: %v", err)
	}

	m := New(db)
	m.today = september
	m.period = september
	m.view = viewConcepts
	m, _ = send(t, m, tea.WindowSizeMsg{Width: 100, Height: 30},
		catalogMsg{concepts: concepts, categories: categories})
	return m
}

func mustSeed(t *testing.T, db *sql.DB, category, name string, kind catalog.ConceptKind) catalog.Concept {
	t.Helper()
	cat, err := catalog.FindOrCreateCategory(db, category)
	if err != nil {
		t.Fatalf("FindOrCreateCategory() unexpected error: %v", err)
	}
	c := catalog.Concept{
		Name: name, CategoryID: cat.ID, Kind: kind,
		MonthMask: domain.Monthly, ActiveFrom: domain.NewPeriod(2026, time.January),
	}
	if kind != catalog.Chore {
		c.Money = &catalog.MoneyDetails{Currency: domain.ARS, Base: decimal.NewFromInt(1000)}
	}
	created, err := catalog.CreateConcept(db, c)
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}
	return created
}

// Rows group by category and the cursor runs over the concepts, skipping
// the headers between them.
func TestConceptRowsGroupByCategory(t *testing.T) {
	db := testDB(t)
	mustSeed(t, db, "Home", "Rent", catalog.Expense)
	mustSeed(t, db, "Utilities", "Gas", catalog.Expense)
	mustSeed(t, db, "Home", "Wash the house", catalog.Chore)

	m := conceptsModel(t, db)

	rows, anchors := m.conceptRows()
	if len(anchors) != 3 {
		t.Fatalf("anchors = %v, want one per concept", anchors)
	}
	if !strings.Contains(rows[0], "HOME") {
		t.Errorf("first row = %q, want the HOME group header", rows[0])
	}

	m, _ = send(t, m, key("down"), key("down"))
	if got, _ := m.cursorConcept(); got.Name != "Gas" {
		t.Errorf("cursor after two downs = %q, want Gas", got.Name)
	}
}

func TestConceptEditFormOpensOnTheCursorConcept(t *testing.T) {
	db := testDB(t)
	mustSeed(t, db, "Home", "Rent", catalog.Expense)
	m := conceptsModel(t, db)

	m, _ = send(t, m, key("e"))
	if _, ok := m.modal.(*form); !ok {
		t.Fatalf("modal = %T, want *form", m.modal)
	}
	if !strings.Contains(m.modal.View(), "Rent") {
		t.Errorf("form view does not name the concept:\n%s", m.modal.View())
	}
}

// Deleting is gated behind a confirm, and Keep leaves the catalog alone.
func TestConceptDeleteIsGatedAndKeepIsANoOp(t *testing.T) {
	db := testDB(t)
	mustSeed(t, db, "Home", "Rent", catalog.Expense)
	m := conceptsModel(t, db)

	m, cmd := send(t, m, key("d"))
	if _, ok := m.modal.(*form); !ok {
		t.Fatalf("modal = %T, want a confirm form", m.modal)
	}
	m, _ = pump(t, m, cmd)

	m, cmd = send(t, m, key("enter"))
	m, writes := pump(t, m, cmd)
	if m.modal != nil {
		t.Fatal("answering the confirm should close it")
	}
	if len(writes) != 0 {
		t.Fatalf("writes = %+v, want none — the confirm defaults to Keep", writes)
	}

	concepts, err := catalog.Concepts(db)
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	if len(concepts) != 1 {
		t.Errorf("Concepts() = %+v, want the concept still there", concepts)
	}
}

// A chore has no money fields to fill in; the form hides them outright.
func TestConceptFormHidesMoneyForAChore(t *testing.T) {
	db := testDB(t)
	mustSeed(t, db, "Home", "Wash the house", catalog.Chore)
	m := conceptsModel(t, db)

	m, cmd := send(t, m, key("e"))
	m, _ = pump(t, m, cmd)

	if strings.Contains(m.modal.View(), "Base amount") {
		t.Errorf("chore form shows the money fields:\n%s", m.modal.View())
	}
}

func TestMonthPresetResolvesToACadence(t *testing.T) {
	tests := []struct {
		preset monthPreset
		months []time.Month
		want   domain.Cadence
	}{
		{presetMonthly, nil, domain.Monthly},
		{presetAguinaldo, nil, domain.Aguinaldo},
		{presetOnce, nil, domain.NewCadence(time.September)},
		{presetPicked, []time.Month{time.March, time.July}, domain.NewCadence(time.March, time.July)},
	}
	for _, tt := range tests {
		v := &conceptValues{preset: tt.preset, months: tt.months, activeFrom: september.String()}
		if got := v.cadence(); got != tt.want {
			t.Errorf("preset %d cadence = %012b, want %012b", tt.preset, got, tt.want)
		}
	}
}

// A one-off is one bit plus a one-month active range, which month_mask and
// the range already express — there is no OneOff case to branch on.
func TestOneOffPresetClosesItsActiveRange(t *testing.T) {
	db := testDB(t)
	cat, err := catalog.FindOrCreateCategory(db, "Home")
	if err != nil {
		t.Fatalf("FindOrCreateCategory() unexpected error: %v", err)
	}

	v := &conceptValues{
		name: "New laptop", categoryID: cat.ID, kind: catalog.Expense,
		currency: domain.USD, base: "1200", preset: presetOnce,
		activeFrom: september.String(),
	}
	if err := v.save(db, 0); err != nil {
		t.Fatalf("save() unexpected error: %v", err)
	}

	concepts, err := catalog.Concepts(db)
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	c := concepts[0]
	if !c.ActiveFrom.Equal(september) || !c.ActiveUntil.Equal(september) {
		t.Errorf("active range = %s – %s, want september only", c.ActiveFrom, c.ActiveUntil)
	}
	if c.MonthMask != domain.NewCadence(time.September) {
		t.Errorf("month_mask = %012b, want september only", c.MonthMask)
	}
}

// Editing an existing concept opens on the preset that already describes it.
func TestPresetOfExistingCadence(t *testing.T) {
	tests := []struct {
		mask domain.Cadence
		want monthPreset
	}{
		{domain.Monthly, presetMonthly},
		{domain.Aguinaldo, presetAguinaldo},
		{domain.NewCadence(time.March, time.July), presetPicked},
	}
	for _, tt := range tests {
		if got := presetOf(tt.mask); got != tt.want {
			t.Errorf("presetOf(%012b) = %d, want %d", tt.mask, got, tt.want)
		}
	}
}

// A new concept goes active this month, not in whatever period another view
// was left on.
func TestNewConceptGoesActiveThisMonth(t *testing.T) {
	m := conceptsModel(t, testDB(t))
	m.period = september.AddMonths(4)

	if got := m.newConcept().ActiveFrom; !got.Equal(m.today) {
		t.Errorf("a new concept goes active in %s, want today's month %s", got, m.today)
	}
}
