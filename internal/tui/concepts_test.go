package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
)

func TestConceptEditFormOpensOnTheCursorConcept(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "1000"}},
	}, 100, 30)
	m.view = viewConcepts

	m, _ = send(t, m, key("e"))
	if _, ok := m.topModal().(*form); !ok {
		t.Fatalf("modal = %T, want *form", m.topModal())
	}
	if !strings.Contains(m.topModal().View(), "Rent") {
		t.Errorf("form view does not name the concept:\n%s", m.topModal().View())
	}
}

// Deleting is gated behind a confirm, and Keep leaves the catalog alone.
func TestConceptDeleteIsGatedAndKeepIsANoOp(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "1000"}},
	}, 100, 30)
	m.view = viewConcepts

	m, cmd := send(t, m, key("d"))
	if _, ok := m.topModal().(*form); !ok {
		t.Fatalf("modal = %T, want a confirm form", m.topModal())
	}
	m, _ = pump(t, m, cmd)

	m, cmd = send(t, m, key("enter"))
	m, writes := pump(t, m, cmd)
	if m.topModal() != nil {
		t.Fatal("answering the confirm should close it")
	}
	if len(writes) != 0 {
		t.Fatalf("writes = %+v, want none — the confirm defaults to Keep", writes)
	}

	concepts, err := catalog.Concepts(m.db)
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	if len(concepts) != 1 {
		t.Errorf("Concepts() = %+v, want the concept still there", concepts)
	}
}

// A chore has no money fields to fill in; the form hides them outright.
func TestConceptFormHidesMoneyForAChore(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{{Name: "Wash the house", Category: "Home", Kind: catalog.Chore}},
	}, 100, 30)
	m.view = viewConcepts

	m, cmd := send(t, m, key("e"))
	m, _ = pump(t, m, cmd)

	if strings.Contains(m.topModal().View(), "Base amount") {
		t.Errorf("chore form shows the money fields:\n%s", m.topModal().View())
	}
}

func TestMonthPresetResolvesToACadence(t *testing.T) {
	tests := []struct {
		preset monthPreset
		months []time.Month
		want   domain.Cadence
	}{
		{presetMonthly, nil, domain.Monthly},
		{presetPicked, []time.Month{time.June, time.December}, domain.NewCadence(time.June, time.December)},
		{presetPicked, []time.Month{time.March, time.July}, domain.NewCadence(time.March, time.July)},
	}
	for _, tt := range tests {
		v := &conceptValues{preset: tt.preset, months: tt.months, activeFrom: september.String()}
		if got := v.cadence(); got != tt.want {
			t.Errorf("preset %d cadence = %012b, want %012b", tt.preset, got, tt.want)
		}
	}
}

// A cadence is which months, and the window is how long. Picking one month
// leaves the window alone; closing it is the window's own field.
func TestPickingOneMonthLeavesTheWindowOpen(t *testing.T) {
	db := fixture.DB(t)
	cat, err := catalog.AppendCategory(db, "Home")
	if err != nil {
		t.Fatalf("AppendCategory() unexpected error: %v", err)
	}

	v := &conceptValues{
		name: "New laptop", categoryID: cat.ID, kind: catalog.Expense,
		currency: domain.USD, base: "1200", preset: presetPicked,
		months: []time.Month{time.September}, activeFrom: september.String(),
	}
	if err := v.save(db, 0); err != nil {
		t.Fatalf("save() unexpected error: %v", err)
	}

	concepts, err := catalog.Concepts(db)
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	c := concepts[0]
	if !c.ActiveUntil.IsZero() {
		t.Errorf("active until = %s, want it left open — no preset writes that field", c.ActiveUntil)
	}
	if c.MonthMask != domain.NewCadence(time.September) {
		t.Errorf("month_mask = %012b, want september only", c.MonthMask)
	}
}

// Editing opens on the preset that already describes the concept.
func TestPresetOfExistingCadence(t *testing.T) {
	tests := []struct {
		mask domain.Cadence
		want monthPreset
	}{
		{domain.Monthly, presetMonthly},
		{domain.NewCadence(time.June, time.December), presetPicked},
		{domain.NewCadence(time.March, time.July), presetPicked},
	}
	for _, tt := range tests {
		if got := presetOf(tt.mask); got != tt.want {
			t.Errorf("presetOf(%012b) = %d, want %d", tt.mask, got, tt.want)
		}
	}
}

// A new concept goes active this month, not the period another view was on.
func TestNewConceptGoesActiveThisMonth(t *testing.T) {
	m := modelFor(t, fixture.World{}, 100, 30)
	m.view = viewConcepts
	m.period = september.AddMonths(4)

	if got := m.newConcept().ActiveFrom; !got.Equal(m.today) {
		t.Errorf("a new concept goes active in %s, want today's month %s", got, m.today)
	}
}
