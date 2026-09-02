package list

import (
	"testing"
	"time"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func TestPendingOrdersOverdueThenCurrentThenUnassigned(t *testing.T) {
	current := domain.NewPeriod(2026, time.September)
	overdue := catalog.List{Name: "Buy list", Period: domain.NewPeriod(2026, time.July)}
	ongoing := catalog.List{Name: "Itinerary", Period: current}
	unassigned := catalog.List{Name: "Someday", Period: domain.Period{}}
	closed := catalog.List{Name: "Done already", Period: overdue.Period, ClosedAt: closedNow()}

	got := Pending([]catalog.List{unassigned, closed, ongoing, overdue}, current)

	want := []string{"Buy list", "Itinerary", "Someday"}
	if len(got) != len(want) {
		t.Fatalf("Pending() = %+v, want %v", got, want)
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("Pending()[%d].Name = %q, want %q", i, got[i].Name, name)
		}
	}
}

func TestPendingOrdersOldestOverdueFirst(t *testing.T) {
	current := domain.NewPeriod(2026, time.September)
	july := catalog.List{Name: "July", Period: domain.NewPeriod(2026, time.July)}
	august := catalog.List{Name: "August", Period: domain.NewPeriod(2026, time.August)}

	got := Pending([]catalog.List{august, july}, current)

	if len(got) != 2 || got[0].Name != "July" || got[1].Name != "August" {
		t.Errorf("Pending() = %+v, want July before August", got)
	}
}

func TestClosedReturnsOnlyClosedLists(t *testing.T) {
	open := catalog.List{Name: "Open"}
	closed := catalog.List{Name: "Closed", ClosedAt: closedNow()}

	got := Closed([]catalog.List{open, closed})

	if len(got) != 1 || got[0].Name != "Closed" {
		t.Errorf("Closed() = %+v, want only Closed", got)
	}
}

func closedNow() *time.Time {
	t := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	return &t
}
