package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/fixture"
)

const mixedNote = `# Plan

Some prose before anything can be ticked.

- [ ] milk
- [ ] bread

A closing paragraph that sits under the last checkbox,
long enough that a cursor anchored to checkboxes can
never reach it at all.
`

// scrollHintPattern is the hint exactly: a bare "more" also occurs in prose,
// and matching that made two of these tests pass on the demo note's own words.
var scrollHintPattern = regexp.MustCompile(`[↑↓] \d+( · [↑↓] \d+)? more`)

// checkboxLine is the rendered line that starts the nth checkbox of the note
// the pane is showing.
func checkboxLine(t *testing.T, m Model, n int) int {
	t.Helper()
	for i, l := range m.noteBodyLines() {
		if box, ok := l.ticks(); ok && box == n {
			return i
		}
	}
	t.Fatalf("the rendered body has no checkbox %d", n)
	return -1
}

func TestBodyCursorReachesTheLastLine(t *testing.T) {
	m := modelFor(t, fixture.World{
		Notes: []catalog.Note{{Title: "Plan", BodyMD: mixedNote}},
	}, minUsableWidth, 32)
	m.view = viewNotes
	m, _ = send(t, m, key("enter"))

	last := len(m.noteBodyLines()) - 1
	if last < 6 {
		t.Fatalf("rendered body is %d lines, too short to prove anything", last+1)
	}
	for range last {
		m, _ = send(t, m, key("down"))
	}
	if m.detail.cursor != last {
		t.Errorf("cursor stopped at line %d of %d; prose past the last checkbox is unreachable",
			m.detail.cursor, last)
	}
}

func TestABodyWithNoCheckboxesStillMoves(t *testing.T) {
	m := modelFor(t, fixture.World{
		Notes: []catalog.Note{{Title: "Prose", BodyMD: "Line one.\n\nLine two.\n\nLine three.\n"}},
	}, minUsableWidth, 32)
	m.view = viewNotes
	m, _ = send(t, m, key("enter"), key("down"))

	if m.detail.cursor == 0 {
		t.Error("down should move the cursor in a note that holds no checkboxes")
	}
}

func TestSpaceOnlyTicksCheckboxLines(t *testing.T) {
	m := modelFor(t, fixture.World{
		Notes: []catalog.Note{{Title: "Plan", BodyMD: mixedNote}},
	}, minUsableWidth, 32)
	m.view = viewNotes
	m, _ = send(t, m, key("enter"))

	prose := -1
	for i, l := range m.noteBodyLines() {
		if _, ok := l.ticks(); !ok && prose < 0 && strings.Contains(stripANSI(l.text), "prose before") {
			prose = i
		}
	}
	if prose < 0 {
		t.Fatal("expected a prose line in the rendered body")
	}
	box := checkboxLine(t, m, 0)

	m.detail.cursor = prose
	if _, cmd := send(t, m, key("space")); cmd != nil {
		t.Error("space on a prose line should do nothing")
	}

	m.detail.cursor = box
	_, cmd := send(t, m, key("space"))
	if err := runWrite(t, cmd); err != nil {
		t.Fatalf("space on a checkbox line reported an error: %v", err)
	}
	notes, _ := catalog.Notes(m.db)
	if !strings.Contains(notes[0].BodyMD, "- [x] milk") {
		t.Errorf("body = %q, want the first box ticked", notes[0].BodyMD)
	}
}

func TestEnterAndEscMoveFocusWithoutLeavingTheScreen(t *testing.T) {
	m := modelFor(t, fixture.World{
		Notes: []catalog.Note{{Title: "Plan", BodyMD: mixedNote}},
	}, minUsableWidth, 32)
	m.view = viewNotes

	m, _ = send(t, m, key("enter"))
	if m.notesFocus != focusBody {
		t.Fatal("enter should move focus into the body")
	}
	if !strings.Contains(stripANSI(m.renderNotes()), "PLAN") {
		t.Error("the pane should still name the note it is showing")
	}

	m, _ = send(t, m, key("esc"))
	if m.notesFocus != focusList {
		t.Error("esc should return focus to the list")
	}
}

func TestThePaneShowsWhateverTheCursorIsOn(t *testing.T) {
	m := modelFor(t, fixture.World{
		Notes: []catalog.Note{
			{Title: "Alpha", BodyMD: "first body"},
			{Title: "Beta", BodyMD: "second body"},
		},
	}, minUsableWidth, 32)
	m.view = viewNotes
	m = m.sync()

	if !strings.Contains(stripANSI(m.renderNotes()), "first body") {
		t.Error("the pane should show the cursor note's body without pressing enter")
	}
	m, _ = send(t, m, key("down"))
	if !strings.Contains(stripANSI(m.renderNotes()), "second body") {
		t.Error("moving the cursor should move the pane with it")
	}
}

func TestAnEmptyBodyPointsAtTheEditor(t *testing.T) {
	m := modelFor(t, fixture.World{Notes: []catalog.Note{{Title: "Blank"}}}, minUsableWidth, 32)
	m.view = viewNotes
	m = m.sync()

	if !strings.Contains(stripANSI(m.renderNotes()), "press e to write") {
		t.Errorf("an empty body should invite the editor:\n%s", stripANSI(m.renderNotes()))
	}
}

func TestNotesFitsEveryTerminalSize(t *testing.T) {
	for _, size := range []struct{ w, h int }{{135, 30}, {150, 40}, {160, 46}, {220, 60}} {
		m := modelFor(t, fixture.Demo(fixture.Period), size.w, size.h)
		m.view = viewNotes
		m = m.sync()

		for i, line := range strings.Split(m.View().Content, "\n") {
			if got := lineWidth(line); got > size.w {
				t.Errorf("%dx%d: line %d is %d columns, want at most %d:\n%s",
					size.w, size.h, i, got, size.w, stripANSI(line))
			}
		}
	}
}

func TestOnlyTheFocusedBlockAccentsItsCursor(t *testing.T) {
	m := modelFor(t, fixture.World{
		Notes: []catalog.Note{{Title: "Plan", BodyMD: mixedNote}},
	}, minUsableWidth, 32)
	m.view = viewNotes
	m = m.sync()

	accent, muted := m.theme.Accent.Render("> "), m.theme.Muted.Render("> ")
	rows, anchors := m.noteRows()
	if !strings.Contains(rows[anchors[0]], accent) {
		t.Error("with focus in the list, the list cursor should be accented")
	}

	m, _ = send(t, m, key("enter"))
	rows, anchors = m.noteRows()
	if !strings.Contains(rows[anchors[0]], muted) {
		t.Error("with focus in the body, the list cursor should go muted")
	}

	// The body's own gutter is accented only where space can act.
	lines := m.noteBodyLines()
	body, _ := m.noteBodyRows(lines)
	want := muted
	if _, ok := lines[m.detail.cursor].ticks(); ok {
		want = accent
	}
	if !strings.Contains(body[m.detail.cursor], want) {
		t.Errorf("body line %d has the wrong gutter for what space does there", m.detail.cursor)
	}
}

// headingRow is the row the list's heading sits on. The list is the half of
// the card that must hold still, so this is what the position tests measure.
// The card's first line of ink is not it: a tall pane starts above the list.
func headingRow(t *testing.T, rendered string) int {
	t.Helper()
	for i, line := range strings.Split(rendered, "\n") {
		if strings.Contains(stripANSI(line), "SEPTEMBER 2026") {
			return i
		}
	}
	t.Fatalf("no heading in:\n%s", stripANSI(rendered))
	return -1
}

func TestTheCardCentersVertically(t *testing.T) {
	m := modelFor(t, fixture.Demo(fixture.Period), 160, 46)
	m.view = viewNotes
	m = m.sync()

	rendered := m.renderNotes()
	above := headingRow(t, rendered)
	below := m.bodyHeight(0) - lipgloss.Height(rendered)
	if above == 0 {
		t.Fatalf("card is glued to the top:\n%s", stripANSI(rendered))
	}
	if diff := above - below; diff > 1 || diff < -1 {
		t.Errorf("slack is %d above and %d below, want it split", above, below)
	}
}

func TestTheCardDoesNotMoveAsTheCursorDoes(t *testing.T) {
	m := modelFor(t, fixture.Demo(fixture.Period), 160, 46)
	m.view = viewNotes
	m = m.sync()

	want := headingRow(t, m.renderNotes())
	for i := range m.shownNotes() {
		m, _ = send(t, m, key("down"))
		if got := headingRow(t, m.renderNotes()); got != want {
			t.Fatalf("after %d moves the card sits %d rows down, want %d", i+1, got, want)
		}
	}
}

func TestABigNoteScrollsInThePane(t *testing.T) {
	// At the floor the demo's long note outruns even a full-height pane.
	m := modelFor(t, fixture.Demo(fixture.Period), minUsableWidth, minUsableHeight)
	m.view = viewNotes
	for i, n := range m.shownNotes() {
		if n.Title == fixture.BigNote {
			m.notesList.cursor = i
		}
	}
	m = m.sync()

	if !scrollHintPattern.MatchString(stripANSI(m.renderNotes())) {
		t.Fatalf("a note past its pane should say how much is below:\n%s",
			stripANSI(m.renderNotes()))
	}

	m, _ = send(t, m, key("enter"))
	last := len(m.noteBodyLines()) - 1
	for range last {
		m, _ = send(t, m, key("down"))
	}
	if m.detail.cursor != last {
		t.Errorf("cursor stopped at %d of %d lines", m.detail.cursor, last)
	}
}

func TestTheListSaysWhenItIsCut(t *testing.T) {
	// More notes than the shortest usable terminal can show at once.
	var notes []catalog.Note
	for i := range 30 {
		notes = append(notes, catalog.Note{
			Title:  fmt.Sprintf("Errand %02d", i),
			Period: september,
		})
	}

	m := modelFor(t, fixture.World{Notes: notes}, minUsableWidth, minUsableHeight)
	m.view = viewNotes
	m = m.sync()

	if content := stripANSI(m.renderNotes()); !scrollHintPattern.MatchString(content) {
		t.Errorf("a cut list should say how much is below it:\n%s", content)
	}
}

func TestALongNoteUsesTheScreenTheListDoesNot(t *testing.T) {
	m := modelFor(t, fixture.Demo(fixture.Period), 160, 46)
	m.view = viewNotes
	m = m.sync()

	shortTop := headingRow(t, m.renderNotes())
	shortPane := lipgloss.Height(m.detail.View())

	for i, n := range m.shownNotes() {
		if n.Title == fixture.BigNote {
			m.notesList.cursor = i
		}
	}
	m = m.sync()

	if got := headingRow(t, m.renderNotes()); got != shortTop {
		t.Errorf("the list moved to %d from %d when a long note was selected", got, shortTop)
	}
	long := lipgloss.Height(m.detail.View())
	if long <= shortPane {
		t.Errorf("pane is %d rows for a long note and %d for a short one; it should grow", long, shortPane)
	}
	// Growing only downward would cap it at the rows under the list's top
	// edge. It has to reach into the empty rows above the list as well.
	if listBlock := lipgloss.Height(m.notesList.View()); long <= listBlock {
		t.Errorf("pane grew to %d rows, no further than the list's %d", long, listBlock)
	}
}
