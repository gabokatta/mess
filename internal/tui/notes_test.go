package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/fixture"
)

// Pinned notes lead, then the shown period's, under one cursor.
func TestNotesListPinnedFirst(t *testing.T) {
	m := modelFor(t, fixture.World{
		Notes: []catalog.Note{
			{Title: "Groceries", Period: september},
			{Title: "Ideas"},
			{Title: "Next month", Period: september.AddMonths(1)},
		},
	}, 90, 30)
	m.view = viewNotes

	shown := m.shownNotes()
	if len(shown) != 2 {
		t.Fatalf("shownNotes() = %d notes, want the pinned one and september's", len(shown))
	}
	if shown[0].Title != "Ideas" || shown[1].Title != "Groceries" {
		t.Errorf("shownNotes() = %q, %q; want Ideas then Groceries", shown[0].Title, shown[1].Title)
	}

	m, _ = send(t, m, key("down"))
	if got, _ := m.cursorNote(); got.Title != "Groceries" {
		t.Errorf("cursor after down = %q, want Groceries", got.Title)
	}
}

func TestNotesDoneToggle(t *testing.T) {
	m := modelFor(t, fixture.World{Notes: []catalog.Note{{Title: "Ideas"}}}, 90, 30)
	m.view = viewNotes

	_, cmd := send(t, m, key("c"))
	if err := runWrite(t, cmd); err != nil {
		t.Fatalf("toggling done reported an error: %v", err)
	}

	notes, err := catalog.Notes(m.db)
	if err != nil {
		t.Fatalf("Notes() unexpected error: %v", err)
	}
	if !notes[0].Done {
		t.Errorf("note = %+v, want it done", notes[0])
	}
}

func TestNotesPinAndUnpin(t *testing.T) {
	m := modelFor(t, fixture.World{Notes: []catalog.Note{{Title: "Ideas"}}}, 90, 30)
	m.view = viewNotes

	_, cmd := send(t, m, key("p"))
	if err := runWrite(t, cmd); err != nil {
		t.Fatalf("pinning reported an error: %v", err)
	}
	notes, _ := catalog.Notes(m.db)
	if !notes[0].Period.Equal(september) {
		t.Fatalf("period after p on a pinned note = %v, want september", notes[0].Period)
	}

	m, _ = send(t, m, notesMsg{notes: notes})
	_, cmd = send(t, m, key("p"))
	if err := runWrite(t, cmd); err != nil {
		t.Fatalf("unpinning reported an error: %v", err)
	}
	notes, _ = catalog.Notes(m.db)
	if !notes[0].Period.IsZero() {
		t.Errorf("period after p on a stamped note = %v, want zero (pinned)", notes[0].Period)
	}
}

func TestNoteDetailOpensAndCloses(t *testing.T) {
	m := modelFor(t, fixture.World{
		Notes: []catalog.Note{{Title: "Ideas", BodyMD: "- [ ] buy a lamp"}},
	}, 90, 30)
	m.view = viewNotes

	m, _ = send(t, m, key("enter"))
	if m.openNote == nil || m.openNote.Title != "Ideas" {
		t.Fatalf("openNote = %+v, want Ideas", m.openNote)
	}

	m, _ = send(t, m, key("esc"))
	if m.openNote != nil {
		t.Error("esc should return to the list")
	}
}

// space rewrites the line under the cursor in the source, so storage stays
// one text field.
func TestNoteDetailSpaceTogglesTheCheckboxUnderTheCursor(t *testing.T) {
	m := modelFor(t, fixture.World{
		Notes: []catalog.Note{{Title: "Ideas", BodyMD: "- [ ] milk\n- [ ] bread"}},
	}, 90, 30)
	m.view = viewNotes

	m, _ = send(t, m, key("enter"), key("down"))
	_, cmd := send(t, m, key("space"))
	if err := runWrite(t, cmd); err != nil {
		t.Fatalf("toggling reported an error: %v", err)
	}

	notes, _ := catalog.Notes(m.db)
	if notes[0].BodyMD != "- [ ] milk\n- [x] bread" {
		t.Errorf("body = %q, want only the second box ticked", notes[0].BodyMD)
	}
}

func TestNoteEditorGuardsAModifiedBody(t *testing.T) {
	m := modelFor(t, fixture.World{Notes: []catalog.Note{{Title: "Ideas", BodyMD: "- [ ] milk"}}}, 90, 30)
	m.view = viewNotes

	m, _ = send(t, m, key("enter"), key("e"))
	editor, ok := m.modal.(*noteEditor)
	if !ok {
		t.Fatalf("modal = %T, want *noteEditor", m.modal)
	}

	m, _ = send(t, m, key("esc"))
	if m.modal != nil {
		t.Error("esc on an unchanged body should close the editor outright")
	}

	m, _ = send(t, m, key("e"))
	editor = m.modal.(*noteEditor)
	editor.area.SetValue("- [x] milk")

	m, _ = send(t, m, key("esc"))
	if m.modal == nil {
		t.Fatal("esc on a changed body should ask before discarding it")
	}
	if !strings.Contains(m.modal.Help(), "discard") {
		t.Errorf("help = %q, want the discard prompt", m.modal.Help())
	}

	m, _ = send(t, m, key("n"))
	if m.modal == nil {
		t.Error("n should keep editing")
	}
	m, _ = send(t, m, key("esc"), key("y"))
	if m.modal != nil {
		t.Error("y should discard and close")
	}
}

func TestNoteEditorSavesOnCtrlS(t *testing.T) {
	m := modelFor(t, fixture.World{Notes: []catalog.Note{{Title: "Ideas", BodyMD: "- [ ] milk"}}}, 90, 30)
	m.view = viewNotes

	m, _ = send(t, m, key("enter"), key("e"))
	m.modal.(*noteEditor).area.SetValue("- [x] milk\n- [ ] bread")

	m, cmd := send(t, m, tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if m.modal != nil {
		t.Error("ctrl+s should save and return")
	}
	if err := runWrite(t, cmd); err != nil {
		t.Fatalf("saving reported an error: %v", err)
	}

	notes, _ := catalog.Notes(m.db)
	if notes[0].BodyMD != "- [x] milk\n- [ ] bread" {
		t.Errorf("body = %q, want the edited markdown", notes[0].BodyMD)
	}
}

func TestNewNoteFormCreatesInTheShownPeriod(t *testing.T) {
	m := modelFor(t, fixture.World{}, 90, 30)
	m.view = viewNotes

	m, cmd := send(t, m, key("n"))
	if _, ok := m.modal.(*form); !ok {
		t.Fatalf("modal = %T, want *form", m.modal)
	}
	m, _ = pump(t, m, cmd)

	for _, r := range "Ideas" {
		m, cmd = send(t, m, tea.KeyPressMsg{Code: r, Text: string(r)})
		m, _ = pump(t, m, cmd)
	}

	m, cmd = send(t, m, key("enter"))
	m, writes := pump(t, m, cmd)
	if m.modal != nil {
		t.Fatal("completing the form should close it")
	}
	if len(writes) != 1 || writes[0].err != nil {
		t.Fatalf("form completion writes = %+v, want one clean write", writes)
	}

	notes, _ := catalog.Notes(m.db)
	if len(notes) != 1 || notes[0].Title != "Ideas" || !notes[0].Period.Equal(september) {
		t.Errorf("Notes() = %+v, want one Ideas note stamped to september", notes)
	}
}

// Reading a note, the arrows walk its checkboxes; they must not move the
// month out from under it.
func TestArrowsAreInertWhileReadingANote(t *testing.T) {
	m := modelFor(t, fixture.World{Notes: []catalog.Note{{Title: "Ideas", BodyMD: "- [ ] milk"}}}, 90, 30)
	m.view = viewNotes

	m, _ = send(t, m, key("enter"))
	before := m.period

	m, _ = send(t, m, key("right"), key("left"))
	if !m.period.Equal(before) {
		t.Errorf("period moved to %s while a note was open", m.period)
	}
	if m.openNote == nil {
		t.Error("the note should still be open")
	}
}

func TestSwitchingViewsClosesTheOpenNote(t *testing.T) {
	m := modelFor(t, fixture.World{Notes: []catalog.Note{{Title: "Ideas"}}}, 90, 30)
	m.view = viewNotes

	m, _ = send(t, m, key("enter"))
	if m.openNote == nil {
		t.Fatal("enter should open the note")
	}
	m, _ = send(t, m, key("tab"))
	if m.openNote != nil {
		t.Error("leaving Notes should close the open note")
	}
}

// A "[ ] " sitting inside prose is not a checkbox: counting it would shift
// every toggle after it onto the wrong line.
func TestNoteDetailIgnoresCheckboxGlyphsInProse(t *testing.T) {
	m := modelFor(t, fixture.World{
		Notes: []catalog.Note{{
			Title:  "Ideas",
			BodyMD: "Write a [ ] to make a checkbox.\n\n- [ ] milk\n- [ ] bread\n",
		}},
	}, 90, 30)
	m.view = viewNotes
	m, _ = send(t, m, key("enter"), key("down"))

	_, anchors := m.noteDetailRows()
	if len(anchors) != 2 {
		t.Fatalf("anchors = %v, want one per real checkbox", anchors)
	}

	_, cmd := send(t, m, key("space"))
	if err := runWrite(t, cmd); err != nil {
		t.Fatalf("toggling reported an error: %v", err)
	}
	notes, _ := catalog.Notes(m.db)
	if notes[0].BodyMD != "Write a [ ] to make a checkbox.\n\n- [ ] milk\n- [x] bread\n" {
		t.Errorf("body = %q, want only bread ticked", notes[0].BodyMD)
	}
}

// A form or textarea on screen has to follow the terminal, not keep the size
// it was built with.
func TestOpenModalFollowsAResize(t *testing.T) {
	m := modelFor(t, fixture.World{Notes: []catalog.Note{{Title: "Ideas", BodyMD: "- [ ] milk"}}}, 90, 30)
	m.view = viewNotes

	m, _ = send(t, m, key("enter"), key("e"))
	before := m.modal.(*noteEditor).area.Width()

	m, _ = send(t, m, tea.WindowSizeMsg{Width: 40, Height: 20})
	if after := m.modal.(*noteEditor).area.Width(); after == before {
		t.Errorf("editor width stayed at %d through a resize", after)
	}
}
