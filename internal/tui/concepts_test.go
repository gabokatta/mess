package tui

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"charm.land/huh/v2"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func conceptsModel(t *testing.T, db *sql.DB, period domain.Period) Model {
	t.Helper()
	m := New(db)
	m.width, m.height = 100, 40
	m.period = period
	m.view = viewConcepts
	concepts, err := catalog.Concepts(db)
	if err != nil {
		t.Fatalf("catalog.Concepts() unexpected error: %v", err)
	}
	categories, err := catalog.Categories(db)
	if err != nil {
		t.Fatalf("catalog.Categories() unexpected error: %v", err)
	}
	baseAmounts, err := catalog.AllBaseAmounts(db)
	if err != nil {
		t.Fatalf("catalog.AllBaseAmounts() unexpected error: %v", err)
	}
	updated, _ := m.Update(conceptsLoadedMsg{concepts: concepts, categories: categories, baseAmounts: baseAmounts})
	return updated.(Model)
}

func TestConceptsViewRendersEmptyState(t *testing.T) {
	db := openTestStore(t)
	m := conceptsModel(t, db, domain.NewPeriod(2026, time.September))
	content := m.View().Content
	if !strings.Contains(content, "no concepts yet") {
		t.Errorf("content = %q, want the empty state", content)
	}
}

func TestConceptsViewListsConceptsGroupedByCategory(t *testing.T) {
	db := openTestStore(t)
	cat, err := catalog.CreateCategory(db, "Hogar", 0)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	concept, err := catalog.CreateConcept(db, catalog.Concept{
		Name: "Alquiler", CategoryID: cat.ID, Kind: catalog.FixedExpense, Currency: domain.ARS,
		MonthMask: domain.Monthly, ActiveFrom: domain.NewPeriod(2026, 1),
	})
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}
	if err := catalog.SetBaseAmount(db, concept.ID, domain.NewPeriod(2026, 1), amountFor(t, "785000")); err != nil {
		t.Fatalf("SetBaseAmount() unexpected error: %v", err)
	}

	m := conceptsModel(t, db, domain.NewPeriod(2026, time.September))
	content := m.View().Content
	for _, want := range []string{"Hogar", "Alquiler", "785000"} {
		if !strings.Contains(content, want) {
			t.Errorf("content = %q, want it to contain %q", content, want)
		}
	}
}

func TestNKeyOpensNewConceptFormAndRendersIt(t *testing.T) {
	db := openTestStore(t)
	m := conceptsModel(t, db, domain.NewPeriod(2026, time.September))

	updated, cmd := m.Update(key("n"))
	m = updated.(Model)
	if m.conceptForm == nil {
		t.Fatal("conceptForm = nil, want a form opened")
	}
	m = settle(t, m, cmd)

	content := m.View().Content
	if !strings.Contains(content, "New concept") || !strings.Contains(content, "Name") {
		t.Errorf("content = %q, want the form's title and Name field", content)
	}
}

// completeConceptForm fills the bound values as if the user had, then flips
// the form to StateCompleted directly — Huh's own field-by-field navigation
// and widget key handling are its library's concern, not this app's.
func completeConceptForm(m Model, mutate func(*conceptFormValues)) Model {
	mutate(m.conceptForm.values)
	m.conceptForm.form.State = huh.StateCompleted
	return m
}

func TestCompletingFormCreatesConceptCategoryAndBaseAmount(t *testing.T) {
	db := openTestStore(t)
	current := domain.NewPeriod(2026, time.September)
	m := conceptsModel(t, db, current)

	updated, _ := m.Update(key("n"))
	m = updated.(Model)
	m = completeConceptForm(m, func(v *conceptFormValues) {
		v.name = "Alquiler"
		v.category = "Hogar"
		v.kind = catalog.FixedExpense
		v.currency = domain.ARS
		v.amount = "785000"
		v.dueDay = "10"
		v.activeFrom = "2026-01"
		v.months = []time.Month{time.January, time.February, time.March, time.April, time.May, time.June,
			time.July, time.August, time.September, time.October, time.November, time.December}
	})

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	if m.conceptForm != nil {
		t.Fatal("a completed form should close")
	}
	m = settle(t, m, cmd)

	concepts, err := catalog.Concepts(db)
	if err != nil {
		t.Fatalf("catalog.Concepts() unexpected error: %v", err)
	}
	if len(concepts) != 1 {
		t.Fatalf("Concepts() returned %d rows, want 1", len(concepts))
	}
	c := concepts[0]
	if c.Name != "Alquiler" || c.Kind != catalog.FixedExpense || c.Currency != domain.ARS || c.DueDay != 10 {
		t.Errorf("Concepts()[0] = %+v, want the entered fields", c)
	}
	if c.MonthMask != domain.Monthly {
		t.Errorf("Concepts()[0].MonthMask = %v, want Monthly (every month selected)", c.MonthMask)
	}

	categories, err := catalog.Categories(db)
	if err != nil {
		t.Fatalf("catalog.Categories() unexpected error: %v", err)
	}
	if len(categories) != 1 || categories[0].Name != "Hogar" {
		t.Errorf("Categories() = %+v, want a single Hogar row created from the form", categories)
	}

	amounts, err := catalog.BaseAmounts(db, c.ID)
	if err != nil {
		t.Fatalf("catalog.BaseAmounts() unexpected error: %v", err)
	}
	if len(amounts) != 1 || !amounts[0].Amount.Equal(amountFor(t, "785000")) {
		t.Errorf("BaseAmounts() = %+v, want a single 785000 entry", amounts)
	}
}

func TestCompletingFormWithDeselectedMonthsWritesPartialCadence(t *testing.T) {
	db := openTestStore(t)
	current := domain.NewPeriod(2026, time.September)
	m := conceptsModel(t, db, current)

	updated, _ := m.Update(key("n"))
	m = updated.(Model)
	m = completeConceptForm(m, func(v *conceptFormValues) {
		v.name = "Aguinaldo"
		v.category = "Ingresos"
		v.kind = catalog.Income
		v.currency = domain.ARS
		v.amount = "500000"
		v.activeFrom = "2026-01"
		v.months = []time.Month{time.June, time.December}
	})

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	m = settle(t, m, cmd)

	concepts, err := catalog.Concepts(db)
	if err != nil {
		t.Fatalf("catalog.Concepts() unexpected error: %v", err)
	}
	if len(concepts) != 1 || concepts[0].MonthMask != domain.Aguinaldo {
		t.Errorf("Concepts() = %+v, want MonthMask = Aguinaldo (June + December only)", concepts)
	}
}

func TestEscCancelsNewConceptFormWithoutWriting(t *testing.T) {
	db := openTestStore(t)
	m := conceptsModel(t, db, domain.NewPeriod(2026, time.September))

	updated, _ := m.Update(key("n"))
	m = updated.(Model)
	updated, cmd := m.Update(keyEsc())
	m = updated.(Model)
	if m.conceptForm != nil {
		t.Error("esc should close the form")
	}
	if cmd != nil {
		t.Error("esc should not write anything")
	}

	concepts, err := catalog.Concepts(db)
	if err != nil {
		t.Fatalf("catalog.Concepts() unexpected error: %v", err)
	}
	if len(concepts) != 0 {
		t.Errorf("Concepts() = %+v, want none created", concepts)
	}
}
