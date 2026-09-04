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

// periodHeading is a period screen's one heading: the month it shows and where
// that sits relative to today. Underlined, so it reads as the page's title
// rather than as another structural label like the group headers below it,
// which share its bold weight but not its underline.
func (m Model) periodHeading() string {
	title := m.theme.Title.Underline(true).
		Render(strings.ToUpper(m.period.Month().String()) + " " + fmt.Sprint(m.period.Year()))
	return title + "  " + m.theme.Muted.Render(periodStatus(m.period, m.today))
}
