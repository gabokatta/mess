package tui

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"charm.land/huh/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func TestOpeningMonthWithNoConfirmedIncomePromptsToConfirm(t *testing.T) {
	db := openTestStore(t)
	period := domain.NewPeriod(2026, time.September)
	salary := seedConceptOfKind(t, db, "Sueldo", catalog.Income, 1000000)

	m := New(db)
	m.width, m.height = 100, 40
	m.period = period
	loaded := loadLines(t, db, period)

	updated, cmd := m.Update(monthLoadedMsg{lines: loaded.Lines})
	m = updated.(Model)
	if m.incomeConfirmForm == nil {
		t.Fatal("incomeConfirmForm = nil, want the prompt opened")
	}
	if m.incomeConfirmForm.values.conceptIDs[0] != salary.ID {
		t.Errorf("conceptIDs[0] = %d, want %d (Sueldo)", m.incomeConfirmForm.values.conceptIDs[0], salary.ID)
	}
	if m.incomeConfirmForm.values.amounts[0] != "1000000.00" {
		t.Errorf("amounts[0] = %q, want prefilled with the projected 1000000.00", m.incomeConfirmForm.values.amounts[0])
	}
	m = settle(t, m, cmd)

	content := m.View().Content
	if !strings.Contains(content, "Sueldo") {
		t.Errorf("content = %q, want the income concept's name in the prompt", content)
	}
}

func TestOpeningMonthWithConfirmedIncomeDoesNotPrompt(t *testing.T) {
	db := openTestStore(t)
	period := domain.NewPeriod(2026, time.September)
	salary := seedConceptOfKind(t, db, "Sueldo", catalog.Income, 1000000)
	amt := amountFor(t, "1000000")
	if err := catalog.SetMonthEntryAmount(db, salary.ID, period, &amt); err != nil {
		t.Fatalf("SetMonthEntryAmount() unexpected error: %v", err)
	}

	m := monthModel(t, db, period)

	if m.incomeConfirmForm != nil {
		t.Error("incomeConfirmForm should stay nil once income is already confirmed")
	}
}

func TestIncomeConfirmDoesNotReopenAfterDismissal(t *testing.T) {
	db := openTestStore(t)
	period := domain.NewPeriod(2026, time.September)
	seedConceptOfKind(t, db, "Sueldo", catalog.Income, 1000000)

	m := monthModel(t, db, period)
	if m.incomeConfirmForm == nil {
		t.Fatal("incomeConfirmForm = nil, want it opened on first load")
	}

	updated, _ := m.Update(keyEsc())
	m = updated.(Model)
	if m.incomeConfirmForm != nil {
		t.Fatal("esc should close the prompt")
	}

	loaded := loadLines(t, db, period)
	updated, _ = m.Update(monthLoadedMsg{lines: loaded.Lines})
	m = updated.(Model)
	if m.incomeConfirmForm != nil {
		t.Error("the prompt should not reopen on a later reload within the same session")
	}
}

// completeIncomeConfirmForm mutates the bound values as if the user had,
// then flips the form to StateCompleted directly — see completeSettingsForm.
func completeIncomeConfirmForm(m Model, mutate func(*incomeConfirmFormValues)) Model {
	mutate(m.incomeConfirmForm.values)
	m.incomeConfirmForm.form.State = huh.StateCompleted
	return m
}

func TestCompletingIncomeConfirmWritesOverrides(t *testing.T) {
	db := openTestStore(t)
	period := domain.NewPeriod(2026, time.September)
	salary := seedConceptOfKind(t, db, "Sueldo", catalog.Income, 1000000)

	m := monthModel(t, db, period)
	m = completeIncomeConfirmForm(m, func(v *incomeConfirmFormValues) {
		v.amounts[0] = "1100000"
	})

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	if m.incomeConfirmForm != nil {
		t.Fatal("a completed form should close")
	}
	m = settle(t, m, cmd)

	entries, err := catalog.MonthEntries(db, period)
	if err != nil {
		t.Fatalf("MonthEntries() unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].ConceptID != salary.ID || entries[0].Amount == nil || !entries[0].Amount.Equal(amountFor(t, "1100000")) {
		t.Fatalf("MonthEntries() = %+v, want a 1100000 override for %s", entries, salary.Name)
	}
}

func TestClearingIncomeConfirmFieldSkipsThatConcept(t *testing.T) {
	db := openTestStore(t)
	period := domain.NewPeriod(2026, time.September)
	seedConceptOfKind(t, db, "Sueldo", catalog.Income, 1000000)

	m := monthModel(t, db, period)
	m = completeIncomeConfirmForm(m, func(v *incomeConfirmFormValues) {
		v.amounts[0] = ""
	})

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	m = settle(t, m, cmd)

	entries, err := catalog.MonthEntries(db, period)
	if err != nil {
		t.Fatalf("MonthEntries() unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("MonthEntries() = %+v, want none (blank field skips confirmation)", entries)
	}
}

func seedConceptOfKind(t *testing.T, db *sql.DB, name string, kind catalog.ConceptKind, base int64) catalog.Concept {
	t.Helper()
	cat, err := catalog.CreateCategory(db, name, 0)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	c, err := catalog.CreateConcept(db, catalog.Concept{
		Name:       name,
		CategoryID: cat.ID,
		Kind:       kind,
		Money:      &catalog.MoneyDetails{Currency: domain.ARS},
		MonthMask:  domain.Monthly,
		ActiveFrom: domain.NewPeriod(2026, time.January),
	})
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}
	if err := catalog.SetBaseAmount(db, c.ID, domain.NewPeriod(2026, time.January), decimal.NewFromInt(base)); err != nil {
		t.Fatalf("SetBaseAmount() unexpected error: %v", err)
	}
	return c
}
