package tui

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/fixture"
)

func TestMonthLoadIgnoresAnOlderResponse(t *testing.T) {
	m := modelFor(t, richWorld(), minUsableWidth, 32)
	old := m.loadMonth()()
	m, _ = m.goTo(m.period.AddMonths(1))
	if len(m.lines) != 0 {
		t.Fatal("navigation left the previous month's rows editable")
	}
	current := m.loadMonth()()
	m, _ = send(t, m, current)
	want := m.lines
	m, _ = send(t, m, old)
	if diff := cmp.Diff(want, m.lines); diff != "" {
		t.Fatalf("older month replaced the selected month (-want +got):\n%s", diff)
	}
}

func TestYearLoadIgnoresAnOlderExchangeRate(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{{Name: "Salary", Category: "Earnings", Kind: catalog.Income}},
		Entries:  []fixture.Entry{{Concept: "Salary", Period: fixture.Period, Amount: "3000"}},
		Rates:    []fixture.Rate{{Period: fixture.Period, Value: "1000"}},
	}, minUsableWidth, 32)
	old := m.loadYear()()
	if err := catalog.SetManualFxRate(m.db, m.period, decimal.NewFromInt(1500)); err != nil {
		t.Fatal(err)
	}
	m, cmd := send(t, m, loadRates(m.db)())
	m, _ = send(t, m, runCmd(t, cmd), old)
	if !m.year.Earned.USD.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("earned USD = %s, want 2 at the new rate", m.year.Earned.USD)
	}
}

func TestFailedLoadPreservesVisibleData(t *testing.T) {
	m := modelFor(t, richWorld(), minUsableWidth, 32)
	want := m.concepts
	m, _ = send(t, m, catalogMsg{err: errors.New("database unavailable")})
	if diff := cmp.Diff(want, m.concepts); diff != "" {
		t.Fatalf("failed load discarded the catalog (-want +got):\n%s", diff)
	}
	if m.flash != "database unavailable" {
		t.Fatalf("flash = %q", m.flash)
	}
}
