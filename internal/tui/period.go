package tui

import "github.com/gabokatta/mess/internal/domain"

// periodStatus is where a period sits relative to the month still running.
// Past months stay freely editable and render muted; there is no "close
// month" action.
func periodStatus(p, today domain.Period) string {
	switch {
	case p.Before(today):
		return "past"
	case p.After(today):
		return "future"
	default:
		return "current"
	}
}
