package tui

import (
	"strings"
	"testing"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/fixture"
)

func carriedWorld() fixture.World {
	return fixture.World{
		Notes: []catalog.Note{
			{Title: "Pinned thing"},
			{Title: "This month", Period: september},
			{Title: "Cancel the gym", Period: september.AddMonths(-1)},
			{Title: "Chase the deposit", Period: september.AddMonths(-3)},
			{Title: "Already handled", Period: september.AddMonths(-2), Done: true},
		},
	}
}

func TestCarriedOverHoldsOpenNotesFromEarlierPeriods(t *testing.T) {
	m := modelFor(t, carriedWorld(), minUsableWidth, 32)

	groups := m.noteGroups()
	if len(groups) != 3 {
		t.Fatalf("noteGroups() = %d groups, want PINNED, THIS MONTH and CARRIED OVER", len(groups))
	}
	if groups[2].label != "CARRIED OVER" {
		t.Fatalf("third label = %q, want CARRIED OVER", groups[2].label)
	}
	if len(groups[2].notes) != 2 {
		t.Fatalf("CARRIED OVER = %d notes, want the two still open", len(groups[2].notes))
	}
	// Oldest first: the thing avoided longest leads the block.
	if groups[2].notes[0].Title != "Chase the deposit" || groups[2].notes[1].Title != "Cancel the gym" {
		t.Errorf("CARRIED OVER = %q, %q; want June's before August's",
			groups[2].notes[0].Title, groups[2].notes[1].Title)
	}
}

func TestCarriedOverIsComputedAgainstTheShownPeriod(t *testing.T) {
	m := modelFor(t, carriedWorld(), minUsableWidth, 32)
	m.period = september.AddMonths(-2)

	groups := m.noteGroups()
	if len(groups) != 3 {
		t.Fatalf("noteGroups() = %d groups, want three", len(groups))
	}
	if len(groups[2].notes) != 1 || groups[2].notes[0].Title != "Chase the deposit" {
		t.Errorf("CARRIED OVER two months back = %+v, want only the June note", groups[2].notes)
	}
	if len(groups[1].notes) != 1 || groups[1].notes[0].Title != "Already handled" {
		t.Errorf("THIS MONTH two months back = %+v, want the note filed to it", groups[1].notes)
	}
}

func TestAQuietMonthCarriesNoGroupAndNoCount(t *testing.T) {
	m := modelFor(t, fixture.World{Notes: []catalog.Note{{Title: "Ideas"}}}, minUsableWidth, 32)
	m.view = viewNotes
	m = m.sync()

	for _, g := range m.noteGroups() {
		if g.label == "CARRIED OVER" {
			t.Error("a month with nothing carried should render no CARRIED OVER group")
		}
	}
	if strings.Contains(stripANSI(m.renderNotes()), "carried") {
		t.Errorf("a clean month should carry no warning:\n%s", stripANSI(m.renderNotes()))
	}
}

func TestClosingACarriedNoteRemovesIt(t *testing.T) {
	m := modelFor(t, carriedWorld(), minUsableWidth, 32)
	m.view = viewNotes
	m = m.sync()

	if !strings.Contains(stripANSI(m.renderNotes()), "carried  2") {
		t.Fatalf("meta cluster should count the debt:\n%s", stripANSI(m.renderNotes()))
	}

	shown := m.shownNotes()
	for i, n := range shown {
		if n.Title == "Chase the deposit" {
			m.notesList.cursor = i
		}
	}
	_, cmd := send(t, m, key("space"))
	if err := runWrite(t, cmd); err != nil {
		t.Fatalf("closing reported an error: %v", err)
	}

	notes, _ := catalog.Notes(m.db)
	m, _ = send(t, m, notesMsg{notes: notes})
	for _, n := range m.shownNotes() {
		if n.Title == "Chase the deposit" {
			t.Error("a closed note should leave CARRIED OVER")
		}
	}
	if !strings.Contains(stripANSI(m.renderNotes()), "carried  1") {
		t.Errorf("the carried count should drop with it:\n%s", stripANSI(m.renderNotes()))
	}

	// Nothing is lost: it is still there in the month it belongs to.
	m.period = september.AddMonths(-3)
	m = m.sync()
	content := stripANSI(m.renderNotes())
	if !strings.Contains(content, "Chase the deposit") || !strings.Contains(content, "done") {
		t.Errorf("the note should read done in its own month:\n%s", content)
	}
}

func TestOnlyCarriedRowsCarryTheirPeriod(t *testing.T) {
	m := modelFor(t, carriedWorld(), minUsableWidth, 32)
	m.view = viewNotes
	m = m.sync()

	content := stripANSI(m.renderNotes())
	if !strings.Contains(content, "FROM") {
		t.Errorf("the column header should name the origin column:\n%s", content)
	}
	rows, anchors := m.noteRows()
	for i, n := range m.shownNotes() {
		row := stripANSI(rows[anchors[i]])
		carried := n.Period.Before(m.period) && !n.Period.IsZero()
		if got := strings.Contains(row, n.Period.String()); got != carried {
			t.Errorf("%q shows its period = %v, want %v:\n%s", n.Title, got, carried, row)
		}
	}
}

func TestTheDemoWorldCarriesSomething(t *testing.T) {
	m := modelFor(t, fixture.Demo(fixture.Period), minUsableWidth, 32)
	m.view = viewNotes
	m = m.sync()

	groups := m.noteGroups()
	if len(groups) != 3 || len(groups[2].notes) != 2 {
		t.Fatalf("demo CARRIED OVER = %d notes, want two open ones from two months", len(groups[2].notes))
	}
	if groups[2].notes[0].Period.Equal(groups[2].notes[1].Period) {
		t.Error("the two carried demo notes should come from different months")
	}
	for _, n := range groups[2].notes {
		if n.Done {
			t.Errorf("%q is closed and should not be carried", n.Title)
		}
	}
}
