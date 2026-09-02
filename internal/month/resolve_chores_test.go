package month

import (
	"testing"
	"time"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func chore(id int64, mask domain.Cadence, from, until domain.Period) catalog.Chore {
	return catalog.Chore{ID: id, Name: "Sacar la basura", MonthMask: mask, ActiveFrom: from, ActiveUntil: until}
}

func TestResolveChoresIncludesActiveChoreWithDoneState(t *testing.T) {
	sept := domain.NewPeriod(2026, time.September)
	c := chore(1, domain.Monthly, domain.NewPeriod(2026, time.January), domain.Period{})
	entries := map[int64]catalog.ChoreEntry{
		1: {ChoreID: 1, Period: sept, Done: true},
	}

	lines := ResolveChores(sept, []catalog.Chore{c}, entries)

	if len(lines) != 1 {
		t.Fatalf("ResolveChores() = %d lines, want 1", len(lines))
	}
	if !lines[0].Done {
		t.Error("Done = false, want true")
	}
}

func TestResolveChoresDefaultsToNotDoneWithoutEntry(t *testing.T) {
	sept := domain.NewPeriod(2026, time.September)
	c := chore(1, domain.Monthly, domain.NewPeriod(2026, time.January), domain.Period{})

	lines := ResolveChores(sept, []catalog.Chore{c}, nil)

	if len(lines) != 1 {
		t.Fatalf("ResolveChores() = %d lines, want 1", len(lines))
	}
	if lines[0].Done {
		t.Error("Done = true, want false (no entry for this period)")
	}
}

func TestResolveChoresExcludesChoreOutsideMonthMask(t *testing.T) {
	sept := domain.NewPeriod(2026, time.September)
	c := chore(1, domain.Aguinaldo, domain.NewPeriod(2026, time.January), domain.Period{})

	lines := ResolveChores(sept, []catalog.Chore{c}, nil)

	if len(lines) != 0 {
		t.Errorf("ResolveChores() = %d lines, want 0 (September is not June/December)", len(lines))
	}
}

func TestUnfinishedChoresCountsOnlyNotDone(t *testing.T) {
	lines := []ChoreLine{
		{Chore: chore(1, domain.Monthly, domain.Period{}, domain.Period{}), Done: true},
		{Chore: chore(2, domain.Monthly, domain.Period{}, domain.Period{}), Done: false},
		{Chore: chore(3, domain.Monthly, domain.Period{}, domain.Period{}), Done: false},
	}

	if got := UnfinishedChores(lines); got != 2 {
		t.Errorf("UnfinishedChores() = %d, want 2", got)
	}
}

func TestUnfinishedChoresZeroWhenAllDone(t *testing.T) {
	lines := []ChoreLine{
		{Chore: chore(1, domain.Monthly, domain.Period{}, domain.Period{}), Done: true},
	}

	if got := UnfinishedChores(lines); got != 0 {
		t.Errorf("UnfinishedChores() = %d, want 0", got)
	}
}

func TestResolveChoresExcludesChoreOutsideActiveRange(t *testing.T) {
	sept := domain.NewPeriod(2026, time.September)
	c := chore(1, domain.Monthly, domain.NewPeriod(2026, time.January), domain.NewPeriod(2026, time.August))

	lines := ResolveChores(sept, []catalog.Chore{c}, nil)

	if len(lines) != 0 {
		t.Errorf("ResolveChores() = %d lines, want 0 (chore retired in August)", len(lines))
	}
}
