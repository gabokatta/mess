package tui

import (
	"testing"
	"time"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
)

func TestCursorReachesAllocationsAfterConceptLinesAndChores(t *testing.T) {
	period := domain.NewPeriod(2026, time.September)
	m := New(openTestStore(t))
	m.width, m.height = 100, 40
	m.period = period
	rent := catalog.Concept{Name: "Alquiler", Kind: catalog.Expense, Currency: domain.ARS}
	trash := catalog.Chore{Name: "Sacar la basura"}
	updated, _ := m.Update(monthLoadedMsg{
		lines:  []month.Line{{Concept: rent, Amount: amountFor(t, "785000")}},
		chores: []month.ChoreLine{{Chore: trash}},
	})
	m = updated.(Model)
	updated, _ = m.Update(allocationsLoadedMsg{allocations: []catalog.SavingAllocation{
		{ID: 1, Period: period, Destination: catalog.Cash, Amount: amountFor(t, "10000"), Currency: domain.ARS},
	}})
	m = updated.(Model)

	updated, _ = m.Update(key("j"))
	m = updated.(Model)
	updated, _ = m.Update(key("j"))
	m = updated.(Model)
	if m.cursor != 2 {
		t.Fatalf("cursor = %d, want 2 (the allocation row, after the concept line and the chore)", m.cursor)
	}
	a, ok := m.cursorAllocation()
	if !ok || a.ID != 1 {
		t.Fatalf("cursorAllocation() = %+v, %v, want the seeded allocation", a, ok)
	}
}

func TestDKeyDeletesTheAllocationUnderTheCursor(t *testing.T) {
	db := openTestStore(t)
	period := domain.NewPeriod(2026, time.September)
	a, err := catalog.CreateSavingAllocation(db, catalog.SavingAllocation{
		Period: period, Destination: catalog.Cash, Amount: amountFor(t, "10000"), Currency: domain.ARS,
	})
	if err != nil {
		t.Fatalf("CreateSavingAllocation() unexpected error: %v", err)
	}
	m := New(db)
	m.width, m.height = 100, 40
	m.period = period
	updated, _ := m.Update(monthLoadedMsg{})
	m = updated.(Model)
	updated, _ = m.Update(allocationsLoadedMsg{allocations: []catalog.SavingAllocation{a}})
	m = updated.(Model)

	updated, cmd := m.Update(key("d"))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("d returned no Cmd, want a delete")
	}
	m = settle(t, m, cmd)

	allocations, err := catalog.SavingAllocations(db, period)
	if err != nil {
		t.Fatalf("SavingAllocations() unexpected error: %v", err)
	}
	if len(allocations) != 0 {
		t.Errorf("SavingAllocations() = %+v, want none left", allocations)
	}
}

func TestDKeyOnAConceptLineDoesNothing(t *testing.T) {
	db := openTestStore(t)
	period := domain.NewPeriod(2026, time.September)
	c := seedConcept(t, db, "Alquiler", 785000)
	m := monthModel(t, db, period)

	updated, cmd := m.Update(key("d"))
	m = updated.(Model)
	if cmd != nil {
		t.Error("d on a concept line should not write anything")
	}

	concepts, err := catalog.Concepts(db)
	if err != nil {
		t.Fatalf("catalog.Concepts() unexpected error: %v", err)
	}
	if len(concepts) != 1 || concepts[0].ID != c.ID {
		t.Errorf("Concepts() = %+v, want the concept untouched", concepts)
	}
}
