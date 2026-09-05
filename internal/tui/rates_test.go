package tui

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
	"github.com/gabokatta/mess/internal/month"
)

// Three houses do not need a cursor: h walks them and commits, and the arrows
// belong to the table.
func TestRatesHouseKeyCyclesAndCommits(t *testing.T) {
	m := modelFor(t, fixture.World{}, 90, 30)
	m.view = viewRates

	for _, want := range []domain.FxHouse{domain.Official, domain.MEP, domain.Blue} {
		_, cmd := send(t, m, key("h"))
		if err := runWrite(t, cmd); err != nil {
			t.Fatalf("cycling the house reported an error: %v", err)
		}
		got, err := catalog.FxHouse(m.db)
		if err != nil {
			t.Fatalf("FxHouse() unexpected error: %v", err)
		}
		if got != want {
			t.Fatalf("FxHouse() = %v, want %v", got, want)
		}
		m, _ = send(t, m, runCmd(t, loadRates(m.db)))
	}
}

// Adopting a house changes every total that runs through a rate.
func TestFxTableFollowsTheAdoptedHouse(t *testing.T) {
	m := modelFor(t, fixture.World{}, 90, 30)
	m.view = viewRates

	m, _ = send(t, m, ratesMsg{settings: catalog.Settings{FxHouse: domain.Blue}})
	if rate := m.fx().At(september); !rate.Value.Equal(decimal.NewFromInt(1540)) {
		t.Errorf("live rate on Blue = %s, want 1540", rate.Value)
	}

	m, _ = send(t, m, ratesMsg{settings: catalog.Settings{FxHouse: domain.Official}})
	if rate := m.fx().At(september); !rate.Value.Equal(decimal.NewFromInt(1535)) {
		t.Errorf("live rate on Official = %s, want 1535", rate.Value)
	}
}

// e prefills the by-hand editor with the rate currently in effect, for the
// month under the cursor.
func TestManualRateFormOpensOnTheCursorPeriod(t *testing.T) {
	m := modelFor(t, fixture.World{}, 90, 30)
	m.view = viewRates
	m, _ = send(t, m, ratesMsg{settings: catalog.Settings{FxHouse: domain.Blue}})
	m = m.sync()

	m, _ = send(t, m, key("e"))
	if _, ok := m.topModal().(*form); !ok {
		t.Fatalf("modal = %T, want *form", m.topModal())
	}
	if !strings.Contains(m.topModal().View(), september.String()) {
		t.Errorf("form view does not name the shown period:\n%s", m.topModal().View())
	}
}

// A manual rate beats the live quote and, conversion being read-time,
// cascades through that month's totals.
func TestManualRateOverridesTheLiveQuote(t *testing.T) {
	m := modelFor(t, fixture.World{}, 90, 30)
	m.view = viewRates
	if err := catalog.SetManualFxRate(m.db, september, decimal.NewFromInt(1600)); err != nil {
		t.Fatalf("SetManualFxRate() unexpected error: %v", err)
	}

	stored, err := catalog.FxRates(m.db)
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	m, _ = send(t, m, ratesMsg{stored: stored, settings: catalog.Settings{FxHouse: domain.Blue}})

	rate := m.fx().At(september)
	if rate.Origin != month.RateManual || !rate.Value.Equal(decimal.NewFromInt(1600)) {
		t.Errorf("rate = %+v, want the manual 1600 over the live 1540", rate)
	}
}

// A weekend has no quotes of its own, so the ones on screen are the last day
// the market traded. The screen says which day that was rather than letting
// Friday's numbers pass for today's.
func TestRatesScreenNamesTheDayItsQuotesAreFrom(t *testing.T) {
	m := modelFor(t, richWorld(), minUsableWidth, 32)
	m.view = viewRates
	m = m.sync()

	if !strings.Contains(stripANSI(m.renderRates()), "quoted 2026-09-02") {
		t.Errorf("the rates screen does not date its quotes:\n%s", stripANSI(m.renderRates()))
	}
}
