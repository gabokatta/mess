package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
)

func TestLinesForKindOrdersByDueDayUnsetLast(t *testing.T) {
	lines := []month.Line{
		{Concept: catalog.Concept{Name: "Internet", Kind: catalog.Expense, DueDay: 20}},
		{Concept: catalog.Concept{Name: "Netflix", Kind: catalog.Expense, DueDay: 0}},
		{Concept: catalog.Concept{Name: "Alquiler", Kind: catalog.Expense, DueDay: 5}},
	}

	got := linesForKind(lines, catalog.Expense)
	want := []string{"Alquiler", "Internet", "Netflix"}
	for i, name := range want {
		if got[i].Concept.Name != name {
			t.Errorf("order[%d] = %q, want %q (due day 5, 20, then unset)", i, got[i].Concept.Name, name)
		}
	}
}

func TestIsLateOnlyWithinTheActualCurrentPeriod(t *testing.T) {
	today := time.Now()
	if today.Day() < 2 {
		t.Skip("this test needs a due day that has already passed today")
	}
	past := today.Day() - 1
	current := domain.PeriodFromTime(today)

	late := month.Line{Concept: catalog.Concept{DueDay: past}, Done: false}
	if !isLate(late, current) {
		t.Error("isLate() = false, want true (due day passed, unticked, current period)")
	}

	done := month.Line{Concept: catalog.Concept{DueDay: past}, Done: true}
	if isLate(done, current) {
		t.Error("isLate() = true, want false (already done)")
	}

	noDueDay := month.Line{Concept: catalog.Concept{DueDay: 0}, Done: false}
	if isLate(noDueDay, current) {
		t.Error("isLate() = true, want false (no due day set)")
	}

	pastPeriod := current.AddMonths(-1)
	stillLate := month.Line{Concept: catalog.Concept{DueDay: past}, Done: false}
	if isLate(stillLate, pastPeriod) {
		t.Error("isLate() = true, want false (a past period is closed, not late)")
	}
}

func TestSortChoresByDueDayOrdersUnsetLast(t *testing.T) {
	chores := []month.ChoreLine{
		{Chore: catalog.Chore{Name: "Basura", DueDay: 20}},
		{Chore: catalog.Chore{Name: "Plantas", DueDay: 0}},
		{Chore: catalog.Chore{Name: "Filtro", DueDay: 5}},
	}

	got := sortChoresByDueDay(chores)
	want := []string{"Filtro", "Basura", "Plantas"}
	for i, name := range want {
		if got[i].Chore.Name != name {
			t.Errorf("order[%d] = %q, want %q (due day 5, 20, then unset)", i, got[i].Chore.Name, name)
		}
	}
}

func TestIsChoreLateOnlyWithinTheActualCurrentPeriod(t *testing.T) {
	today := time.Now()
	if today.Day() < 2 {
		t.Skip("this test needs a due day that has already passed today")
	}
	past := today.Day() - 1
	current := domain.PeriodFromTime(today)

	late := month.ChoreLine{Chore: catalog.Chore{DueDay: past}, Done: false}
	if !isChoreLate(late, current) {
		t.Error("isChoreLate() = false, want true (due day passed, unticked, current period)")
	}

	done := month.ChoreLine{Chore: catalog.Chore{DueDay: past}, Done: true}
	if isChoreLate(done, current) {
		t.Error("isChoreLate() = true, want false (already done)")
	}

	pastPeriod := current.AddMonths(-1)
	if isChoreLate(late, pastPeriod) {
		t.Error("isChoreLate() = true, want false (a past period is closed, not late)")
	}
}

func TestMonthViewRendersLateMarkerForAnOverdueUnconfirmedLine(t *testing.T) {
	today := time.Now()
	if today.Day() < 2 {
		t.Skip("this test needs a due day that has already passed today")
	}
	m := New(openTestStore(t))
	m.width, m.height = 100, 40
	m.period = domain.PeriodFromTime(today)

	rent := catalog.Concept{Name: "Alquiler", Kind: catalog.Expense, Currency: domain.ARS, DueDay: today.Day() - 1}
	updated, _ := m.Update(monthLoadedMsg{lines: []month.Line{
		{Concept: rent, Amount: amountFor(t, "785000"), Done: false},
	}})
	m = updated.(Model)

	content := m.View().Content
	if !strings.Contains(content, "late") {
		t.Errorf("content = %q, want the late marker for the overdue line", content)
	}
}
