package catalog

import (
	"testing"
	"time"

	"github.com/gabokatta/mess/internal/domain"
)

func TestNotePinnedAndStampedRoundTrip(t *testing.T) {
	db := openTestStore(t).DB()
	september := domain.NewPeriod(2026, time.September)

	if _, err := CreateNote(db, Note{Title: "Ideas", BodyMD: "- [ ] buy a lamp"}); err != nil {
		t.Fatalf("CreateNote() unexpected error: %v", err)
	}
	if _, err := CreateNote(db, Note{Title: "Groceries", Period: september}); err != nil {
		t.Fatalf("CreateNote() unexpected error: %v", err)
	}

	got, err := Notes(db)
	if err != nil {
		t.Fatalf("Notes() unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Notes() returned %d rows, want 2", len(got))
	}
	if !got[0].Period.Equal(september) {
		t.Errorf("Groceries period = %v, want 2026-09", got[0].Period)
	}
	if !got[1].Period.IsZero() {
		t.Errorf("Ideas period = %v, want zero (pinned)", got[1].Period)
	}
	if got[1].BodyMD != "- [ ] buy a lamp" {
		t.Errorf("Ideas body = %q, want the markdown it was created with", got[1].BodyMD)
	}
}

func TestSetNotePeriodPinsAndUnpins(t *testing.T) {
	db := openTestStore(t).DB()
	september := domain.NewPeriod(2026, time.September)

	created, err := CreateNote(db, Note{Title: "Ideas", Period: september})
	if err != nil {
		t.Fatalf("CreateNote() unexpected error: %v", err)
	}

	if err := SetNotePeriod(db, created.ID, domain.Period{}); err != nil {
		t.Fatalf("SetNotePeriod() unexpected error: %v", err)
	}
	notes, err := Notes(db)
	if err != nil {
		t.Fatalf("Notes() unexpected error: %v", err)
	}
	if !notes[0].Period.IsZero() {
		t.Errorf("period after unpinning = %v, want zero", notes[0].Period)
	}

	if err := SetNotePeriod(db, created.ID, september); err != nil {
		t.Fatalf("SetNotePeriod() unexpected error: %v", err)
	}
	notes, err = Notes(db)
	if err != nil {
		t.Fatalf("Notes() unexpected error: %v", err)
	}
	if !notes[0].Period.Equal(september) {
		t.Errorf("period after stamping = %v, want 2026-09", notes[0].Period)
	}
}

func TestSetNoteBodyAndDone(t *testing.T) {
	db := openTestStore(t).DB()
	created, err := CreateNote(db, Note{Title: "Ideas"})
	if err != nil {
		t.Fatalf("CreateNote() unexpected error: %v", err)
	}

	if err := SetNoteBody(db, created.ID, "- [x] buy a lamp"); err != nil {
		t.Fatalf("SetNoteBody() unexpected error: %v", err)
	}
	if err := SetNoteDone(db, created.ID, true); err != nil {
		t.Fatalf("SetNoteDone() unexpected error: %v", err)
	}

	notes, err := Notes(db)
	if err != nil {
		t.Fatalf("Notes() unexpected error: %v", err)
	}
	if notes[0].BodyMD != "- [x] buy a lamp" || !notes[0].Done {
		t.Errorf("Notes()[0] = %+v, want the rewritten body and Done", notes[0])
	}
}
