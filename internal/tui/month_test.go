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
		Kind:       catalog.Expense,
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

func seedChore(t *testing.T, db *sql.DB, name string) catalog.Concept {
	t.Helper()
	cat, err := catalog.CreateCategory(db, name, 0)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	c, err := catalog.CreateConcept(db, catalog.Concept{
		Name:       name,
		CategoryID: cat.ID,
		Kind:       catalog.Chore,
		MonthMask:  domain.Monthly,
		ActiveFrom: domain.NewPeriod(2026, time.January),
	})
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
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
	updated, _ := m.Update(monthLoadedMsg{lines: loaded.Lines})
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

func TestSpaceTicksDoneWithoutOpeningEdit(t *testing.T) {
	db := openTestStore(t)
	c := seedConcept(t, db, "Alquiler", 785000)
	period := domain.NewPeriod(2026, time.January)

	m := monthModel(t, db, period)

	updated, cmd := m.Update(keySpace())
	m = updated.(Model)
	if m.editing != nil {
		t.Fatalf("editing = %+v, want ticking to leave the amount edit closed", m.editing)
	}

	m = settle(t, m, cmd)

	entries, err := catalog.MonthEntries(db, period)
	if err != nil {
		t.Fatalf("MonthEntries() unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].ConceptID != c.ID || !entries[0].Done {
		t.Fatalf("MonthEntries() = %+v, want %s done=true persisted", entries, c.Name)
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

func TestECommitsTypedAmountAsOverride(t *testing.T) {
	db := openTestStore(t)
	seedConcept(t, db, "Alquiler", 785000)
	period := domain.NewPeriod(2026, time.January)

	m := monthModel(t, db, period)

	updated, _ := m.Update(key("e"))
	m = updated.(Model)
	if m.editing == nil {
		t.Fatal("e did not open the amount edit")
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

	updated, _ := m.Update(key("e"))
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

	updated, _ := m.Update(key("e"))
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

func TestCursorReachesChoreLinesAfterMoneyLines(t *testing.T) {
	db := openTestStore(t)
	seedConcept(t, db, "Alquiler", 785000)
	seedChore(t, db, "Sacar la basura")
	period := domain.NewPeriod(2026, time.January)

	m := monthModel(t, db, period)

	updated, _ := m.Update(key("j"))
	m = updated.(Model)
	if m.cursor != 1 {
		t.Fatalf("cursor = %d, want 1 (Chore follows Income/Expense in one shared cursor space)", m.cursor)
	}
	l, ok := m.cursorLine()
	if !ok || l.Concept.Name != "Sacar la basura" {
		t.Fatalf("cursorLine() = %+v, %v, want the chore", l, ok)
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

	entries, err := catalog.MonthEntries(db, period)
	if err != nil {
		t.Fatalf("MonthEntries() unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].ConceptID != c.ID || !entries[0].Done {
		t.Fatalf("MonthEntries() = %+v, want %s done=true persisted", entries, c.Name)
	}
}

func TestEOnAChoreLineIsANoOp(t *testing.T) {
	db := openTestStore(t)
	seedChore(t, db, "Sacar la basura")
	period := domain.NewPeriod(2026, time.January)

	m := monthModel(t, db, period)

	updated, cmd := m.Update(key("e"))
	m = updated.(Model)
	if m.editing != nil {
		t.Error("e on a Chore line should not open the amount edit — there's no amount to edit")
	}
	if cmd != nil {
		t.Error("e on a Chore line should not write anything")
	}
}

func TestMonthViewHeaderShowsOneConfirmedARSTotal(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40

	rent := catalog.Concept{Name: "Alquiler", Kind: catalog.Expense, Money: &catalog.MoneyDetails{Currency: domain.ARS, Share: domain.NewPercent(50)}}
	salary := catalog.Concept{Name: "Sueldo", Kind: catalog.Income, Money: &catalog.MoneyDetails{Currency: domain.ARS, Share: domain.NewPercent(100)}}
	lines := []month.Line{
		{Concept: salary, Money: &month.LineMoney{Amount: amountFor(t, "1000000"), Confirmed: true}, Done: true},
		{Concept: rent, Money: &month.LineMoney{Amount: amountFor(t, "785000"), Confirmed: false}, Done: false},
	}

	updated, _ := m.Update(monthLoadedMsg{lines: lines})
	m = updated.(Model)
	content := m.View().Content

	for _, want := range []string{"1000000.00", "1 of 2 confirmed"} {
		if !strings.Contains(content, want) {
			t.Errorf("month view content missing %q:\n%s", want, content)
		}
	}
	if strings.Contains(content, "projected  share") {
		t.Errorf("content = %q, want no separate projected total row in the header", content)
	}
}

func TestMonthViewHeaderShowsUSDEquivalentWhenRateKnown(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40

	salary := catalog.Concept{Name: "Sueldo", Kind: catalog.Income, Money: &catalog.MoneyDetails{Currency: domain.ARS, Share: domain.NewPercent(100)}}
	updated, _ := m.Update(monthLoadedMsg{lines: []month.Line{
		{Concept: salary, Money: &month.LineMoney{Amount: amountFor(t, "1000000"), Confirmed: true}, Done: true},
	}})
	m = updated.(Model)
	updated, _ = m.Update(allocationsLoadedMsg{rates: []catalog.FxRate{{Period: m.period, Value: amountFor(t, "1000")}}})
	m = updated.(Model)

	content := m.View().Content
	if !strings.Contains(content, "1000.00 USD") {
		t.Errorf("month view content missing the USD equivalent:\n%s", content)
	}
}

func TestMonthViewHeaderFoldsUSDLineIntoTheARSTotal(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40

	allowance := catalog.Concept{Name: "Family", Kind: catalog.Income, Money: &catalog.MoneyDetails{Currency: domain.USD, Share: domain.NewPercent(100)}}
	updated, _ := m.Update(monthLoadedMsg{lines: []month.Line{
		{Concept: allowance, Money: &month.LineMoney{Amount: amountFor(t, "450"), Confirmed: true}, Done: true},
	}})
	m = updated.(Model)
	updated, _ = m.Update(allocationsLoadedMsg{rates: []catalog.FxRate{{Period: m.period, Value: amountFor(t, "1000")}}})
	m = updated.(Model)

	content := m.View().Content
	if !strings.Contains(content, "450000.00") {
		t.Errorf("month view content = %q, want the USD line folded into the ARS total at rate (450 * 1000)", content)
	}
}

func TestMonthViewHeaderShowsChoresDoneCount(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40

	done := catalog.Concept{Name: "Sacar la basura", Kind: catalog.Chore}
	pending := catalog.Concept{Name: "Regar plantas", Kind: catalog.Chore}
	updated, _ := m.Update(monthLoadedMsg{lines: []month.Line{
		{Concept: done, Done: true},
		{Concept: pending, Done: false},
	}})
	m = updated.(Model)

	content := m.View().Content
	if !strings.Contains(content, "1 of 2 chores done") {
		t.Errorf("month view content missing the chores-done count:\n%s", content)
	}
}

func TestMonthViewHeaderOmitsChoresDoneCountWithNoChores(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40

	rent := catalog.Concept{Name: "Alquiler", Kind: catalog.Expense, Money: &catalog.MoneyDetails{Currency: domain.ARS}}
	updated, _ := m.Update(monthLoadedMsg{lines: []month.Line{
		{Concept: rent, Money: &month.LineMoney{Amount: amountFor(t, "785000")}},
	}})
	m = updated.(Model)

	content := m.View().Content
	if strings.Contains(content, "chores done") {
		t.Errorf("content = %q, want no chores-done count when the month has no chores", content)
	}
}

func TestMonthViewRendersChoreGroupAlongsideIncomeAndExpense(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40

	rent := catalog.Concept{Name: "Alquiler", Kind: catalog.Expense, Money: &catalog.MoneyDetails{Currency: domain.ARS}}
	trash := catalog.Concept{Name: "Sacar la basura", Kind: catalog.Chore}
	updated, _ := m.Update(monthLoadedMsg{
		lines: []month.Line{
			{Concept: rent, Money: &month.LineMoney{Amount: amountFor(t, "785000"), Confirmed: false}, Done: false},
			{Concept: trash, Done: true},
		},
	})
	m = updated.(Model)
	content := m.View().Content

	for _, want := range []string{"Expense", "Alquiler", "Chore", "Sacar la basura"} {
		if !strings.Contains(content, want) {
			t.Errorf("month view content missing %q:\n%s", want, content)
		}
	}
}

func TestMonthViewShowsAssignedListsWithProgress(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40
	m.period = domain.NewPeriod(2026, time.September)

	updated, _ := m.Update(listsLoadedMsg{lists: []catalog.List{
		{Name: "Venezuela trip", Period: domain.NewPeriod(2026, time.September), BodyMD: "- [x] flights\n- [ ] hotel"},
		{Name: "Someday list", Period: domain.NewPeriod(2026, time.July)},
	}})
	m = updated.(Model)
	content := m.View().Content

	if !strings.Contains(content, "Venezuela trip") || !strings.Contains(content, "1/2") {
		t.Errorf("content missing the assigned list and its progress:\n%s", content)
	}
	if strings.Contains(content, "Someday list") {
		t.Errorf("content = %q, want only this period's lists, not other months'", content)
	}
}

func TestMonthViewShowsEditBoxForFocusedLine(t *testing.T) {
	db := openTestStore(t)
	seedConcept(t, db, "Alquiler", 785000)
	period := domain.NewPeriod(2026, time.January)

	m := monthModel(t, db, period)

	updated, _ := m.Update(key("e"))
	m = updated.(Model)

	content := m.View().Content
	if !strings.Contains(content, "785000.00") {
		t.Errorf("month view content = %q, want the prefilled edit box visible", content)
	}
}
