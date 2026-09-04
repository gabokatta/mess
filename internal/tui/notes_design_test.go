package tui

import (
	"strings"
	"testing"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/fixture"
)

// noteGroups is the one place the list's shape is decided: which notes show,
// under which label, in which order.
func TestNoteGroupsLabelPinnedAndThisMonth(t *testing.T) {
	m := modelFor(t, fixture.World{
		Notes: []catalog.Note{
			{Title: "Groceries", Period: september},
			{Title: "Ideas"},
			{Title: "Next month", Period: september.AddMonths(1)},
			{Title: "Almonds", Period: september},
		},
	}, minUsableWidth, 32)

	groups := m.noteGroups()
	if len(groups) != 2 {
		t.Fatalf("noteGroups() = %d groups, want PINNED and THIS MONTH", len(groups))
	}
	if groups[0].label != "PINNED" || groups[1].label != "THIS MONTH" {
		t.Fatalf("labels = %q, %q; want PINNED then THIS MONTH", groups[0].label, groups[1].label)
	}
	if len(groups[0].notes) != 1 || groups[0].notes[0].Title != "Ideas" {
		t.Errorf("PINNED = %+v, want just Ideas", groups[0].notes)
	}
	if len(groups[1].notes) != 2 {
		t.Fatalf("THIS MONTH = %d notes, want september's two", len(groups[1].notes))
	}
	if groups[1].notes[0].Title != "Almonds" || groups[1].notes[1].Title != "Groceries" {
		t.Errorf("THIS MONTH = %q, %q; want them sorted by title",
			groups[1].notes[0].Title, groups[1].notes[1].Title)
	}
}

// The only checkboxes on this card belong to a note's own task list. A note's
// own state is a word, so the two can never be confused for each other.
func TestNoteRowsCarryStatusInsteadOfACheckbox(t *testing.T) {
	m := modelFor(t, fixture.World{
		Notes: []catalog.Note{
			{Title: "Ideas"},
			{Title: "Renew car insurance", Done: true},
		},
	}, minUsableWidth, 32)
	m.view = viewNotes
	m = m.sync()

	content := stripANSI(m.renderNotes())
	for _, box := range []string{"[ ]", "[x]"} {
		if strings.Contains(content, box) {
			t.Errorf("list still draws %q:\n%s", box, content)
		}
	}
	for _, want := range []string{"open", "done"} {
		if !strings.Contains(content, want) {
			t.Errorf("list is missing the %q status:\n%s", want, content)
		}
	}
}

// Done is the status cell's job alone. Greying the title as well would spend
// the brightness channel restating it.
func TestADoneNoteKeepsAPlainTitle(t *testing.T) {
	m := modelFor(t, fixture.World{
		Notes: []catalog.Note{{Title: "Renew car insurance", Done: true}},
	}, minUsableWidth, 32)
	m.view = viewNotes
	m = m.sync()

	if !strings.Contains(m.renderNotes(), m.theme.Bright.Render("Renew car insurance")) {
		t.Error("a done note's title should render in plain foreground, not muted")
	}
}

// Width wraps what overflows; the scroller's cursor math assumes one line per
// row, so a wrapped title lands the cursor on the wrong note.
func TestLongNoteTitleStaysOnOneLine(t *testing.T) {
	m := modelFor(t, fixture.World{
		Notes: []catalog.Note{
			{Title: "Renegociar el alquiler y revisar las expensas del edificio antes de diciembre"},
			{Title: "Short one"},
		},
	}, minUsableWidth, 32)
	m.view = viewNotes
	m = m.sync()

	rows, anchors := m.noteRows()
	for i, row := range rows {
		if strings.Contains(row, "\n") {
			t.Errorf("row %d spans more than one line:\n%s", i, stripANSI(row))
		}
	}
	if len(anchors) != 2 {
		t.Fatalf("anchors = %d, want one per note", len(anchors))
	}
	if !strings.Contains(stripANSI(rows[anchors[0]]), "…") {
		t.Errorf("a title past the column should end in an ellipsis:\n%s", stripANSI(rows[anchors[0]]))
	}
}

// The column header names every column once, above the scroller, and does not
// repeat inside it.
func TestNoteColumnHeaderRendersOnceAboveTheList(t *testing.T) {
	m := modelFor(t, richWorld(), minUsableWidth, 32)
	m.view = viewNotes
	m = m.sync()

	content := stripANSI(m.renderNotes())
	for _, col := range []string{"NOTE", "PROG", "STATUS"} {
		if got := strings.Count(content, col); got != 1 {
			t.Errorf("%q appears %d times, want exactly 1:\n%s", col, got, content)
		}
	}
}

// Group headers are structural: bold foreground and a muted rule, with no
// palette hue: on this app, colour names a category and nothing else.
func TestNoteGroupHeadersCarryNoPaletteHue(t *testing.T) {
	m := modelFor(t, fixture.World{Notes: []catalog.Note{{Title: "Ideas"}}}, minUsableWidth, 32)
	m.view = viewNotes
	m = m.sync()

	want := m.theme.Title.Render("PINNED") + " " +
		m.theme.Muted.Render(strings.Repeat("─", m.noteListWidth()-len("PINNED")-1))
	if !strings.Contains(m.renderNotes(), want) {
		t.Errorf("PINNED should render bold with a muted rule, got:\n%q", m.renderNotes())
	}
}

// The period the screen shows is named once, as the heading, not repeated as a
// group label two lines under it.
func TestTheShownPeriodIsNamedOnce(t *testing.T) {
	m := modelFor(t, fixture.World{Notes: []catalog.Note{{Title: "Ideas", Period: september}}}, minUsableWidth, 32)
	m.view = viewNotes
	m = m.sync()

	content := stripANSI(m.renderNotes())
	if !strings.Contains(content, "SEPTEMBER 2026") {
		t.Errorf("heading should name the month:\n%s", content)
	}
	if strings.Contains(content, september.String()) {
		t.Errorf("the group label should read THIS MONTH, not %q:\n%s", september.String(), content)
	}
}

func TestNotesMetaCountsWhatIsClosed(t *testing.T) {
	m := modelFor(t, fixture.World{
		Notes: []catalog.Note{
			{Title: "Ideas"},
			{Title: "Renew car insurance", Done: true},
			{Title: "Close out", Period: september},
		},
	}, minUsableWidth, 32)
	m.view = viewNotes
	m = m.sync()

	if !strings.Contains(stripANSI(m.renderNotes()), "done  1 / 3") {
		t.Errorf("meta cluster should read done 1 / 3:\n%s", stripANSI(m.renderNotes()))
	}
}
