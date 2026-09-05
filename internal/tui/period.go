package tui

import (
	"fmt"
	"strings"

	"github.com/gabokatta/mess/internal/domain"
)

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

func (m Model) periodHeading() string {
	title := m.theme.Title.Underline(true).
		Render(strings.ToUpper(m.period.Month().String()) + " " + fmt.Sprint(m.period.Year()))
	return title + "  " + m.theme.Muted.Render(periodStatus(m.period, m.today))
}
