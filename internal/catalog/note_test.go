package catalog_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
)

func TestNotePinnedAndStampedRoundTrip(t *testing.T) {
	db := fixture.DB(t)

	if _, err := catalog.CreateNote(db, catalog.Note{Title: "Ideas", BodyMD: "- [ ] buy a lamp"}); err != nil {
		t.Fatalf("CreateNote() unexpected error: %v", err)
	}
	if _, err := catalog.CreateNote(db, catalog.Note{Title: "Groceries", Period: fixture.Period}); err != nil {
		t.Fatalf("CreateNote() unexpected error: %v", err)
	}

	got, err := catalog.Notes(db)
	if err != nil {
		t.Fatalf("Notes() unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Notes() returned %d rows, want 2", len(got))
	}
	if !got[0].Period.Equal(fixture.Period) {
		t.Errorf("Groceries period = %v, want %v", got[0].Period, fixture.Period)
	}
	if !got[1].Period.IsZero() {
		t.Errorf("Ideas period = %v, want zero (pinned)", got[1].Period)
	}
	if got[1].BodyMD != "- [ ] buy a lamp" {
		t.Errorf("Ideas body = %q, want the markdown it was created with", got[1].BodyMD)
	}
}

func TestSetNotePeriodPinsAndUnpins(t *testing.T) {
	db := fixture.DB(t)
	created := fixture.MustLoad(t, db, fixture.World{
		Notes: []catalog.Note{{Title: "Ideas", Period: fixture.Period}},
	}).Notes["Ideas"]

	if err := catalog.SetNotePeriod(db, created.ID, domain.Period{}); err != nil {
		t.Fatalf("SetNotePeriod() unexpected error: %v", err)
	}
	notes, err := catalog.Notes(db)
	if err != nil {
		t.Fatalf("Notes() unexpected error: %v", err)
	}
	if !notes[0].Period.IsZero() {
		t.Errorf("period after unpinning = %v, want zero", notes[0].Period)
	}

	if err := catalog.SetNotePeriod(db, created.ID, fixture.Period); err != nil {
		t.Fatalf("SetNotePeriod() unexpected error: %v", err)
	}
	notes, err = catalog.Notes(db)
	if err != nil {
		t.Fatalf("Notes() unexpected error: %v", err)
	}
	if !notes[0].Period.Equal(fixture.Period) {
		t.Errorf("period after stamping = %v, want %v", notes[0].Period, fixture.Period)
	}
}

func TestSetNoteBodyAndDone(t *testing.T) {
	db := fixture.DB(t)
	created := fixture.MustLoad(t, db, fixture.World{
		Notes: []catalog.Note{{Title: "Ideas"}},
	}).Notes["Ideas"]

	if err := catalog.SetNoteBody(db, created.ID, "- [x] buy a lamp"); err != nil {
		t.Fatalf("SetNoteBody() unexpected error: %v", err)
	}
	if err := catalog.SetNoteDone(db, created.ID, true); err != nil {
		t.Fatalf("SetNoteDone() unexpected error: %v", err)
	}

	notes, err := catalog.Notes(db)
	if err != nil {
		t.Fatalf("Notes() unexpected error: %v", err)
	}
	want := catalog.Note{ID: created.ID, Title: "Ideas", BodyMD: "- [x] buy a lamp", Done: true}
	if diff := cmp.Diff(want, notes[0]); diff != "" {
		t.Errorf("Notes()[0] mismatch (-want +got):\n%s", diff)
	}
}
