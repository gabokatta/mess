package tui

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
	"github.com/gabokatta/mess/internal/store"
)

var september = domain.NewPeriod(2026, time.September)

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

// send feeds messages through the real Update, which is where the view logic
// lives, and hands back the model and the last command.
func send(t *testing.T, m Model, msgs ...tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, msg := range msgs {
		next, c := m.Update(msg)
		m, cmd = next.(Model), c
	}
	return m, cmd
}

// pump runs cmd and feeds everything it produces back through Update, the
// way the runtime does. Huh advances its own fields through commands rather
// than synchronously, so a form only reaches its completed state this way.
// Writes are collected instead of applied, so a test can assert on them
// without the reload cascade that follows one in the real app.
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
		msg, answered := runQuickly(next)
		if !answered {
			continue
		}
		switch msg := msg.(type) {
		case nil:
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

// runQuickly reports a command's message, or false when it does not answer
// promptly — a cursor-blink timer, which a test drops rather than waits on.
func runQuickly(cmd tea.Cmd) (tea.Msg, bool) {
	answered := make(chan tea.Msg, 1)
	go func() { answered <- cmd() }()
	select {
	case msg := <-answered:
		return msg, true
	case <-time.After(50 * time.Millisecond):
		return nil, false
	}
}

// runWrite executes a write command and returns the error it reports.
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

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "mess.db"))
	if err != nil {
		t.Fatalf("store.Open() unexpected error: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s.DB()
}

func testLine(id int64, name string, kind catalog.ConceptKind, amount int64, confirmed bool) month.Line {
	c := catalog.Concept{ID: id, Name: name, Kind: kind, CategoryID: 1}
	l := month.Line{Concept: c}
	if kind != catalog.Chore {
		c.Money = &catalog.MoneyDetails{Currency: domain.ARS, Base: decimal.NewFromInt(amount)}
		l.Concept = c
		l.Money = &month.LineMoney{
			Amount:    domain.NewMoney(decimal.NewFromInt(amount), domain.ARS),
			Confirmed: confirmed,
		}
	}
	return l
}

func testModel(t *testing.T, lines ...month.Line) Model {
	t.Helper()
	m := New(testDB(t))
	m.today = september
	m.period = september
	m, _ = send(t, m, tea.WindowSizeMsg{Width: 90, Height: 30}, monthMsg{lines: lines})
	return m
}

func TestTabCyclesViews(t *testing.T) {
	m := testModel(t)

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

// The cursor runs over the grouped order the list renders in, not the order
// the catalog came back in, so index and row always mean the same line.
func TestMonthCursorFollowsTheGroupedOrder(t *testing.T) {
	m := testModel(t,
		testLine(1, "Wash the house", catalog.Chore, 0, false),
		testLine(2, "Rent", catalog.Expense, 785000, false),
		testLine(3, "Salary", catalog.Income, 2400000, true),
		testLine(4, "Dollars", catalog.Saving, 480000, false),
	)

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

// The scroller skips group headers and blank rows: every anchor is a line
// the cursor can actually land on.
func TestMonthAnchorsPointAtSelectableRowsOnly(t *testing.T) {
	m := testModel(t,
		testLine(1, "Salary", catalog.Income, 2400000, true),
		testLine(2, "Rent", catalog.Expense, 785000, false),
		testLine(3, "ABL", catalog.Expense, 34200, false),
	)

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
	db := testDB(t)
	cat, err := catalog.FindOrCreateCategory(db, "Home")
	if err != nil {
		t.Fatalf("FindOrCreateCategory() unexpected error: %v", err)
	}
	rent, err := catalog.CreateConcept(db, catalog.Concept{
		Name: "Rent", CategoryID: cat.ID, Kind: catalog.Expense,
		Money: &catalog.MoneyDetails{Currency: domain.ARS}, MonthMask: domain.Monthly,
		ActiveFrom: domain.NewPeriod(2026, time.January),
	})
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}

	m := New(db)
	m.today = september
	m.period = september
	m, _ = send(t, m, tea.WindowSizeMsg{Width: 90, Height: 30},
		monthMsg{lines: []month.Line{testLine(rent.ID, "Rent", catalog.Expense, 785000, false)}})

	_, cmd := send(t, m, key("space"))
	if err := runWrite(t, cmd); err != nil {
		t.Fatalf("ticking reported an error: %v", err)
	}

	loaded, err := month.Load(db, september)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if !loaded.Lines[0].Done {
		t.Error("the line should be done after space")
	}
}

func TestEditOpensTheInlineAmountEditAndEscCloses(t *testing.T) {
	m := testModel(t, testLine(1, "Rent", catalog.Expense, 785000, false))

	m, _ = send(t, m, key("e"))
	edit, ok := m.modal.(*amountEdit)
	if !ok {
		t.Fatalf("modal = %T, want *amountEdit", m.modal)
	}
	if edit.input.Value() != "785000.00" {
		t.Errorf("edit box opened with %q, want the baseline prefilled", edit.input.Value())
	}

	m, _ = send(t, m, key("esc"))
	if m.modal != nil {
		t.Error("esc should close the edit")
	}
}

// A chore has no amount behind it, so e is a no-op there.
func TestEditIsANoOpOnAChore(t *testing.T) {
	m := testModel(t, testLine(1, "Wash the house", catalog.Chore, 0, false))

	m, _ = send(t, m, key("e"))
	if m.modal != nil {
		t.Errorf("modal = %T, want none on a chore", m.modal)
	}
}

func TestAmountEditCommitsTheTypedValue(t *testing.T) {
	db := testDB(t)
	cat, _ := catalog.FindOrCreateCategory(db, "Home")
	rent, err := catalog.CreateConcept(db, catalog.Concept{
		Name: "Rent", CategoryID: cat.ID, Kind: catalog.Expense,
		Money:     &catalog.MoneyDetails{Currency: domain.ARS, Base: decimal.NewFromInt(785000)},
		MonthMask: domain.Monthly, ActiveFrom: domain.NewPeriod(2026, time.January),
	})
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}

	m := New(db)
	m.today = september
	m.period = september
	m, _ = send(t, m, tea.WindowSizeMsg{Width: 90, Height: 30},
		monthMsg{lines: []month.Line{testLine(rent.ID, "Rent", catalog.Expense, 785000, false)}})

	m, _ = send(t, m, key("e"))
	m.modal.(*amountEdit).input.SetValue("812000")
	m, cmd := send(t, m, key("enter"))

	if m.modal != nil {
		t.Error("enter should close the edit")
	}
	if err := runWrite(t, cmd); err != nil {
		t.Fatalf("commit reported an error: %v", err)
	}

	loaded, err := month.Load(db, september)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	got := loaded.Lines[0]
	if !got.Money.Confirmed || !got.Money.Amount.Amount().Equal(decimal.NewFromInt(812000)) {
		t.Errorf("line = %+v, want a confirmed 812000", got.Money)
	}
}

func TestShiftPeriodResetsTheCursor(t *testing.T) {
	m := testModel(t,
		testLine(1, "Salary", catalog.Income, 2400000, true),
		testLine(2, "Rent", catalog.Expense, 785000, false),
	)

	m, _ = send(t, m, key("down"))
	m, _ = send(t, m, key("right"))

	if !m.period.Equal(domain.NewPeriod(2026, time.October)) {
		t.Errorf("period = %s, want 2026-10", m.period)
	}
	if m.monthList.cursor != 0 {
		t.Errorf("cursor = %d, want 0 — the row it pointed at belongs to the month being left", m.monthList.cursor)
	}
}

// The Year view moves a year at a time, since a year is the unit on screen.
func TestShiftPeriodMovesAYearInTheYearView(t *testing.T) {
	m := testModel(t)
	m, _ = send(t, m, key("tab"), key("right"))

	if m.period.Year() != 2027 || m.period.Month() != time.September {
		t.Errorf("period = %s, want 2027-09", m.period)
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

// Eighteen concepts do not fit an 80x24 terminal: the list scrolls, and the
// cursor pushes the viewport at the edges rather than walking off it.
func TestMonthListScrollsWithTheCursor(t *testing.T) {
	lines := make([]month.Line, 0, 30)
	for i := 1; i <= 30; i++ {
		lines = append(lines, testLine(int64(i), fmt.Sprintf("Bill %02d", i), catalog.Expense, int64(i)*1000, false))
	}

	m := New(testDB(t))
	m.today, m.period = september, september
	m, _ = send(t, m, tea.WindowSizeMsg{Width: 80, Height: 24}, monthMsg{lines: lines})

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

// An open form owns the keyboard, so left/right stay its own navigation
// rather than moving the period out from under it.
func TestOpenModalKeepsTheArrows(t *testing.T) {
	db := testDB(t)
	mustSeed(t, db, "Home", "Rent", catalog.Expense)
	m := conceptsModel(t, db)

	m, cmd := send(t, m, key("d"))
	m, _ = pump(t, m, cmd)
	before := m.period

	m, _ = send(t, m, key("left"), key("right"))
	if !m.period.Equal(before) {
		t.Errorf("period moved to %s while a form was open", m.period)
	}
	if m.modal == nil {
		t.Error("the form should still be open")
	}
}

// The arrows move the period on every screen that shows one, and are inert
// on the screens that do not.
func TestArrowsMoveThePeriodOnlyWhereOneIsShown(t *testing.T) {
	m := testModel(t)

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

// The catalog is period-free: a concept is a template, and month_mask plus
// the active range say when it fires.
func TestConceptsHasNoPeriod(t *testing.T) {
	m := testModel(t)
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

// t goes back to the month still running, from wherever you wandered to.
func TestTodayReturnsToTheRunningMonth(t *testing.T) {
	m := testModel(t)

	m, _ = send(t, m, key("right"), key("right"), key("right"))
	if m.period.Equal(m.today) {
		t.Fatal("expected to be off the running month")
	}

	m, _ = send(t, m, key("t"))
	if !m.period.Equal(m.today) {
		t.Errorf("period = %s, want today's month %s", m.period, m.today)
	}
}

// The hint costs help-line width, so it only appears when pressing t would
// do something.
func TestTodayHintAppearsOnlyWhenOffTheRunningMonth(t *testing.T) {
	m := testModel(t)

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

// Concepts is period-free, so t has nothing to return to there.
func TestTodayIsInertWhereNoPeriodIsShown(t *testing.T) {
	m := testModel(t)
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

// Every screen says what tab and q do, in the same key-then-verb shape as
// the rest of the line.
func TestEveryHelpLineNamesTabAndQuit(t *testing.T) {
	m := testModel(t)
	for range viewNames {
		if got := m.help(); !strings.HasSuffix(got, "tab switch · q quit") {
			t.Errorf("%s help = %q, want it to end with the two global keys", m.view, got)
		}
		m, _ = send(t, m, key("tab"))
	}
}
