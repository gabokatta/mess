package list

import (
	"sort"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

// Pending is lists' open ones (no closed_at), ordered overdue first,
// then the current period, then unassigned, then future-dated — the order
// the Pending tab renders in. Within a bucket, the oldest period leads.
func Pending(lists []catalog.List, current domain.Period) []catalog.List {
	var open []catalog.List
	for _, l := range lists {
		if l.ClosedAt == nil {
			open = append(open, l)
		}
	}
	sort.Slice(open, func(i, j int) bool {
		a, b := open[i], open[j]
		ra, rb := pendingRank(a, current), pendingRank(b, current)
		if ra != rb {
			return ra < rb
		}
		if a.Period != b.Period {
			return a.Period.Before(b.Period)
		}
		return a.Name < b.Name
	})
	return open
}

// Closed is a list's closed ones, ordered by name.
func Closed(lists []catalog.List) []catalog.List {
	var closed []catalog.List
	for _, l := range lists {
		if l.ClosedAt != nil {
			closed = append(closed, l)
		}
	}
	sort.Slice(closed, func(i, j int) bool { return closed[i].Name < closed[j].Name })
	return closed
}

func pendingRank(l catalog.List, current domain.Period) int {
	switch {
	case l.Period.IsZero():
		return 2
	case l.Period.Before(current):
		return 0
	case l.Period.Equal(current):
		return 1
	default:
		return 3
	}
}
