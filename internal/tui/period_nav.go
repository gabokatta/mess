package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/gabokatta/mess/internal/domain"
)

// shiftPeriod moves the shown period by delta months and reloads everything
// that reads off it — both panes' cursors reset since the rows they pointed
// at belong to the period being left.
func (m Model) shiftPeriod(delta int) (Model, tea.Cmd) {
	m.period = m.period.AddMonths(delta)
	m.financeCursor = 0
	m.choreCursor = 0
	return m, tea.Batch(
		loadMonth(m.db, m.period),
		loadAllocations(m.db, m.period),
		loadLastMonthChores(m.db, m.period),
		loadYear(m.db, m.period.Year()),
	)
}

// periodStatus is where a period sits relative to today — past months stay
// freely editable but render muted, there is no "close month" action.
type periodStatus int

const (
	periodCurrent periodStatus = iota
	periodPast
	periodFuture
)

func (s periodStatus) String() string {
	switch s {
	case periodPast:
		return "past"
	case periodFuture:
		return "future"
	default:
		return "current"
	}
}

// resolvePeriodStatus compares p against today, in months — a same-month
// comparison regardless of the day today falls on.
func resolvePeriodStatus(p, today domain.Period) periodStatus {
	switch {
	case p.Before(today):
		return periodPast
	case p.After(today):
		return periodFuture
	default:
		return periodCurrent
	}
}

func currentPeriodStatus(p domain.Period) periodStatus {
	return resolvePeriodStatus(p, domain.PeriodFromTime(time.Now()))
}
