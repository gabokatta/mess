package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
)

func period(m time.Month) domain.Period { return domain.NewPeriod(fixture.Year, m) }

// A year holding a close, a gap that has to inherit, and a manual override,
// with today sitting in September so the last quarter is unreached.
func ratesWorld() fixture.World {
	return fixture.World{
		FxHouse: domain.Blue,
		Rates: []fixture.Rate{
			{Period: period(time.January), Value: "1050"},
			{Period: period(time.February), Value: "1088"},
			{Period: period(time.April), Value: "1150"},
			{Period: period(time.June), Value: "1356", Source: catalog.Manual},
			{Period: period(time.August), Value: "1500"},
		},
	}
}

func rowsFor(t *testing.T, m Model) map[time.Month]rateRow {
	t.Helper()
	rows := m.rateRows()
	if len(rows) != 12 {
		t.Fatalf("rateRows() = %d rows, want 12", len(rows))
	}
	byMonth := make(map[time.Month]rateRow, 12)
	for _, r := range rows {
		byMonth[r.period.Month()] = r
	}
	return byMonth
}

func TestRateRowsCoverEveryMonthOfTheYear(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 32)

	rows := m.rateRows()
	for i, r := range rows {
		if want := time.Month(i + 1); r.period.Month() != want {
			t.Errorf("rateRows()[%d] = %s, want %s", i, r.period, want)
		}
		if r.period.Year() != fixture.Year {
			t.Errorf("rateRows()[%d] = %s, want year %d", i, r.period, fixture.Year)
		}
	}
}

func TestRateRowsNameWhereEachRateCameFrom(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 32)
	rows := rowsFor(t, m)

	for _, tt := range []struct {
		month time.Month
		want  rateStatus
	}{
		{time.January, statusClose},
		{time.March, statusInherited},
		{time.April, statusClose},
		{time.June, statusManual},
		{time.September, statusLive},
		{time.October, statusPending},
		{time.December, statusPending},
	} {
		if got := rows[tt.month].status; got != tt.want {
			t.Errorf("%s = %q, want %q", tt.month, got, tt.want)
		}
	}
}

func TestRateRowsGiveAnUnreachedMonthNoValue(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 32)
	rows := rowsFor(t, m)

	for _, month := range []time.Month{time.October, time.November, time.December} {
		if row := rows[month]; row.rate.OK() {
			t.Errorf("%s carries %s, want no rate at all", month, row.rate.Value)
		}
	}
}

func TestRateRowsFlagACloseFetchedAtAnotherHouse(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 32)
	rows := rowsFor(t, m)
	if rows[time.January].mismatch {
		t.Error("january is flagged while the house still matches")
	}

	m, _ = send(t, m, ratesMsg{stored: m.stored, settings: catalog.Settings{FxHouse: domain.Official}})
	rows = rowsFor(t, m)

	if !rows[time.January].mismatch {
		t.Error("january is not flagged after the house moved to Official")
	}
	if rows[time.June].mismatch {
		t.Error("june is flagged, but a manual rate came from no house")
	}
	if rows[time.March].mismatch {
		t.Error("march is flagged, but an inherited row stores nothing of its own")
	}
}

func TestRateDeltaMeasuresAgainstTheMonthBefore(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 32)
	rows := rowsFor(t, m)

	if rows[time.January].hasDelta {
		t.Error("january carries a delta with no month before it")
	}
	// 1088 over 1050 is 3,62%.
	if got := signedPercent(rows[time.February].delta); got != "+3,6%" {
		t.Errorf("february delta = %s, want +3,6%%", got)
	}
	if rows[time.October].hasDelta {
		t.Error("an unreached month carries a delta")
	}
}

func TestRatesTableRendersItsColumnsAndRows(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 32)
	m.view = viewRates
	m = m.sync()
	out := stripANSI(m.renderRates())

	for _, want := range []string{"MONTH", "RATE", "SOURCE", "HOUSE", "jan", "dec", "inherited", "pending"} {
		if !strings.Contains(out, want) {
			t.Errorf("the table does not show %q:\n%s", want, out)
		}
	}
}

func TestRatesCursorWalksTheTableAndTheYearKeysStepAYear(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 32)
	m.view = viewRates
	m = m.sync()

	start := m.ratesList.cursor
	m, _ = send(t, m, key("down"), key("down"))
	if m.ratesList.cursor != start+2 {
		t.Errorf("cursor = %d, want %d after two downs", m.ratesList.cursor, start+2)
	}

	m, _ = send(t, m, key("right"))
	if got := m.period.Year(); got != fixture.Year+1 {
		t.Errorf("year = %d, want %d after one right", got, fixture.Year+1)
	}
}

func TestRatesCursorOpensOnTheAppPeriod(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 32)
	m, _ = send(t, m, key("tab"), key("tab"), key("tab"), key("tab"))
	if m.view != viewRates {
		t.Fatalf("view = %v, want viewRates", m.view)
	}
	m = m.sync()

	if got := m.rateRows()[m.ratesList.cursor].period; !got.Equal(fixture.Period) {
		t.Errorf("cursor sits on %s, want %s", got, fixture.Period)
	}
}

func TestRatesYearWithNothingStoredIsTwelvePendingRows(t *testing.T) {
	m := modelFor(t, fixture.World{}, minUsableWidth, 32)
	m.view = viewRates
	m, _ = send(t, m, ratesMsg{settings: catalog.Settings{FxHouse: domain.Blue}}, quotesMsg{})
	m = m.sync()

	for _, r := range m.rateRows() {
		if r.status != statusPending {
			t.Errorf("%s = %q, want every month pending", r.period, r.status)
		}
	}
	for i, row := range m.rateTableRows() {
		if !strings.Contains(stripANSI(row), "pending") {
			t.Errorf("table row %d does not read pending:\n%s", i, stripANSI(row))
		}
	}
}

func TestRatePaneNamesTheProvenanceOfTheCursorMonth(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 34)
	m.view = viewRates
	m = m.sync()

	// September: today, so the live quote at the house in use.
	out := stripANSI(m.renderRates())
	for _, want := range []string{"SEPTEMBER 2026", "Rate", "Source", "House", "Quoted", "2026-09-02"} {
		if !strings.Contains(out, want) {
			t.Errorf("the pane does not carry %q:\n%s", want, out)
		}
	}
}

func TestRatePaneNamesWhereAnInheritedRateCameFrom(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 34)
	m.view = viewRates
	m.ratesList.cursor = int(time.March) - 1
	m = m.sync()

	out := stripANSI(m.renderRates())
	if !strings.Contains(out, "From") || !strings.Contains(out, period(time.February).String()) {
		t.Errorf("the pane does not name the month march inherited from:\n%s", out)
	}
	if strings.Contains(out, "Quoted") {
		t.Errorf("an inherited month prints a quote date it does not have:\n%s", out)
	}
}

func TestRatePaneLeavesOutWhatAManualRateDoesNotHave(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 34)
	m.view = viewRates
	m.ratesList.cursor = int(time.June) - 1
	m = m.sync()

	out := stripANSI(m.renderRates())
	if strings.Contains(out, "Quoted") {
		t.Errorf("a manual rate prints a quote date:\n%s", out)
	}
	if !strings.Contains(out, "manual") {
		t.Errorf("the pane does not name the manual source:\n%s", out)
	}
}

func TestRatePaneHeightDoesNotMoveWithTheCursor(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 34)
	m.view = viewRates

	want := -1
	for cursor := range ratesMonths {
		m.ratesList.cursor = cursor
		m = m.sync()
		got := lipgloss.Height(m.ratePane(m.cursorRate()))
		if want == -1 {
			want = got
		}
		if got != want {
			t.Fatalf("pane is %d lines on %s, want %d everywhere", got, m.cursorRate().period, want)
		}
	}
}

func TestHousesBlockSurvivesAFailedFetch(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 34)
	m.view = viewRates
	m, _ = send(t, m, quotesMsg{})
	m = m.sync()

	out := stripANSI(m.renderRates())
	for _, want := range []string{"HOUSES", "blue", "official", "mep", "no quote today"} {
		if !strings.Contains(out, want) {
			t.Errorf("the houses block does not show %q with no quotes:\n%s", want, out)
		}
	}
}

func TestHousesBlockMarksTheOneInUseAndCarriesItsSpread(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 34)
	m.view = viewRates
	m = m.sync()

	out := stripANSI(m.renderRates())
	if !strings.Contains(out, "using") {
		t.Errorf("no house is marked as the one in use:\n%s", out)
	}
	// Blue quotes 1520 / 1540 in the test harness.
	if !strings.Contains(out, "1.540") || !strings.Contains(out, "20") {
		t.Errorf("blue's sell and spread are not both shown:\n%s", out)
	}
}

func TestMonthOnMonthChartPlotsTheDeltas(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 40)
	m.view = viewRates
	m = m.sync()

	bars := m.rateDeltaBars()
	if len(bars) != ratesMonths {
		t.Fatalf("rateDeltaBars() = %d bars, want %d", len(bars), ratesMonths)
	}
	if !bars[0].value.IsZero() {
		t.Errorf("january plots %s, want nothing before the first month", bars[0].value)
	}
	if got := signedPercent(bars[1].value); got != "+3,6%" {
		t.Errorf("february plots %s, want +3,6%%", got)
	}
	for _, i := range []int{9, 10, 11} {
		if !bars[i].pending {
			t.Errorf("bar %d is not marked pending", i)
		}
	}
}

func TestMonthOnMonthChartKeepsANegativeMonth(t *testing.T) {
	m := modelFor(t, fixture.World{
		FxHouse: domain.Blue,
		Rates: []fixture.Rate{
			{Period: period(time.January), Value: "1200"},
			{Period: period(time.February), Value: "1000"},
		},
	}, minUsableWidth, 40)
	m.view = viewRates
	m = m.sync()

	if got := m.rateDeltaBars()[1].value; !got.IsNegative() {
		t.Fatalf("february plots %s, want a negative move", got)
	}
	if out := stripANSI(m.renderRates()); !strings.Contains(out, "-16,7%") {
		t.Errorf("the table does not carry the negative move:\n%s", out)
	}
}

func TestMonthOnMonthChartNamesItsSteepestMonthAndTheYearsDrift(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 40)
	m.view = viewRates
	m = m.sync()

	out := stripANSI(m.renderRates())
	for _, want := range []string{"MONTH ON MONTH", "steepest", "year +"} {
		if !strings.Contains(out, want) {
			t.Errorf("the chart caption does not carry %q:\n%s", want, out)
		}
	}
}

func TestRatesTitleBreaksDownWhereTheYearsRatesCameFrom(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 40)
	m.view = viewRates
	m = m.sync()

	out := stripANSI(m.renderRates())
	for _, want := range []string{"2026", "close 4", "manual 1", "live 1", "inherited 3", "pending 3"} {
		if !strings.Contains(out, want) {
			t.Errorf("the title does not carry %q:\n%s", want, out)
		}
	}
}

func TestRatesTitleNamesOnlyTheStatesThatOccur(t *testing.T) {
	m := modelFor(t, fixture.World{}, minUsableWidth, 40)
	m.view = viewRates
	m, _ = send(t, m, ratesMsg{settings: catalog.Settings{FxHouse: domain.Blue}}, quotesMsg{})
	m = m.sync()

	out := stripANSI(m.renderRates())
	if !strings.Contains(out, "pending 12") {
		t.Errorf("the title does not count the pending months:\n%s", out)
	}
	if strings.Contains(out, "close 0") || strings.Contains(out, "manual 0") {
		t.Errorf("the title counts states that did not occur:\n%s", out)
	}
}

func TestRatesTitleCountsTheRowsFetchedAtAnotherHouse(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 40)
	m.view = viewRates
	m, _ = send(t, m, ratesMsg{stored: m.stored, settings: catalog.Settings{FxHouse: domain.Official}})
	m = m.sync()

	if out := stripANSI(m.renderRates()); !strings.Contains(out, "4 at another house") {
		t.Errorf("the title does not count the mismatched rows:\n%s", out)
	}
}

func TestSetRateEditsTheCursorRowNotTheAppPeriod(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 40)
	m.view = viewRates
	m.ratesList.cursor = int(time.March) - 1
	m = m.sync()

	m, _ = send(t, m, key("e"))
	if _, ok := m.topModal().(*form); !ok {
		t.Fatalf("modal = %T, want *form", m.topModal())
	}
	if !strings.Contains(m.topModal().View(), period(time.March).String()) {
		t.Errorf("the form does not name march:\n%s", m.topModal().View())
	}
}

func TestSetRateRefusesAMonthNobodyHasReached(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 40)
	m.view = viewRates
	m.ratesList.cursor = int(time.December) - 1
	m = m.sync()

	m, _ = send(t, m, key("e"))
	if m.topModal() != nil {
		t.Errorf("modal = %T, want none on an unreached month", m.topModal())
	}
}

func TestClearRateRemovesTheStoredRow(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 40)
	m.view = viewRates
	m.ratesList.cursor = int(time.June) - 1
	m = m.sync()

	_, cmd := send(t, m, key("d"))
	if err := runWrite(t, cmd); err != nil {
		t.Fatalf("clearing a rate reported an error: %v", err)
	}

	stored, err := catalog.FxRates(m.db)
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	for _, r := range stored {
		if r.Period.Equal(period(time.June)) {
			t.Fatalf("june is still stored as %+v", r)
		}
	}
}

func TestClearRateDoesNothingOnAnInheritedRow(t *testing.T) {
	m := modelFor(t, ratesWorld(), minUsableWidth, 40)
	m.view = viewRates
	m.ratesList.cursor = int(time.March) - 1
	m = m.sync()

	if _, cmd := send(t, m, key("d")); cmd != nil {
		t.Error("d acts on a row that stores nothing")
	}
}
