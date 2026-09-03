package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
	"github.com/gabokatta/mess/internal/month"
)

func TestRatesEnterAdoptsTheHouseUnderTheCursor(t *testing.T) {
	m := modelFor(t, fixture.World{}, 90, 30)
	m.view = viewRates

	m, _ = send(t, m, key("down"), key("down"))
	_, cmd := send(t, m, key("enter"))
	if err := runWrite(t, cmd); err != nil {
		t.Fatalf("adopting a house reported an error: %v", err)
	}

	got, err := catalog.FxHouse(m.db)
	if err != nil {
		t.Fatalf("FxHouse() unexpected error: %v", err)
	}
	if got != domain.MEP {
		t.Errorf("FxHouse() = %v, want MEP", got)
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

// e prefills the by-hand editor with the rate currently in effect.
func TestManualRateFormOpensOnTheShownPeriod(t *testing.T) {
	m := modelFor(t, fixture.World{}, 90, 30)
	m.view = viewRates
	m, _ = send(t, m, ratesMsg{settings: catalog.Settings{FxHouse: domain.Blue}})

	m, _ = send(t, m, key("e"))
	if _, ok := m.modal.(*form); !ok {
		t.Fatalf("modal = %T, want *form", m.modal)
	}
	if !strings.Contains(m.modal.View(), september.String()) {
		t.Errorf("form view does not name the shown period:\n%s", m.modal.View())
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

// An inherited rate is not a close and gets no bar.
func TestYearClosesLeavesUnknownMonthsAtZero(t *testing.T) {
	m := modelFor(t, fixture.World{}, 90, 30)
	m.view = viewRates
	m, _ = send(t, m, ratesMsg{
		settings: catalog.Settings{FxHouse: domain.Blue},
		stored: []catalog.FxRate{
			{Period: domain.NewPeriod(fixture.Year, time.January), Value: decimal.NewFromInt(1100), Source: catalog.Close},
			{Period: domain.NewPeriod(fixture.Year, time.March), Value: decimal.NewFromInt(1200), Source: catalog.Close},
		},
	})

	closes := m.yearCloses()
	if !closes[0].Equal(decimal.NewFromInt(1100)) || !closes[2].Equal(decimal.NewFromInt(1200)) {
		t.Errorf("stored closes = %v, want january and march", closes)
	}
	if !closes[1].IsZero() {
		t.Errorf("february = %s, want zero rather than an inherited bar", closes[1])
	}
	if !closes[8].Equal(decimal.NewFromInt(1540)) {
		t.Errorf("september = %s, want the live quote", closes[8])
	}
}
