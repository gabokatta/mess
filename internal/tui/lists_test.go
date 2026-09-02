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

func seedList(t *testing.T, db *sql.DB, p catalog.List) catalog.List {
	t.Helper()
	created, err := catalog.CreateList(db, p)
	if err != nil {
		t.Fatalf("CreateList() unexpected error: %v", err)
	}
	return created
}

func listsModel(t *testing.T, db *sql.DB, period domain.Period) Model {
	t.Helper()
	m := New(db)
	m.width, m.height = 100, 40
	m.period = period
	m.view = viewLists
	lists, err := catalog.Lists(db)
	if err != nil {
		t.Fatalf("catalog.Lists() unexpected error: %v", err)
	}
	updated, _ := m.Update(listsLoadedMsg{lists: lists})
	return updated.(Model)
}

func TestListsViewRendersPendingOrderedAndWithProgress(t *testing.T) {
	db := openTestStore(t)
	current := domain.NewPeriod(2026, time.September)
	seedList(t, db, catalog.List{
		Name:   "Itinerary",
		BodyMD: "## ARG\n- [x] book flights\n- [ ] renew passport",
		Period: current,
	})
	seedList(t, db, catalog.List{
		Name:   "Buy list",
		Period: domain.NewPeriod(2026, time.July),
		BodyMD: "- [ ] paint",
	})

	m := listsModel(t, db, current)
	content := m.View().Content

	buyIdx := strings.Index(content, "Buy list")
	itinIdx := strings.Index(content, "Itinerary")
	if buyIdx == -1 || itinIdx == -1 || buyIdx > itinIdx {
		t.Errorf("content = %q, want overdue Buy list before the current-month Itinerary", content)
	}
	if !strings.Contains(content, "1/2") {
		t.Errorf("content missing Itinerary's 1/2 progress:\n%s", content)
	}
	if !strings.Contains(content, "overdue") {
		t.Errorf("content missing the overdue badge for Buy list:\n%s", content)
	}
}

func TestListsCursorMovesWithJKAndClamps(t *testing.T) {
	db := openTestStore(t)
	seedList(t, db, catalog.List{Name: "Buy list", BodyMD: "- [ ] milk\n- [ ] eggs"})
	current := domain.NewPeriod(2026, time.September)

	m := listsModel(t, db, current)
	if m.listCursor != 0 {
		t.Fatalf("listCursor = %d, want 0 at load", m.listCursor)
	}

	updated, _ := m.Update(key("j"))
	m = updated.(Model)
	if m.listCursor != 1 {
		t.Fatalf("listCursor = %d, want 1 after j", m.listCursor)
	}

	updated, _ = m.Update(key("j"))
	m = updated.(Model)
	if m.listCursor != 1 {
		t.Fatalf("listCursor = %d, want clamped at 1 (last checkbox)", m.listCursor)
	}

	updated, _ = m.Update(key("k"))
	m = updated.(Model)
	if m.listCursor != 0 {
		t.Fatalf("listCursor = %d, want 0 after k", m.listCursor)
	}
}

func TestSpaceTogglesCheckboxUnderListCursor(t *testing.T) {
	db := openTestStore(t)
	seedList(t, db, catalog.List{Name: "Buy list", BodyMD: "- [ ] milk\n- [ ] eggs"})
	current := domain.NewPeriod(2026, time.September)

	m := listsModel(t, db, current)
	updated, _ := m.Update(key("j"))
	m = updated.(Model)

	updated, cmd := m.Update(keySpace())
	m = updated.(Model)
	m = settle(t, m, cmd)

	lists, err := catalog.Lists(db)
	if err != nil {
		t.Fatalf("Lists() unexpected error: %v", err)
	}
	if len(lists) != 1 || lists[0].BodyMD != "- [ ] milk\n- [x] eggs" {
		t.Fatalf("Lists() = %+v, want only eggs ticked", lists)
	}
}

func TestEKeyOpensTextareaAndCtrlSCommits(t *testing.T) {
	db := openTestStore(t)
	p := seedList(t, db, catalog.List{Name: "Buy list", BodyMD: "- [ ] milk"})
	current := domain.NewPeriod(2026, time.September)

	m := listsModel(t, db, current)
	updated, _ := m.Update(key("e"))
	m = updated.(Model)
	if m.listEditing == nil || m.listEditing.listID != p.ID {
		t.Fatalf("listEditing = %+v, want editing opened for %d", m.listEditing, p.ID)
	}
	if got := m.listEditing.textarea.Value(); got != "- [ ] milk" {
		t.Errorf("textarea value = %q, want prefilled with the body", got)
	}

	m.listEditing.textarea.SetValue("- [ ] milk\n- [ ] bread")
	updated, cmd := m.Update(keyCtrlS())
	m = updated.(Model)
	if m.listEditing != nil {
		t.Fatal("ctrl+s should close the edit")
	}
	m = settle(t, m, cmd)

	lists, err := catalog.Lists(db)
	if err != nil {
		t.Fatalf("Lists() unexpected error: %v", err)
	}
	if len(lists) != 1 || lists[0].BodyMD != "- [ ] milk\n- [ ] bread" {
		t.Fatalf("Lists() = %+v, want the edited body persisted", lists)
	}
}

func TestEscCancelsListEditWithoutWriting(t *testing.T) {
	db := openTestStore(t)
	seedList(t, db, catalog.List{Name: "Buy list", BodyMD: "- [ ] milk"})
	current := domain.NewPeriod(2026, time.September)

	m := listsModel(t, db, current)
	updated, _ := m.Update(key("e"))
	m = updated.(Model)
	m.listEditing.textarea.SetValue("wiped out")

	updated, cmd := m.Update(keyEsc())
	m = updated.(Model)
	if m.listEditing != nil {
		t.Error("esc should close the edit")
	}
	if cmd != nil {
		t.Error("esc should not write anything")
	}

	lists, err := catalog.Lists(db)
	if err != nil {
		t.Fatalf("Lists() unexpected error: %v", err)
	}
	if lists[0].BodyMD != "- [ ] milk" {
		t.Errorf("Lists()[0].BodyMD = %q, want unchanged", lists[0].BodyMD)
	}
}

func TestCKeyClosesAndReopensList(t *testing.T) {
	db := openTestStore(t)
	seedList(t, db, catalog.List{Name: "Buy list", BodyMD: "- [ ] milk"})
	current := domain.NewPeriod(2026, time.September)

	m := listsModel(t, db, current)
	updated, cmd := m.Update(key("c"))
	m = updated.(Model)
	m = settle(t, m, cmd)

	lists, err := catalog.Lists(db)
	if err != nil {
		t.Fatalf("Lists() unexpected error: %v", err)
	}
	if lists[0].ClosedAt == nil {
		t.Fatal("Lists()[0].ClosedAt = nil, want it closed")
	}

	m.showClosed = true
	m.listCursor = 0
	updated, cmd = m.Update(key("c"))
	m = updated.(Model)
	m = settle(t, m, cmd)

	lists, err = catalog.Lists(db)
	if err != nil {
		t.Fatalf("Lists() unexpected error: %v", err)
	}
	if lists[0].ClosedAt != nil {
		t.Fatal("Lists()[0].ClosedAt != nil, want it reopened")
	}
}

func TestClosingListResetsCursor(t *testing.T) {
	db := openTestStore(t)
	seedList(t, db, catalog.List{Name: "A", BodyMD: "- [ ] a"})
	seedList(t, db, catalog.List{Name: "B", BodyMD: "- [ ] b"})
	current := domain.NewPeriod(2026, time.September)

	m := listsModel(t, db, current)
	updated, _ := m.Update(key("j"))
	m = updated.(Model)
	if m.listCursor != 1 {
		t.Fatalf("listCursor = %d, want 1 before closing", m.listCursor)
	}

	updated, _ = m.Update(key("c"))
	m = updated.(Model)
	if m.listCursor != 0 {
		t.Errorf("listCursor = %d, want reset to 0 after closing", m.listCursor)
	}
}

func TestNKeyOpensNewListFormAndRendersIt(t *testing.T) {
	db := openTestStore(t)
	current := domain.NewPeriod(2026, time.September)
	m := listsModel(t, db, current)

	updated, cmd := m.Update(key("n"))
	m = updated.(Model)
	if m.newList == nil {
		t.Fatal("newList = nil, want a name prompt opened")
	}
	m = settle(t, m, cmd)

	content := m.View().Content
	if !strings.Contains(content, "List name") {
		t.Errorf("content = %q, want the List name field", content)
	}
}

func TestCompletingNewListFormCreatesUnassignedBodylessList(t *testing.T) {
	db := openTestStore(t)
	current := domain.NewPeriod(2026, time.September)
	m := listsModel(t, db, current)

	updated, _ := m.Update(key("n"))
	m = updated.(Model)
	m.newList.values.name = "Buy list"
	m.newList.form.State = huh.StateCompleted

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	if m.newList != nil {
		t.Fatal("a completed form should close")
	}
	m = settle(t, m, cmd)

	lists, err := catalog.Lists(db)
	if err != nil {
		t.Fatalf("catalog.Lists() unexpected error: %v", err)
	}
	if len(lists) != 1 || lists[0].Name != "Buy list" || lists[0].BodyMD != "" || !lists[0].Period.IsZero() {
		t.Errorf("Lists() = %+v, want a single unassigned, bodyless Buy list", lists)
	}
}

func TestCompletingNewListFormWithAPeriodAssignsIt(t *testing.T) {
	db := openTestStore(t)
	current := domain.NewPeriod(2026, time.September)
	m := listsModel(t, db, current)

	updated, _ := m.Update(key("n"))
	m = updated.(Model)
	m.newList.values.name = "Venezuela trip"
	m.newList.values.period = "2026-09"
	m.newList.form.State = huh.StateCompleted

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	m = settle(t, m, cmd)

	lists, err := catalog.Lists(db)
	if err != nil {
		t.Fatalf("catalog.Lists() unexpected error: %v", err)
	}
	if len(lists) != 1 || !lists[0].Period.Equal(current) {
		t.Errorf("Lists() = %+v, want assigned to 2026-09", lists)
	}
}

func TestPKeyOpensPeriodAssignFormPrefilledWithTheCurrentAssignment(t *testing.T) {
	db := openTestStore(t)
	current := domain.NewPeriod(2026, time.September)
	seedList(t, db, catalog.List{Name: "Buy list", Period: current})
	m := listsModel(t, db, current)

	updated, cmd := m.Update(key("p"))
	m = updated.(Model)
	if m.periodAssignForm == nil {
		t.Fatal("periodAssignForm = nil, want a form opened")
	}
	if m.periodAssignForm.values.period != "2026-09" {
		t.Errorf("values.period = %q, want prefilled with the current assignment", m.periodAssignForm.values.period)
	}
	m = settle(t, m, cmd)

	content := m.View().Content
	if !strings.Contains(content, "Assign period") {
		t.Errorf("content = %q, want the form's title", content)
	}
}

func TestCompletingPeriodAssignFormReassignsTheList(t *testing.T) {
	db := openTestStore(t)
	current := domain.NewPeriod(2026, time.September)
	p := seedList(t, db, catalog.List{Name: "Buy list"})
	m := listsModel(t, db, current)

	updated, _ := m.Update(key("p"))
	m = updated.(Model)
	m.periodAssignForm.values.period = "2026-10"
	m.periodAssignForm.form.State = huh.StateCompleted

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	if m.periodAssignForm != nil {
		t.Fatal("a completed form should close")
	}
	m = settle(t, m, cmd)

	lists, err := catalog.Lists(db)
	if err != nil {
		t.Fatalf("catalog.Lists() unexpected error: %v", err)
	}
	if len(lists) != 1 || lists[0].ID != p.ID || !lists[0].Period.Equal(domain.NewPeriod(2026, time.October)) {
		t.Fatalf("Lists() = %+v, want reassigned to 2026-10", lists)
	}
}

func TestClearingPeriodAssignFormUnassignsTheList(t *testing.T) {
	db := openTestStore(t)
	current := domain.NewPeriod(2026, time.September)
	seedList(t, db, catalog.List{Name: "Buy list", Period: current})
	m := listsModel(t, db, current)

	updated, _ := m.Update(key("p"))
	m = updated.(Model)
	m.periodAssignForm.values.period = ""
	m.periodAssignForm.form.State = huh.StateCompleted

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	m = settle(t, m, cmd)

	lists, err := catalog.Lists(db)
	if err != nil {
		t.Fatalf("catalog.Lists() unexpected error: %v", err)
	}
	if len(lists) != 1 || !lists[0].Period.IsZero() {
		t.Fatalf("Lists() = %+v, want unassigned", lists)
	}
}

func TestEscCancelsPeriodAssignFormWithoutWriting(t *testing.T) {
	db := openTestStore(t)
	current := domain.NewPeriod(2026, time.September)
	seedList(t, db, catalog.List{Name: "Buy list", Period: current})
	m := listsModel(t, db, current)

	updated, _ := m.Update(key("p"))
	m = updated.(Model)
	m.periodAssignForm.values.period = "2026-01"

	updated, cmd := m.Update(keyEsc())
	m = updated.(Model)
	if m.periodAssignForm != nil {
		t.Error("esc should close the form")
	}
	if cmd != nil {
		t.Error("esc should not write anything")
	}

	lists, err := catalog.Lists(db)
	if err != nil {
		t.Fatalf("catalog.Lists() unexpected error: %v", err)
	}
	if !lists[0].Period.Equal(current) {
		t.Errorf("Lists()[0].Period = %v, want unchanged", lists[0].Period)
	}
}

func TestEnterWithBlankNameKeepsFormOpen(t *testing.T) {
	db := openTestStore(t)
	current := domain.NewPeriod(2026, time.September)
	m := listsModel(t, db, current)

	updated, _ := m.Update(key("n"))
	m = updated.(Model)
	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	if m.newList == nil {
		t.Fatal("enter with a blank required name should keep the form open")
	}
	m = settle(t, m, cmd)

	lists, err := catalog.Lists(db)
	if err != nil {
		t.Fatalf("catalog.Lists() unexpected error: %v", err)
	}
	if len(lists) != 0 {
		t.Errorf("Lists() = %+v, want none created", lists)
	}
}

func TestEscCancelsNewListFormWithoutWriting(t *testing.T) {
	db := openTestStore(t)
	current := domain.NewPeriod(2026, time.September)
	m := listsModel(t, db, current)

	updated, _ := m.Update(key("n"))
	m = updated.(Model)
	m.newList.values.name = "Buy list"
	updated, cmd := m.Update(keyEsc())
	m = updated.(Model)
	if m.newList != nil {
		t.Error("esc should close the form")
	}
	if cmd != nil {
		t.Error("esc should not write anything")
	}

	lists, err := catalog.Lists(db)
	if err != nil {
		t.Fatalf("catalog.Lists() unexpected error: %v", err)
	}
	if len(lists) != 0 {
		t.Errorf("Lists() = %+v, want none created", lists)
	}
}

func TestFKeyTogglesClosedFilterAndResetsCursor(t *testing.T) {
	db := openTestStore(t)
	p := seedList(t, db, catalog.List{Name: "Buy list", BodyMD: "- [ ] milk"})
	current := domain.NewPeriod(2026, time.September)
	closedAt := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	if err := catalog.SetListClosed(db, p.ID, &closedAt); err != nil {
		t.Fatalf("SetListClosed() unexpected error: %v", err)
	}

	m := listsModel(t, db, current)
	content := m.View().Content
	if !strings.Contains(content, "no open lists") {
		t.Fatalf("content = %q, want the empty pending state", content)
	}

	updated, _ := m.Update(key("f"))
	m = updated.(Model)
	if !m.showClosed {
		t.Fatal("showClosed = false, want true after f")
	}
	content = m.View().Content
	if !strings.Contains(content, "Buy list") {
		t.Errorf("content = %q, want the closed Buy list visible", content)
	}
}
