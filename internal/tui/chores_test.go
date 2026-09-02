package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/huh/v2"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func TestNKeyInMonthViewOpensNewChoreFormAndRendersIt(t *testing.T) {
	db := openTestStore(t)
	m := monthModel(t, db, domain.NewPeriod(2026, time.September))

	updated, cmd := m.Update(key("n"))
	m = updated.(Model)
	if m.choreForm == nil {
		t.Fatal("choreForm = nil, want a form opened")
	}
	m = settle(t, m, cmd)

	content := m.View().Content
	if !strings.Contains(content, "New chore") || !strings.Contains(content, "Name") {
		t.Errorf("content = %q, want the form's title and Name field", content)
	}
}

func TestNKeyOnAConceptLineDoesNotOpenTheChoreForm(t *testing.T) {
	db := openTestStore(t)
	seedConcept(t, db, "Alquiler", 785000)
	m := monthModel(t, db, domain.NewPeriod(2026, time.September))

	updated, _ := m.Update(key("n"))
	m = updated.(Model)
	if m.choreForm != nil {
		t.Error("choreForm should stay nil — the cursor is on a concept line, not free to add a chore")
	}
}

// completeChoreForm fills the bound values as if the user had, then flips
// the form to StateCompleted directly — see completeConceptForm.
func completeChoreForm(m Model, mutate func(*choreFormValues)) Model {
	mutate(m.choreForm.values)
	m.choreForm.form.State = huh.StateCompleted
	return m
}

func TestCompletingChoreFormCreatesIt(t *testing.T) {
	db := openTestStore(t)
	period := domain.NewPeriod(2026, time.September)
	m := monthModel(t, db, period)

	updated, _ := m.Update(key("n"))
	m = updated.(Model)
	m = completeChoreForm(m, func(v *choreFormValues) {
		v.name = "Sacar la basura"
		v.activeFrom = "2026-01"
	})

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	if m.choreForm != nil {
		t.Fatal("a completed form should close")
	}
	m = settle(t, m, cmd)

	chores, err := catalog.Chores(db)
	if err != nil {
		t.Fatalf("catalog.Chores() unexpected error: %v", err)
	}
	if len(chores) != 1 || chores[0].Name != "Sacar la basura" {
		t.Fatalf("Chores() = %+v, want the created chore", chores)
	}
	if chores[0].MonthMask != domain.Monthly {
		t.Errorf("MonthMask = %v, want Monthly (every month selected by default)", chores[0].MonthMask)
	}

	content := m.View().Content
	if !strings.Contains(content, "Sacar la basura") {
		t.Errorf("content = %q, want the new chore visible in the month view", content)
	}
}

func TestEscCancelsNewChoreFormWithoutWriting(t *testing.T) {
	db := openTestStore(t)
	m := monthModel(t, db, domain.NewPeriod(2026, time.September))

	updated, _ := m.Update(key("n"))
	m = updated.(Model)
	updated, cmd := m.Update(keyEsc())
	m = updated.(Model)
	if m.choreForm != nil {
		t.Error("esc should close the form")
	}
	if cmd != nil {
		t.Error("esc should not write anything")
	}

	chores, err := catalog.Chores(db)
	if err != nil {
		t.Fatalf("catalog.Chores() unexpected error: %v", err)
	}
	if len(chores) != 0 {
		t.Errorf("Chores() = %+v, want none created", chores)
	}
}
