package tui

import (
	"database/sql"

	tea "charm.land/bubbletea/v2"

	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
)

// lastMonthChoresLoadedMsg is the result of loadLastMonthChores' Cmd,
// delivered back to Update once the database read completes.
type lastMonthChoresLoadedMsg struct {
	unfinished int
	err        error
}

// loadLastMonthChores resolves the previous period's chores so the month
// view can surface "Last month: N unfinished" — visible without becoming a
// backlog, since October generates its own chores regardless.
func loadLastMonthChores(db *sql.DB, period domain.Period) tea.Cmd {
	return func() tea.Msg {
		loaded, err := month.Load(db, period.AddMonths(-1))
		if err != nil {
			return lastMonthChoresLoadedMsg{err: err}
		}
		return lastMonthChoresLoadedMsg{unfinished: month.UnfinishedChores(loaded.Chores)}
	}
}
