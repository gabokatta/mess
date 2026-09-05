package tui

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/cursor"
	tea "charm.land/bubbletea/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
	"github.com/gabokatta/mess/internal/month"
)

var september = fixture.Period

func key(s string) tea.KeyPressMsg {
	switch s {
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	default:
		return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
	}
}

func send(t *testing.T, m Model, msgs ...tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, msg := range msgs {
		next, c := m.Update(msg)
		m, cmd = next.(Model), c
	}
	return m, cmd
}

const pumpDeadline = 5 * time.Second

// Huh advances its fields through commands, so a form only completes this
// way. Writes are collected, not applied, to skip the reload cascade.
func pump(t *testing.T, m Model, cmd tea.Cmd) (Model, []savedMsg) {
	t.Helper()
	var writes []savedMsg
	queue := []tea.Cmd{cmd}

	for steps := 0; len(queue) > 0; steps++ {
		if steps > 200 {
			t.Fatal("pump did not settle")
		}
		next := queue[0]
		queue = queue[1:]
		if next == nil {
			continue
		}
		switch msg := runWithDeadline(t, next).(type) {
		case nil:
		case cursor.BlinkMsg:
		case tea.BatchMsg:
			queue = append(queue, msg...)
		case savedMsg:
			writes = append(writes, msg)
		default:
			updated, c := m.Update(msg)
			m = updated.(Model)
			queue = append(queue, c)
		}
	}
	return m, writes
}

func runWithDeadline(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	answered := make(chan tea.Msg, 1)
	go func() { answered <- cmd() }()
	select {
	case msg := <-answered:
		return msg
	case <-time.After(pumpDeadline):
		name := runtime.FuncForPC(reflect.ValueOf(cmd).Pointer()).Name()
		t.Fatalf("%s did not answer within %s", name, pumpDeadline)
		return nil
	}
}

func runWrite(t *testing.T, cmd tea.Cmd) error {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}
	msg, ok := cmd().(savedMsg)
	if !ok {
		t.Fatalf("command produced %T, want savedMsg", msg)
	}
	return msg.err
}

func TestTabCyclesViews(t *testing.T) {
	m := modelFor(t, fixture.World{}, 90, 30)

	for _, want := range []view{viewYear, viewNotes, viewConcepts, viewRates, viewMonth} {
		m, _ = send(t, m, key("tab"))
		if m.view != want {
			t.Fatalf("after tab, view = %s, want %s", m.view, want)
		}
	}

	m, _ = send(t, m, key("shift+tab"))
	if m.view != viewRates {
		t.Errorf("after shift+tab from Month, view = %s, want Rates", m.view)
	}
}

func TestMonthCursorFollowsTheGroupedOrder(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{
			{Name: "Wash the house", Category: "Home", Kind: catalog.Chore},
			{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "785000"},
			{Name: "Salary", Category: "Earnings", Kind: catalog.Income, Base: "2400000"},
			{Name: "Dollars", Category: "Home", Kind: catalog.Saving, Base: "480000"},
		},
		Entries: []fixture.Entry{
			{Concept: "Salary", Period: fixture.Period, Amount: "2400000"},
		},
	}, 90, 30)

	want := []string{"Salary", "Rent", "Dollars", "Wash the house"}
	for i, name := range want {
		if i > 0 {
			m, _ = send(t, m, key("down"))
		}
		got, ok := m.cursorLine()
		if !ok || got.Concept.Name != name {
			t.Fatalf("cursor at %d = %q, want %q", i, got.Concept.Name, name)
		}
	}

	m, _ = send(t, m, key("down"))
	if got, _ := m.cursorLine(); got.Concept.Name != "Wash the house" {
		t.Errorf("cursor past the end = %q, want it clamped to the last line", got.Concept.Name)
	}
}

func TestMonthAnchorsPointAtSelectableRowsOnly(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{
			{Name: "Salary", Category: "Earnings", Kind: catalog.Income, Base: "2400000"},
			{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "785000"},
			{Name: "ABL", Category: "Home", Kind: catalog.Expense, Base: "34200"},
		},
		Entries: []fixture.Entry{
			{Concept: "Salary", Period: fixture.Period, Amount: "2400000"},
		},
	}, 90, 30)

	rows, anchors := m.monthRows()
	if len(anchors) != 3 {
		t.Fatalf("anchors = %v, want one per line", anchors)
	}
	for _, at := range anchors {
		if rows[at] == "" {
			t.Errorf("anchor %d points at a blank row", at)
		}
	}
	if anchors[0] != 1 {
		t.Errorf("first anchor = %d, want 1 (the row after the INCOME header)", anchors[0])
	}
}

func TestSpaceTicksTheLineUnderTheCursor(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "785000"}},
	}, 90, 30)

	_, cmd := send(t, m, key("space"))
	if err := runWrite(t, cmd); err != nil {
		t.Fatalf("ticking reported an error: %v", err)
	}

	loaded, err := month.Load(m.db, september)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if !loaded.Lines[0].Done {
		t.Error("the line should be done after space")
	}
}

func TestEditOpensTheInlineAmountEditAndEscCloses(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "785000"}},
	}, 90, 30)

	m, _ = send(t, m, key("e"))
	edit, ok := m.topModal().(*amountEdit)
	if !ok {
		t.Fatalf("modal = %T, want *amountEdit", m.topModal())
	}
	if edit.input.Value() != "785000.00" {
		t.Errorf("edit box opened with %q, want the baseline prefilled", edit.input.Value())
	}

	m, _ = send(t, m, key("esc"))
	if m.topModal() != nil {
		t.Error("esc should close the edit")
	}
}

func TestEditIsANoOpOnAChore(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{{Name: "Wash the house", Category: "Home", Kind: catalog.Chore}},
	}, 90, 30)

	m, _ = send(t, m, key("e"))
	if m.topModal() != nil {
		t.Errorf("modal = %T, want none on a chore", m.topModal())
	}
}

func TestAmountEditCommitsTheTypedValue(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "785000"}},
	}, 90, 30)

	m, _ = send(t, m, key("e"))
	m.topModal().(*amountEdit).input.SetValue("812000")
	m, cmd := send(t, m, key("enter"))

	if m.topModal() != nil {
		t.Error("enter should close the edit")
	}
	if err := runWrite(t, cmd); err != nil {
		t.Fatalf("commit reported an error: %v", err)
	}

	loaded, err := month.Load(m.db, september)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	got := loaded.Lines[0]
	if !got.Money.Overridden || !got.Money.Amount.Amount().Equal(decimal.NewFromInt(812000)) {
		t.Errorf("line = %+v, want a confirmed 812000", got.Money)
	}
}

func TestShiftPeriodResetsTheCursor(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{
			{Name: "Salary", Category: "Earnings", Kind: catalog.Income, Base: "2400000"},
			{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "785000"},
		},
		Entries: []fixture.Entry{
			{Concept: "Salary", Period: fixture.Period, Amount: "2400000"},
		},
	}, 90, 30)

	m, _ = send(t, m, key("down"))
	m, _ = send(t, m, key("right"))

	if !m.period.Equal(domain.NewPeriod(fixture.Year, time.October)) {
		t.Errorf("period = %s, want %d-10", m.period, fixture.Year)
	}
	if m.monthList.cursor != 0 {
		t.Errorf("cursor = %d, want 0 — the row it pointed at belongs to the month being left", m.monthList.cursor)
	}
}

func TestShiftPeriodMovesAYearInTheYearView(t *testing.T) {
	m := modelFor(t, fixture.World{}, 90, 30)
	m, _ = send(t, m, key("tab"), key("right"))

	if m.period.Year() != fixture.Year+1 || m.period.Month() != time.September {
		t.Errorf("period = %s, want %d-09", m.period, fixture.Year+1)
	}
}

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"2400000", "2.400.000"},
		{"785000", "785.000"},
		{"1000", "1.000"},
		{"999", "999"},
		{"0", "0"},
		{"-30000", "-30.000"},
		{"1520.50", "1.520,50"},
		{"1234567.89", "1.234.567,89"},
	}
	for _, tt := range tests {
		d, err := decimal.NewFromString(tt.in)
		if err != nil {
			t.Fatalf("bad test input %q: %v", tt.in, err)
		}
		if got := formatAmount(d); got != tt.want {
			t.Errorf("formatAmount(%s) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMonthListScrollsWithTheCursor(t *testing.T) {
	concepts := make([]fixture.Concept, 0, 30)
	for i := 1; i <= 30; i++ {
		concepts = append(concepts, fixture.Concept{
			Name: fmt.Sprintf("Bill %02d", i), Category: "Home", Kind: catalog.Expense,
			Base: fmt.Sprint(i * 1000),
		})
	}
	m := modelFor(t, fixture.World{Concepts: concepts}, 80, 24)

	if m.monthList.vp.YOffset() != 0 {
		t.Fatalf("offset at the top = %d, want 0", m.monthList.vp.YOffset())
	}

	for range 29 {
		m, _ = send(t, m, key("down"))
	}
	if m.monthList.vp.YOffset() == 0 {
		t.Fatal("the viewport never scrolled, so the last lines are unreachable")
	}
	if got, _ := m.cursorLine(); got.Concept.Name != "Bill 30" {
		t.Fatalf("cursor = %q, want Bill 30", got.Concept.Name)
	}
	if !strings.Contains(stripANSI(m.monthList.View()), "Bill 30") {
		t.Errorf("the cursor's line is off screen:\n%s", stripANSI(m.monthList.View()))
	}

	for range 29 {
		m, _ = send(t, m, key("up"))
	}
	if m.monthList.vp.YOffset() != 0 {
		t.Errorf("offset back at the top = %d, want 0", m.monthList.vp.YOffset())
	}
}

func TestOpenModalKeepsTheArrows(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "785000"}},
	}, 90, 30)
	m.view = viewConcepts

	m, cmd := send(t, m, key("d"))
	m, _ = pump(t, m, cmd)
	before := m.period

	m, _ = send(t, m, key("left"), key("right"))
	if !m.period.Equal(before) {
		t.Errorf("period moved to %s while a form was open", m.period)
	}
	if m.topModal() == nil {
		t.Error("the form should still be open")
	}
}

func TestArrowsMoveThePeriodOnlyWhereOneIsShown(t *testing.T) {
	m := modelFor(t, fixture.World{}, 90, 30)

	for range viewNames {
		view, moves := m.view, m.showsPeriod()
		before := m.period

		m, _ = send(t, m, key("right"))
		switch moved := !m.period.Equal(before); {
		case moves && !moved:
			t.Errorf("right did not move the period in %s", view)
		case !moves && moved:
			t.Errorf("right moved the period in %s, which shows none", view)
		}

		m, _ = send(t, m, key("left"))
		if !m.period.Equal(before) {
			t.Errorf("the period did not come back in %s", view)
		}
		m, _ = send(t, m, key("tab"))
	}

	if m.showsPeriod() != true || m.view != viewMonth {
		t.Fatalf("expected to land back on Month, got %s", m.view)
	}
}

func TestConceptsHasNoPeriod(t *testing.T) {
	m := modelFor(t, fixture.World{}, 90, 30)
	m, _ = send(t, m, key("tab"), key("tab"), key("tab"))
	if m.view != viewConcepts {
		t.Fatalf("view = %s, want Concepts", m.view)
	}
	if m.showsPeriod() {
		t.Error("Concepts should not claim a period")
	}
	if strings.Contains(stripANSI(m.renderConcepts()), m.period.String()) {
		t.Error("the Concepts header should not name a period")
	}
}

func TestTodayReturnsToTheRunningMonth(t *testing.T) {
	m := modelFor(t, fixture.World{}, 90, 30)

	m, _ = send(t, m, key("right"), key("right"), key("right"))
	if m.period.Equal(m.today) {
		t.Fatal("expected to be off the running month")
	}

	m, _ = send(t, m, key("t"))
	if !m.period.Equal(m.today) {
		t.Errorf("period = %s, want today's month %s", m.period, m.today)
	}
}

func TestTodayHintAppearsOnlyWhenOffTheRunningMonth(t *testing.T) {
	m := modelFor(t, fixture.World{}, 90, 30)

	if strings.Contains(m.help(), "t today") {
		t.Errorf("help offers t on the running month: %s", m.help())
	}

	m, _ = send(t, m, key("right"))
	if !strings.Contains(m.help(), "t today") {
		t.Errorf("help does not offer t after wandering: %s", m.help())
	}

	m, _ = send(t, m, key("t"))
	if strings.Contains(m.help(), "t today") {
		t.Errorf("help still offers t after returning: %s", m.help())
	}
}

func TestTodayIsInertWhereNoPeriodIsShown(t *testing.T) {
	m := modelFor(t, fixture.World{}, 90, 30)
	m, _ = send(t, m, key("right"))
	wandered := m.period

	m, _ = send(t, m, key("tab"), key("tab"), key("tab"))
	if m.view != viewConcepts {
		t.Fatalf("view = %s, want Concepts", m.view)
	}

	m, _ = send(t, m, key("t"))
	if !m.period.Equal(wandered) {
		t.Errorf("t moved the period from Concepts, which shows none")
	}
	if strings.Contains(m.help(), "t today") {
		t.Errorf("Concepts help offers t: %s", m.help())
	}
}

func TestEveryHelpLineNamesTabAndQuit(t *testing.T) {
	m := modelFor(t, fixture.World{}, 90, 30)
	for range viewNames {
		if got := m.help(); !strings.HasSuffix(got, "tab switch · q quit") {
			t.Errorf("%s help = %q, want it to end with the two global keys", m.view, got)
		}
		m, _ = send(t, m, key("tab"))
	}
}
