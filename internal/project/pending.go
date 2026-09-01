package project

import (
	"sort"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

// Pending is projects' open ones (no closed_at), ordered overdue first,
// then the current period, then unassigned, then future-dated — the order
// the Pending tab renders in. Within a bucket, the oldest period leads.
func Pending(projects []catalog.Project, current domain.Period) []catalog.Project {
	var open []catalog.Project
	for _, p := range projects {
		if p.ClosedAt == nil {
			open = append(open, p)
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

// Closed is a project's closed ones, ordered by name.
func Closed(projects []catalog.Project) []catalog.Project {
	var closed []catalog.Project
	for _, p := range projects {
		if p.ClosedAt != nil {
			closed = append(closed, p)
		}
	}
	sort.Slice(closed, func(i, j int) bool { return closed[i].Name < closed[j].Name })
	return closed
}

func pendingRank(p catalog.Project, current domain.Period) int {
	switch {
	case p.Period.IsZero():
		return 2
	case p.Period.Before(current):
		return 0
	case p.Period.Equal(current):
		return 1
	default:
		return 3
	}
}
