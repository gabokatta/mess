package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/huh/v2"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func TestEKeyOnConceptsOpensEditFormPrefilled(t *testing.T) {
	db := openTestStore(t)
	c := seedConcept(t, db, "Alquiler", 785000)
	m := conceptsModel(t, db, domain.NewPeriod(2026, time.September))

	updated, cmd := m.Update(key("e"))
	m = updated.(Model)
	if m.conceptEditForm == nil {
		t.Fatal("conceptEditForm = nil, want a form opened")
	}
	v := m.conceptEditForm.values
	if v.name != "Alquiler" || v.categoryID != c.CategoryID {
		t.Errorf("values = %+v, want prefilled from %+v", v, c)
	}
	if v.amount != "785000.00" {
		t.Errorf("values.amount = %q, want prefilled with the latest base amount", v.amount)
	}
	m = settle(t, m, cmd)

	content := m.View().Content
	if !strings.Contains(content, "Edit concept") || !strings.Contains(content, "Alquiler") {
		t.Errorf("content = %q, want the edit form's title and prefilled name", content)
	}
}

func TestJKMovesConceptCursor(t *testing.T) {
	db := openTestStore(t)
	seedConcept(t, db, "Alquiler", 785000)
	seedConcept(t, db, "Internet", 15000)
	m := conceptsModel(t, db, domain.NewPeriod(2026, time.September))

	updated, _ := m.Update(key("j"))
	m = updated.(Model)
	if m.conceptCursor != 1 {
		t.Fatalf("conceptCursor = %d, want 1 after j", m.conceptCursor)
	}
	updated, _ = m.Update(key("j"))
	m = updated.(Model)
	if m.conceptCursor != 1 {
		t.Fatalf("conceptCursor = %d, want clamped at 1 (last concept)", m.conceptCursor)
	}
}

// completeConceptEditForm fills the bound values as if the user had, then
// flips the form to StateCompleted directly — see completeConceptForm.
func completeConceptEditForm(m Model, mutate func(*conceptEditFormValues)) Model {
	mutate(m.conceptEditForm.values)
	m.conceptEditForm.form.State = huh.StateCompleted
	return m
}

func TestCompletingEditFormPreservesSortOrder(t *testing.T) {
	db := openTestStore(t)
	cat, err := catalog.CreateCategory(db, "Hogar", 0)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	c, err := catalog.CreateConcept(db, catalog.Concept{
		Name: "Alquiler", CategoryID: cat.ID, Kind: catalog.Expense, Money: &catalog.MoneyDetails{Currency: domain.ARS},
		MonthMask: domain.Monthly, SortOrder: 3, ActiveFrom: domain.NewPeriod(2026, time.January),
	})
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}
	if err := catalog.SetBaseAmount(db, c.ID, domain.NewPeriod(2026, time.January), amountFor(t, "785000")); err != nil {
		t.Fatalf("SetBaseAmount() unexpected error: %v", err)
	}
	m := conceptsModel(t, db, domain.NewPeriod(2026, time.September))

	updated, _ := m.Update(key("e"))
	m = updated.(Model)
	m = completeConceptEditForm(m, func(v *conceptEditFormValues) {
		v.name = "Alquiler nuevo"
	})

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	m = settle(t, m, cmd)

	concepts, err := catalog.Concepts(db)
	if err != nil {
		t.Fatalf("catalog.Concepts() unexpected error: %v", err)
	}
	if len(concepts) != 1 || concepts[0].SortOrder != 3 {
		t.Fatalf("Concepts()[0].SortOrder = %d, want 3 preserved, not reset to 0", concepts[0].SortOrder)
	}
}

func TestCompletingEditFormUpdatesTheConceptInPlace(t *testing.T) {
	db := openTestStore(t)
	c := seedConcept(t, db, "Alquiler", 785000)
	m := conceptsModel(t, db, domain.NewPeriod(2026, time.September))

	updated, _ := m.Update(key("e"))
	m = updated.(Model)
	m = completeConceptEditForm(m, func(v *conceptEditFormValues) {
		v.name = "Alquiler nuevo"
	})

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	if m.conceptEditForm != nil {
		t.Fatal("a completed form should close")
	}
	m = settle(t, m, cmd)

	concepts, err := catalog.Concepts(db)
	if err != nil {
		t.Fatalf("catalog.Concepts() unexpected error: %v", err)
	}
	if len(concepts) != 1 || concepts[0].ID != c.ID || concepts[0].Name != "Alquiler nuevo" {
		t.Fatalf("Concepts() = %+v, want the same row renamed, not a duplicate", concepts)
	}
}

func TestEditingKindToChoreDropsMoney(t *testing.T) {
	db := openTestStore(t)
	c := seedConcept(t, db, "Alquiler", 785000)
	m := conceptsModel(t, db, domain.NewPeriod(2026, time.September))

	updated, _ := m.Update(key("e"))
	m = updated.(Model)
	m = completeConceptEditForm(m, func(v *conceptEditFormValues) {
		v.kind = catalog.Chore
	})

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	m = settle(t, m, cmd)

	concepts, err := catalog.Concepts(db)
	if err != nil {
		t.Fatalf("catalog.Concepts() unexpected error: %v", err)
	}
	if len(concepts) != 1 || concepts[0].ID != c.ID || concepts[0].Kind != catalog.Chore || concepts[0].Money != nil {
		t.Fatalf("Concepts()[0] = %+v, want %s switched to Chore with Money nil", concepts, c.Name)
	}
}

func TestEditingAmountWithANewEffectiveDateAddsARaiseWithoutTouchingHistory(t *testing.T) {
	db := openTestStore(t)
	c := seedConcept(t, db, "Alquiler", 785000)
	m := conceptsModel(t, db, domain.NewPeriod(2026, time.September))

	updated, _ := m.Update(key("e"))
	m = updated.(Model)
	m = completeConceptEditForm(m, func(v *conceptEditFormValues) {
		v.amount = "900000"
		v.amountEffective = "2026-03"
	})

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	m = settle(t, m, cmd)

	amounts, err := catalog.BaseAmounts(db, c.ID)
	if err != nil {
		t.Fatalf("catalog.BaseAmounts() unexpected error: %v", err)
	}
	if len(amounts) != 2 {
		t.Fatalf("BaseAmounts() = %+v, want the original January row plus the new March raise", amounts)
	}
	if !amounts[0].Amount.Equal(amountFor(t, "785000")) || !amounts[0].EffectiveFrom.Equal(domain.NewPeriod(2026, time.January)) {
		t.Errorf("BaseAmounts()[0] = %+v, want January's 785000 untouched", amounts[0])
	}
	if !amounts[1].Amount.Equal(amountFor(t, "900000")) || !amounts[1].EffectiveFrom.Equal(domain.NewPeriod(2026, time.March)) {
		t.Errorf("BaseAmounts()[1] = %+v, want the new 900000 row effective March", amounts[1])
	}
}

func TestEditingAmountWithTheSameEffectiveDateCorrectsItInPlace(t *testing.T) {
	db := openTestStore(t)
	c := seedConcept(t, db, "Alquiler", 785000)
	m := conceptsModel(t, db, domain.NewPeriod(2026, time.September))

	updated, _ := m.Update(key("e"))
	m = updated.(Model)
	m = completeConceptEditForm(m, func(v *conceptEditFormValues) {
		v.amount = "800000"
	})

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	m = settle(t, m, cmd)

	amounts, err := catalog.BaseAmounts(db, c.ID)
	if err != nil {
		t.Fatalf("catalog.BaseAmounts() unexpected error: %v", err)
	}
	if len(amounts) != 1 || !amounts[0].Amount.Equal(amountFor(t, "800000")) {
		t.Fatalf("BaseAmounts() = %+v, want the January row corrected in place, not a second row", amounts)
	}
}

func TestEscCancelsConceptEditFormWithoutWriting(t *testing.T) {
	db := openTestStore(t)
	seedConcept(t, db, "Alquiler", 785000)
	m := conceptsModel(t, db, domain.NewPeriod(2026, time.September))

	updated, _ := m.Update(key("e"))
	m = updated.(Model)
	m.conceptEditForm.values.name = "should not persist"

	updated, cmd := m.Update(keyEsc())
	m = updated.(Model)
	if m.conceptEditForm != nil {
		t.Error("esc should close the form")
	}
	if cmd != nil {
		t.Error("esc should not write anything")
	}

	concepts, err := catalog.Concepts(db)
	if err != nil {
		t.Fatalf("catalog.Concepts() unexpected error: %v", err)
	}
	if concepts[0].Name != "Alquiler" {
		t.Errorf("Concepts()[0].Name = %q, want unchanged", concepts[0].Name)
	}
}
