package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
)

func TestYearFitsEveryTerminalSize(t *testing.T) {
	for _, size := range [][2]int{{minUsableWidth, minUsableHeight}, {150, 40}, {160, 46}, {220, 60}} {
		width, height := size[0], size[1]
		t.Run(fmt.Sprintf("%dx%d", width, height), func(t *testing.T) {
			m := modelFor(t, fixture.Demo(fixture.Period), width, height)
			m.view = viewYear
			m = m.sync()

			for i, line := range strings.Split(m.View().Content, "\n") {
				if got := lineWidth(line); got > width {
					t.Fatalf("line %d is %d columns wide, want at most %d:\n%s", i, got, width, line)
				}
			}
		})
	}
}

func TestYearBoxesShowEveryFigureInBothCurrencies(t *testing.T) {
	m := modelFor(t, fixture.Demo(fixture.Period), 160, 46)
	m.view = viewYear
	content := stripANSI(m.sync().renderYear())

	for title, f := range map[string]struct{ ars, usd decimal.Decimal }{
		"earned": {m.year.Earned.ARS, m.year.Earned.USD},
		"spent":  {m.year.Spent.ARS, m.year.Spent.USD},
		"saved":  {m.year.Saved.ARS, m.year.Saved.USD},
		"pocket": {m.year.Pocket.ARS, m.year.Pocket.USD},
	} {
		if !strings.Contains(content, title) {
			t.Errorf("renderYear is missing the %q box:\n%s", title, content)
		}
		for _, amount := range []string{formatAmount(f.ars), formatAmount(f.usd)} {
			if !strings.Contains(content, amount) {
				t.Errorf("%s box is missing %q:\n%s", title, amount, content)
			}
		}
	}
}

func TestNegativeMonthsHangBelowTheBaseline(t *testing.T) {
	m := modelFor(t, richWorld(), minUsableWidth, 32)

	bars := []yearBar{
		{label: "jan", value: decimal.NewFromInt(100)},
		{label: "feb", value: decimal.NewFromInt(-50)},
	}
	// 100 up and 50 down over six rows puts the baseline four rows from the top.
	lines := strings.Split(stripANSI(m.renderPlot(bars, 1, 6)), "\n")
	if len(lines) != 6 {
		t.Fatalf("renderPlot drew %d rows, want 6", len(lines))
	}

	want := []string{"█  ", "█  ", "█  ", "█  ", "  █", "  █"}
	for i, w := range want {
		if got := strings.TrimRight(lines[i], " "); got != strings.TrimRight(w, " ") {
			t.Errorf("row %d = %q, want %q (jan above the baseline, feb below it)", i, lines[i], w)
		}
	}

	if !strings.Contains(m.renderPlot(bars, 1, 6), m.theme.Alert.Render("█")) {
		t.Error("the negative month is not drawn in the alert colour")
	}
}

func TestPendingMonthsAreLabelledButNotDrawn(t *testing.T) {
	m := modelFor(t, richWorld(), minUsableWidth, 32)

	bars := []yearBar{
		{label: "sep", value: decimal.NewFromInt(10), current: true},
		{label: "dec", pending: true},
	}
	axis := m.renderAxis(bars, 3, 7)

	if want := m.theme.Muted.Faint(true).Width(3).Render("dec"); !strings.Contains(axis, want) {
		t.Errorf("december's label is not dimmed:\n%q", axis)
	}
	if want := m.theme.Accent.Width(3).Render("sep"); !strings.Contains(axis, want) {
		t.Errorf("the current month's label is not accented:\n%q", axis)
	}
}

func TestCategoryRowsCarryAmountAndShare(t *testing.T) {
	m := modelFor(t, fixture.Demo(fixture.Period), 160, 46)

	rows := m.categoryRows(catBarMax, 0)
	if len(rows) != len(m.year.Categories) {
		t.Fatalf("categoryRows drew %d rows, want %d", len(rows), len(m.year.Categories))
	}

	first := stripANSI(rows[0])
	top := m.year.Categories[0]
	share := sharePercent(top.Total, m.year.Spent.ARS)
	for _, want := range []string{top.Category.Name, formatAmount(top.Total), share} {
		if !strings.Contains(first, want) {
			t.Errorf("top category row is missing %q:\n%q", want, first)
		}
	}
	// The bar scales against the largest category, so the rank leader fills it.
	if !strings.Contains(first, strings.Repeat("█", catBarMax)) {
		t.Errorf("the largest category does not fill the bar:\n%q", first)
	}
}

func TestLongCategoryListAnnouncesWhatIsHidden(t *testing.T) {
	concepts := make([]fixture.Concept, 20)
	entries := make([]fixture.Entry, 20)
	for i := range concepts {
		concepts[i] = fixture.Concept{
			Name:     fmt.Sprintf("Concept %d", i),
			Category: fmt.Sprintf("Category %02d", i),
			Kind:     catalog.Expense,
		}
		entries[i] = fixture.Entry{Concept: concepts[i].Name, Period: fixture.Period, Amount: "100000"}
	}

	m := modelFor(t, fixture.World{
		Concepts: concepts,
		Entries:  entries,
		Rates:    []fixture.Rate{{Period: fixture.Period, Value: "1500"}},
	}, minUsableWidth, minUsableHeight)
	m.view = viewYear
	m = m.sync()

	if !strings.Contains(stripANSI(m.renderYear()), "more") {
		t.Error("the category list does not announce hidden rows")
	}
}

func TestEveryArrowPressMovesTheCursorOnScreen(t *testing.T) {
	m := modelFor(t, fixture.Demo(fixture.Period), 160, 46)
	m.view = viewYear
	m = m.sync()
	if len(m.year.Categories) <= catVisibleRows {
		t.Fatalf("the demo year has %d categories; this test needs more than %d",
			len(m.year.Categories), catVisibleRows)
	}

	previous := m.renderYear()
	for i := 1; i < len(m.year.Categories); i++ {
		m = m.moveCursor(1).sync()
		got := m.renderYear()
		if got == previous {
			t.Fatalf("press %d changed nothing on screen", i)
		}
		previous = got
	}

	// Panning to the end brings the last category into the window.
	last := m.year.Categories[len(m.year.Categories)-1]
	if !strings.Contains(stripANSI(previous), last.Category.Name) {
		t.Errorf("panning to the end did not reach %q:\n%s", last.Category.Name, previous)
	}
}

func TestEmptyYearSaysSo(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{
			{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "785000"},
		},
	}, minUsableWidth, minUsableHeight)
	m.view = viewYear

	content := stripANSI(m.sync().renderYear())
	if !strings.Contains(content, "nothing confirmed this year yet") {
		t.Errorf("an untouched year does not say it is empty:\n%s", content)
	}
	if strings.Contains(content, "SPEND BY CATEGORY") {
		t.Errorf("an untouched year still drew the category block:\n%s", content)
	}
}

func TestChartNoteFitsTheChartItLabels(t *testing.T) {
	m := modelFor(t, fixture.Demo(fixture.Period), 160, 46)

	wide := stripANSI(m.chartNote(spentOf, false, 80))
	if !strings.Contains(wide, "ARS") || !strings.Contains(wide, "USD") {
		t.Errorf("chartNote(room 80) = %q, want both currencies", wide)
	}
	if !strings.Contains(wide, "peak jun") {
		t.Errorf("chartNote = %q, want it to name june", wide)
	}

	narrow := stripANSI(m.chartNote(spentOf, false, lipgloss.Width(wide)-1))
	if strings.Contains(narrow, "USD") {
		t.Errorf("chartNote(tight) = %q, want the dollar half dropped", narrow)
	}
	if lipgloss.Width(narrow) > lipgloss.Width(wide)-1 {
		t.Errorf("chartNote(tight) = %q, %d wide, want at most %d",
			narrow, lipgloss.Width(narrow), lipgloss.Width(wide)-1)
	}
	if got := m.chartNote(spentOf, false, 4); got != "" {
		t.Errorf("chartNote(room 4) = %q, want nothing at all", got)
	}
}

func TestTurningPointIgnoresPendingMonths(t *testing.T) {
	m := modelFor(t, fixture.Demo(fixture.Period), 160, 46)

	low, ok := turningPoint(m.year.Months, spentOf, true)
	if !ok {
		t.Fatal("turningPoint found nothing in a populated year")
	}
	if low.Period.After(m.today) {
		t.Errorf("the low landed on %s, a month that has not happened", low.Period)
	}
	if want := domain.NewPeriod(fixture.Year, time.January); !low.Period.Equal(want) {
		t.Errorf("low = %s, want %s (january is the cheapest confirmed month)", low.Period, want)
	}
}
