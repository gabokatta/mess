package tui

import (
	"context"
	"database/sql"

	tea "charm.land/bubbletea/v2"

	"github.com/gabokatta/mes/internal/catalog"
	"github.com/gabokatta/mes/internal/dolarapi"
	"github.com/gabokatta/mes/internal/domain"
)

// fxFilledMsg is the result of fillCurrentFxRate's Cmd.
type fxFilledMsg struct {
	err error
}

// fillCurrentFxRate fetches today's quote for the configured fx house and
// fills period's rate if it's still empty. Runs once, on app open.
func fillCurrentFxRate(db *sql.DB, client *dolarapi.Client, period domain.Period) tea.Cmd {
	return func() tea.Msg {
		house, err := catalog.FxHouse(db)
		if err != nil {
			return fxFilledMsg{err: err}
		}
		value, err := client.Quote(context.Background(), house)
		if err != nil {
			return fxFilledMsg{err: err}
		}
		return fxFilledMsg{err: catalog.FillFetchedFxRate(db, period, value)}
	}
}
