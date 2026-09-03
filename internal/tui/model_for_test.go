package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
	"github.com/gabokatta/mess/internal/rates"
)

// defaultQuotes is what modelFor feeds every model that doesn't care about
// live rates. A test that does sends its own quotesMsg.
var defaultQuotes = []rates.Quote{
	{House: domain.Blue, Buy: decimal.NewFromInt(1520), Sell: decimal.NewFromInt(1540)},
	{House: domain.Official, Buy: decimal.NewFromInt(1485), Sell: decimal.NewFromInt(1535)},
	{House: domain.MEP, Buy: decimal.NewFromInt(1532), Sell: decimal.NewFromInt(1535)},
}

// modelFor loads world into a fresh database and runs the app's own load
// commands against it, in the dependency order Init runs them: rates and
// quotes first, since loadYear reads m.fx(). A test states its terminal
// size, which the layout tests need anyway.
func modelFor(t *testing.T, world fixture.World, width, height int) Model {
	t.Helper()
	db := fixture.DB(t)
	fixture.MustLoad(t, db, world)

	m := New(db)
	m.today = fixture.Period
	m.period = fixture.Period

	m, _ = send(t, m, tea.WindowSizeMsg{Width: width, Height: height})
	m, _ = send(t, m, runCmd(t, loadRates(db)), quotesMsg{quotes: defaultQuotes})
	m, _ = send(t, m,
		runCmd(t, loadMonth(db, m.period)),
		runCmd(t, loadCatalog(db)),
		runCmd(t, loadNotes(db)),
		runCmd(t, loadYear(db, m.period.Year(), m.fx())),
	)
	return m
}

// runCmd runs a load command synchronously — every one of them is a plain
// database read with nothing to wait on.
func runCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}
	return cmd()
}
