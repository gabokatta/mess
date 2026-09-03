package tui

import "github.com/gabokatta/mess/internal/domain"

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
