package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
	"github.com/gabokatta/mess/internal/rates"
)

// quotedOn is a weekday inside fixture.Period, since a quote always carries
// the day the market was open on.
var quotedOn = time.Date(fixture.Year, time.September, 2, 0, 0, 0, 0, time.UTC)

var defaultQuotes = []rates.Quote{
	{House: domain.Blue, Buy: decimal.NewFromInt(1520), Sell: decimal.NewFromInt(1540), Date: quotedOn},
	{House: domain.Official, Buy: decimal.NewFromInt(1485), Sell: decimal.NewFromInt(1535), Date: quotedOn},
	{House: domain.MEP, Buy: decimal.NewFromInt(1532), Sell: decimal.NewFromInt(1535), Date: quotedOn},
}

// modelFor runs the app's own load commands in Init's dependency order: rates
// and quotes first, since loadYear reads m.fx().
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

func runCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}
	return cmd()
}
