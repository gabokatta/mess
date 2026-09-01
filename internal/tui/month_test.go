package tui

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
)

func seedConcept(t *testing.T, db *sql.DB, name string, base int64) catalog.Concept {
	t.Helper()
	cat, err := catalog.CreateCategory(db, name, 0)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	c, err := catalog.CreateConcept(db, catalog.Concept{
		Name:       name,
		CategoryID: cat.ID,
		Kind:       catalog.FixedExpense,
		Currency:   domain.ARS,
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

func seedChore(t *testing.T, db *sql.DB, name string) catalog.Chore {
	t.Helper()
	c, err := catalog.CreateChore(db, catalog.Chore{
		Name:       name,
		MonthMask:  domain.Monthly,
		ActiveFrom: domain.NewPeriod(2026, time.January),
	})
	if err != nil {
		t.Fatalf("CreateChore() unexpected error: %v", err)
	}
	return c
}

func loadLines(t *testing.T, db *sql.DB, period domain.Period) month.Month {
	t.Helper()
	loaded, err := month.Load(db, period)
	if err != nil {
		t.Fatalf("month.Load() unexpected error: %v", err)
	}
	return loaded
}

func monthModel(t *testing.T, db *sql.DB, period domain.Period) Model {
	t.Helper()
	m := New(db)
	m.width, m.height = 100, 40
	m.period = period
	loaded := loadLines(t, db, period)
	updated, _ := m.Update(monthLoadedMsg{lines: loaded.Lines, chores: loaded.Chores})
	return updated.(Model)
}

func TestCursorMovesWithJKAndClamps(t *testing.T) {
	db := openTestStore(t)
	seedConcept(t, db, "Alquiler", 785000)
	seedConcept(t, db, "Internet", 15000)
	period := domain.NewPeriod(2026, time.January)

	m := monthModel(t, db, period)
	if m.cursor != 0 {
		t.Fatalf("cursor = %d, want 0 at load", m.cursor)
	}

	updated, _ := m.Update(key("j"))
	m = updated.(Model)
	if m.cursor != 1 {
		t.Fatalf("cursor = %d, want 1 after j", m.cursor)
	}

	updated, _ = m.Update(key("j"))
	m = updated.(Model)
	if m.cursor != 1 {
		t.Fatalf("cursor = %d, want clamped at 1 (last line)", m.cursor)
	}

	updated, _ = m.Update(key("k"))
	m = updated.(Model)
	if m.cursor != 0 {
		t.Fatalf("cursor = %d, want 0 after k", m.cursor)
	}
}

func TestSpaceTicksDoneAndOpensAmountEdit(t *testing.T) {
	db := openTestStore(t)
	c := seedConcept(t, db, "Alquiler", 785000)
	period := domain.NewPeriod(2026, time.January)

	m := monthModel(t, db, period)

	updated, cmd := m.Update(keySpace())
	m = updated.(Model)
	if m.editing == nil || m.editing.conceptID != c.ID {
		t.Fatalf("editing = %+v, want editing opened for %s", m.editing, c.Name)
	}
	if got := m.editing.input.Value(); got != "785000.00" {
		t.Errorf("edit input value = %q, want prefilled with the base amount 785000.00", got)
	}

	m = settle(t, m, cmd)

	entries, err := catalog.MonthEntries(db, period)
	if err != nil {
		t.Fatalf("MonthEntries() unexpected error: %v", err)
	}
	if len(entries) != 1 || !entries[0].Done {
		t.Fatalf("MonthEntries() = %+v, want done=true persisted", entries)
	}
}

func TestSpaceUntickingDoesNotOpenEdit(t *testing.T) {
	db := openTestStore(t)
	c := seedConcept(t, db, "Alquiler", 785000)
	period := domain.NewPeriod(2026, time.January)
	if err := catalog.SetMonthEntryDone(db, c.ID, period, true); err != nil {
		t.Fatalf("SetMonthEntryDone() unexpected error: %v", err)
	}

	m := monthModel(t, db, period)

	updated, cmd := m.Update(keySpace())
	m = updated.(Model)
	if m.editing != nil {
		t.Error("un-ticking should not open the amount edit")
	}
	m = settle(t, m, cmd)

	entries, err := catalog.MonthEntries(db, period)
	if err != nil {
		t.Fatalf("MonthEntries() unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].Done {
		t.Fatalf("MonthEntries() = %+v, want done=false", entries)
	}
}

func TestEnterCommitsTypedAmountAsOverride(t *testing.T) {
	db := openTestStore(t)
	seedConcept(t, db, "Alquiler", 785000)
	period := domain.NewPeriod(2026, time.January)

	m := monthModel(t, db, period)

	updated, _ := m.Update(keyEnter())
	m = updated.(Model)
	if m.editing == nil {
		t.Fatal("enter did not open the amount edit")
	}
	m.editing.input.SetValue("800000")

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	if m.editing != nil {
		t.Fatal("enter should close the edit after committing")
	}
	m = settle(t, m, cmd)

	entries, err := catalog.MonthEntries(db, period)
	if err != nil {
		t.Fatalf("MonthEntries() unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].Amount == nil || !entries[0].Amount.Equal(decimal.NewFromInt(800000)) {
		t.Fatalf("MonthEntries() = %+v, want an 800000 override", entries)
	}
}

func TestEscCancelsEditWithoutWriting(t *testing.T) {
	db := openTestStore(t)
	seedConcept(t, db, "Alquiler", 785000)
	period := domain.NewPeriod(2026, time.January)

	m := monthModel(t, db, period)

	updated, _ := m.Update(keyEnter())
	m = updated.(Model)
	m.editing.input.SetValue("999999")

	updated, cmd := m.Update(keyEsc())
	m = updated.(Model)
	if m.editing != nil {
		t.Error("esc should close the edit")
	}
	if cmd != nil {
		t.Error("esc should not write anything")
	}

	entries, err := catalog.MonthEntries(db, period)
	if err != nil {
		t.Fatalf("MonthEntries() unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("MonthEntries() = %+v, want no rows after cancel", entries)
	}
}

func TestClearingAmountRemovesOverride(t *testing.T) {
	db := openTestStore(t)
	c := seedConcept(t, db, "Alquiler", 785000)
	period := domain.NewPeriod(2026, time.January)
	amt := decimal.NewFromInt(800000)
	if err := catalog.SetMonthEntryAmount(db, c.ID, period, &amt); err != nil {
		t.Fatalf("SetMonthEntryAmount() unexpected error: %v", err)
	}

	m := monthModel(t, db, period)

	updated, _ := m.Update(keyEnter())
	m = updated.(Model)
	if got := m.editing.input.Value(); got != "800000.00" {
		t.Fatalf("edit input value = %q, want prefilled with the current override 800000.00", got)
	}
	m.editing.input.SetValue("")

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	m = settle(t, m, cmd)

	entries, err := catalog.MonthEntries(db, period)
	if err != nil {
		t.Fatalf("MonthEntries() unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].Amount != nil {
		t.Fatalf("MonthEntries() = %+v, want the override cleared", entries)
	}
}

func TestCursorMovesFromConceptsOntoChores(t *testing.T) {
	db := openTestStore(t)
	seedConcept(t, db, "Alquiler", 785000)
	seedChore(t, db, "Sacar la basura")
	period := domain.NewPeriod(2026, time.January)

	m := monthModel(t, db, period)

	updated, _ := m.Update(key("j"))
	m = updated.(Model)
	if m.cursor != 1 {
		t.Fatalf("cursor = %d, want 1 (on the chore row)", m.cursor)
	}

	updated, _ = m.Update(key("j"))
	m = updated.(Model)
	if m.cursor != 1 {
		t.Fatalf("cursor = %d, want clamped at 1 (last row is the chore)", m.cursor)
	}
}

func TestSpaceTogglesChoreDoneWithoutOpeningEdit(t *testing.T) {
	db := openTestStore(t)
	c := seedChore(t, db, "Sacar la basura")
	period := domain.NewPeriod(2026, time.January)

	m := monthModel(t, db, period)

	updated, cmd := m.Update(keySpace())
	m = updated.(Model)
	if m.editing != nil {
		t.Error("ticking a chore should not open the amount edit")
	}
	m = settle(t, m, cmd)

	entries, err := catalog.ChoreEntries(db, period)
	if err != nil {
		t.Fatalf("ChoreEntries() unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].ChoreID != c.ID || !entries[0].Done {
		t.Fatalf("ChoreEntries() = %+v, want %s done=true persisted", entries, c.Name)
	}
}

func TestMonthViewRendersProjectedAndConfirmedTotals(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40

	rent := catalog.Concept{Name: "Alquiler", Kind: catalog.FixedExpense, Currency: domain.ARS, Share: domain.NewPercent(50)}
	salary := catalog.Concept{Name: "Sueldo", Kind: catalog.Income, Currency: domain.ARS, Share: domain.NewPercent(100)}
	lines := []month.Line{
		{Concept: salary, Amount: amountFor(t, "1000000"), Confirmed: true, Done: true},
		{Concept: rent, Amount: amountFor(t, "785000"), Confirmed: false, Done: false},
	}

	updated, _ := m.Update(monthLoadedMsg{lines: lines})
	m = updated.(Model)
	content := m.View().Content

	for _, want := range []string{"215000.00", "607500.00", "1000000.00"} {
		if !strings.Contains(content, want) {
			t.Errorf("month view content missing %q (projected/confirmed totals):\n%s", want, content)
		}
	}
}

func TestMonthViewRendersChoresGroup(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40

	rent := catalog.Concept{Name: "Alquiler", Kind: catalog.FixedExpense, Currency: domain.ARS}
	trash := catalog.Chore{Name: "Sacar la basura"}
	updated, _ := m.Update(monthLoadedMsg{
		lines:  []month.Line{{Concept: rent, Amount: amountFor(t, "785000"), Confirmed: false, Done: false}},
		chores: []month.ChoreLine{{Chore: trash, Done: true}},
	})
	m = updated.(Model)
	content := m.View().Content

	for _, want := range []string{"Chores", "Sacar la basura"} {
		if !strings.Contains(content, want) {
			t.Errorf("month view content missing %q:\n%s", want, content)
		}
	}
}

func TestMonthViewShowsEditBoxForFocusedLine(t *testing.T) {
	db := openTestStore(t)
	seedConcept(t, db, "Alquiler", 785000)
	period := domain.NewPeriod(2026, time.January)

	m := monthModel(t, db, period)

	updated, _ := m.Update(keyEnter())
	m = updated.(Model)

	content := m.View().Content
	if !strings.Contains(content, "785000.00") {
		t.Errorf("month view content = %q, want the prefilled edit box visible", content)
	}
}
