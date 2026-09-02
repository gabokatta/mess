package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
)

func TestMonthViewRendersLastMonthUnfinishedCount(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40
	trash := catalog.Chore{Name: "Sacar la basura"}
	updated, _ := m.Update(monthLoadedMsg{chores: []month.ChoreLine{{Chore: trash, Done: true}}})
	m = updated.(Model)
	updated, _ = m.Update(lastMonthChoresLoadedMsg{unfinished: 3})
	m = updated.(Model)

	content := m.View().Content
	if !strings.Contains(content, "Last month: 3 unfinished") {
		t.Errorf("content = %q, want the unfinished-chores line", content)
	}
}

func TestLoadLastMonthChoresCountsUndoneFromThePriorPeriod(t *testing.T) {
	db := openTestStore(t)
	prior := domain.NewPeriod(2026, time.August)
	current := domain.NewPeriod(2026, time.September)
	a := seedChore(t, db, "Sacar la basura")
	seedChore(t, db, "Regar las plantas")
	if err := catalog.SetChoreEntryDone(db, a.ID, prior, true); err != nil {
		t.Fatalf("SetChoreEntryDone() unexpected error: %v", err)
	}

	cmd := loadLastMonthChores(db, current)
	msg := cmd().(lastMonthChoresLoadedMsg)
	if msg.err != nil {
		t.Fatalf("loadLastMonthChores() unexpected error: %v", msg.err)
	}
	if msg.unfinished != 1 {
		t.Errorf("unfinished = %d, want 1 (one of two chores done in August)", msg.unfinished)
	}
}
