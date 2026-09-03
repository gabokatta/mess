package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
	"github.com/gabokatta/mess/internal/rates"
)

func ratesModel(t *testing.T) Model {
	t.Helper()
	m := New(testDB(t))
	m.today = september
	m.period = september
	m.view = viewRates
	return applySize(t, m, quotesMsg{quotes: []rates.Quote{
		{House: domain.Blue, Buy: decimal.NewFromInt(1520), Sell: decimal.NewFromInt(1540)},
		{House: domain.Official, Buy: decimal.NewFromInt(1485), Sell: decimal.NewFromInt(1535)},
		{House: domain.MEP, Buy: decimal.NewFromInt(1532), Sell: decimal.NewFromInt(1535)},
	}})
}

func applySize(t *testing.T, m Model, msgs ...tea.Msg) Model {
	t.Helper()
	m, _ = send(t, m, tea.WindowSizeMsg{Width: 90, Height: 30})
	m, _ = send(t, m, msgs...)
	return m
}

func TestRatesEnterAdoptsTheHouseUnderTheCursor(t *testing.T) {
	m := ratesModel(t)

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

// The house the app converts with is the one the live quote is read from,
// so adopting a house changes every total that runs through a rate.
func TestFxTableFollowsTheAdoptedHouse(t *testing.T) {
	m := ratesModel(t)

	m, _ = send(t, m, ratesMsg{settings: catalog.Settings{FxHouse: domain.Blue}})
	if rate := m.fx().At(september); !rate.Value.Equal(decimal.NewFromInt(1540)) {
		t.Errorf("live rate on Blue = %s, want 1540", rate.Value)
	}

	m, _ = send(t, m, ratesMsg{settings: catalog.Settings{FxHouse: domain.Official}})
	if rate := m.fx().At(september); !rate.Value.Equal(decimal.NewFromInt(1535)) {
		t.Errorf("live rate on Official = %s, want 1535", rate.Value)
	}
}

// e opens the by-hand editor prefilled with the rate currently in effect,
// so correcting a published rate starts from what the app believed.
func TestManualRateFormOpensOnTheShownPeriod(t *testing.T) {
	m := ratesModel(t)
	m, _ = send(t, m, ratesMsg{settings: catalog.Settings{FxHouse: domain.Blue}})

	m, _ = send(t, m, key("e"))
	if _, ok := m.modal.(*form); !ok {
		t.Fatalf("modal = %T, want *form", m.modal)
	}
	if !strings.Contains(m.modal.View(), september.String()) {
		t.Errorf("form view does not name the shown period:\n%s", m.modal.View())
	}
}

// A rate set by hand wins over the live quote for the same month, and
// because conversion is read-time it cascades through that month's totals.
func TestManualRateOverridesTheLiveQuote(t *testing.T) {
	m := ratesModel(t)
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

// The chart only draws a month mess actually knows a rate for; an inherited
// rate is not a close and gets no bar.
func TestYearClosesLeavesUnknownMonthsAtZero(t *testing.T) {
	m := ratesModel(t)
	m, _ = send(t, m, ratesMsg{
		settings: catalog.Settings{FxHouse: domain.Blue},
		stored: []catalog.FxRate{
			{Period: domain.NewPeriod(2026, time.January), Value: decimal.NewFromInt(1100), Source: catalog.Close},
			{Period: domain.NewPeriod(2026, time.March), Value: decimal.NewFromInt(1200), Source: catalog.Close},
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
