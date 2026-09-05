package backup

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
	"github.com/gabokatta/mess/internal/store"
)

func TestExportDumpsEveryRowAsReadableJSON(t *testing.T) {
	db := fixture.DB(t)
	fixture.MustLoad(t, db, fixture.World{
		Concepts: []fixture.Concept{{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "785000"}},
	})

	data, err := Export(db)
	if err != nil {
		t.Fatalf("Export() unexpected error: %v", err)
	}

	rows, ok := data.Tables["category"]
	if !ok || len(rows) != 1 {
		t.Fatalf("Tables[category] = %+v, want a single row", rows)
	}
	if rows[0]["name"] != "Home" {
		t.Errorf(`Tables["category"][0]["name"] = %v (%T), want "Home" as a plain string`, rows[0]["name"], rows[0]["name"])
	}

	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}
	if !jsonContains(raw, "Home") {
		t.Errorf("marshaled export = %s, want the plain text \"Home\", not a base64 blob", raw)
	}
}

func TestImportWipesAndReloadsFromData(t *testing.T) {
	src := fixture.DB(t)
	fixture.MustLoad(t, src, fixture.World{
		Concepts: []fixture.Concept{{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "785000"}},
	})
	data, err := Export(src)
	if err != nil {
		t.Fatalf("Export() unexpected error: %v", err)
	}

	dst := fixture.DB(t)
	fixture.MustLoad(t, dst, fixture.World{
		Concepts: []fixture.Concept{{Name: "Stale concept that import must remove", Category: "Junk", Kind: catalog.Expense, Base: "1"}},
	})

	if err := Import(dst, data); err != nil {
		t.Fatalf("Import() unexpected error: %v", err)
	}

	got, err := catalog.Concepts(dst)
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Rent" {
		t.Fatalf("Concepts() = %+v, want only the imported Rent row", got)
	}
}

func TestExportImportRoundTripsEveryTable(t *testing.T) {
	src := fixture.DB(t)
	fixture.MustLoad(t, src, fixture.World{
		Concepts: []fixture.Concept{
			{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "785000"},
			{Name: "Wash the house", Category: "Home", Kind: catalog.Chore},
		},
		Entries: []fixture.Entry{
			{Concept: "Rent", Period: fixture.Period, Done: true},
			{Concept: "Wash the house", Period: fixture.Period, Done: true},
		},
		Notes:   []catalog.Note{{Title: "Buy list", BodyMD: "- [x] milk", Period: fixture.Period}},
		Rates:   []fixture.Rate{{Period: fixture.Period, Value: "1000"}},
		FxHouse: domain.MEP,
	})

	data, err := Export(src)
	if err != nil {
		t.Fatalf("Export() unexpected error: %v", err)
	}

	dst := fixture.DB(t)
	if err := Import(dst, data); err != nil {
		t.Fatalf("Import() unexpected error: %v", err)
	}

	wantCategories, err := catalog.Categories(src)
	if err != nil {
		t.Fatalf("Categories(src) unexpected error: %v", err)
	}
	gotCategories, err := catalog.Categories(dst)
	if err != nil {
		t.Fatalf("Categories(dst) unexpected error: %v", err)
	}
	if diff := cmp.Diff(wantCategories, gotCategories); diff != "" {
		t.Errorf("Categories() mismatch (-want +got):\n%s", diff)
	}

	wantConcepts, err := catalog.Concepts(src)
	if err != nil {
		t.Fatalf("Concepts(src) unexpected error: %v", err)
	}
	gotConcepts, err := catalog.Concepts(dst)
	if err != nil {
		t.Fatalf("Concepts(dst) unexpected error: %v", err)
	}
	if diff := cmp.Diff(wantConcepts, gotConcepts); diff != "" {
		t.Errorf("Concepts() mismatch (-want +got):\n%s", diff)
	}

	wantEntries, err := catalog.MonthEntries(src, fixture.Period)
	if err != nil {
		t.Fatalf("MonthEntries(src) unexpected error: %v", err)
	}
	gotEntries, err := catalog.MonthEntries(dst, fixture.Period)
	if err != nil {
		t.Fatalf("MonthEntries(dst) unexpected error: %v", err)
	}
	if diff := cmp.Diff(wantEntries, gotEntries); diff != "" {
		t.Errorf("MonthEntries() mismatch (-want +got):\n%s", diff)
	}

	wantNotes, err := catalog.Notes(src)
	if err != nil {
		t.Fatalf("Notes(src) unexpected error: %v", err)
	}
	gotNotes, err := catalog.Notes(dst)
	if err != nil {
		t.Fatalf("Notes(dst) unexpected error: %v", err)
	}
	if diff := cmp.Diff(wantNotes, gotNotes); diff != "" {
		t.Errorf("Notes() mismatch (-want +got):\n%s", diff)
	}

	wantRates, err := catalog.FxRates(src)
	if err != nil {
		t.Fatalf("FxRates(src) unexpected error: %v", err)
	}
	gotRates, err := catalog.FxRates(dst)
	if err != nil {
		t.Fatalf("FxRates(dst) unexpected error: %v", err)
	}
	if diff := cmp.Diff(wantRates, gotRates); diff != "" {
		t.Errorf("FxRates() mismatch (-want +got):\n%s", diff)
	}

	wantHouse, err := catalog.FxHouse(src)
	if err != nil {
		t.Fatalf("FxHouse(src) unexpected error: %v", err)
	}
	gotHouse, err := catalog.FxHouse(dst)
	if err != nil {
		t.Fatalf("FxHouse(dst) unexpected error: %v", err)
	}
	if gotHouse != wantHouse {
		t.Errorf("FxHouse() = %v, want %v", gotHouse, wantHouse)
	}
}

func TestSnapshotWritesARestorableCopyBesideTheOriginal(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "mess.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() unexpected error: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if _, err := catalog.CreateCategory(s.DB(), "Home", 0, 0); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}

	snapshotPath, err := Snapshot(s.DB(), dbPath)
	if err != nil {
		t.Fatalf("Snapshot() unexpected error: %v", err)
	}
	if snapshotPath == dbPath {
		t.Fatalf("Snapshot() path = %q, want a different path than the original", snapshotPath)
	}

	restored, err := store.Open(snapshotPath)
	if err != nil {
		t.Fatalf("store.Open(snapshot) unexpected error: %v", err)
	}
	defer restored.Close()

	got, err := catalog.Categories(restored.DB())
	if err != nil {
		t.Fatalf("Categories() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Home" {
		t.Errorf("Categories() from the snapshot = %+v, want the Home row captured before import", got)
	}
}

func TestSnapshotPathsNeverCollide(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "mess.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() unexpected error: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	first, err := Snapshot(s.DB(), dbPath)
	if err != nil {
		t.Fatalf("Snapshot() unexpected error: %v", err)
	}
	second, err := Snapshot(s.DB(), dbPath)
	if err != nil {
		t.Fatalf("Snapshot() unexpected error: %v", err)
	}
	if first == second {
		t.Errorf("Snapshot() twice in immediate succession = %q both times, want distinct paths", first)
	}
}

func jsonContains(raw []byte, s string) bool {
	for i := 0; i+len(s) <= len(raw); i++ {
		if string(raw[i:i+len(s)]) == s {
			return true
		}
	}
	return false
}
