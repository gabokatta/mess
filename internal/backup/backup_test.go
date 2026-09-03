package backup

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "mess.db"))
	if err != nil {
		t.Fatalf("store.Open() unexpected error: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestExportDumpsEveryRowAsReadableJSON(t *testing.T) {
	s := openTestStore(t)
	if _, err := catalog.CreateCategory(s.DB(), "Home", 1); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}

	data, err := Export(s.DB())
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
	src := openTestStore(t)
	if _, err := catalog.CreateCategory(src.DB(), "Home", 1); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	data, err := Export(src.DB())
	if err != nil {
		t.Fatalf("Export() unexpected error: %v", err)
	}

	dst := openTestStore(t)
	if _, err := catalog.CreateCategory(dst.DB(), "Stale row that import must remove", 0); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}

	if err := Import(dst.DB(), data); err != nil {
		t.Fatalf("Import() unexpected error: %v", err)
	}

	got, err := catalog.Categories(dst.DB())
	if err != nil {
		t.Fatalf("Categories() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Home" {
		t.Fatalf("Categories() = %+v, want only the imported Home row", got)
	}
}

func TestExportImportRoundTripsEveryTable(t *testing.T) {
	src := openTestStore(t)
	db := src.DB()
	period := domain.NewPeriod(2026, time.January)

	cat, err := catalog.CreateCategory(db, "Home", 1)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	rent, err := catalog.CreateConcept(db, catalog.Concept{
		Name: "Rent", CategoryID: cat.ID, Kind: catalog.Expense,
		Money:     &catalog.MoneyDetails{Currency: domain.ARS, Base: decimal.NewFromInt(785000)},
		MonthMask: domain.Monthly, ActiveFrom: period,
	})
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}
	if err := catalog.SetMonthEntryDone(db, rent.ID, period, true); err != nil {
		t.Fatalf("SetMonthEntryDone() unexpected error: %v", err)
	}
	chore, err := catalog.CreateConcept(db, catalog.Concept{
		Name: "Wash the house", CategoryID: cat.ID, Kind: catalog.Chore,
		MonthMask: domain.Monthly, ActiveFrom: period,
	})
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}
	if err := catalog.SetMonthEntryDone(db, chore.ID, period, true); err != nil {
		t.Fatalf("SetMonthEntryDone() unexpected error: %v", err)
	}
	if _, err := catalog.CreateNote(db, catalog.Note{Title: "Buy list", BodyMD: "- [x] milk", Period: period}); err != nil {
		t.Fatalf("CreateNote() unexpected error: %v", err)
	}
	if err := catalog.SaveFxClose(db, period, decimal.NewFromInt(1000)); err != nil {
		t.Fatalf("SaveFxClose() unexpected error: %v", err)
	}
	if err := catalog.SetFxHouse(db, domain.MEP); err != nil {
		t.Fatalf("SetFxHouse() unexpected error: %v", err)
	}

	data, err := Export(db)
	if err != nil {
		t.Fatalf("Export() unexpected error: %v", err)
	}

	dst := openTestStore(t)
	if err := Import(dst.DB(), data); err != nil {
		t.Fatalf("Import() unexpected error: %v", err)
	}

	gotCats, err := catalog.Categories(dst.DB())
	if err != nil {
		t.Fatalf("Categories() unexpected error: %v", err)
	}
	if len(gotCats) != 1 || gotCats[0].Name != "Home" {
		t.Errorf("Categories() = %+v, want the imported Home row", gotCats)
	}

	gotConcepts, err := catalog.Concepts(dst.DB())
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	if len(gotConcepts) != 2 {
		t.Fatalf("Concepts() = %+v, want the money concept and the chore both imported", gotConcepts)
	}
	for _, c := range gotConcepts {
		if c.Name == "Rent" && !c.Money.Base.Equal(decimal.NewFromInt(785000)) {
			t.Errorf("Rent base = %s, want 785000", c.Money.Base)
		}
		if c.Name == "Wash the house" && c.Money != nil {
			t.Errorf("chore Money = %+v, want nil", c.Money)
		}
	}

	gotEntries, err := catalog.MonthEntries(dst.DB(), period)
	if err != nil {
		t.Fatalf("MonthEntries() unexpected error: %v", err)
	}
	if len(gotEntries) != 2 {
		t.Fatalf("MonthEntries() = %+v, want both entries imported", gotEntries)
	}

	gotNotes, err := catalog.Notes(dst.DB())
	if err != nil {
		t.Fatalf("Notes() unexpected error: %v", err)
	}
	if len(gotNotes) != 1 || gotNotes[0].BodyMD != "- [x] milk" || !gotNotes[0].Period.Equal(period) {
		t.Errorf("Notes() = %+v, want the imported Buy list", gotNotes)
	}

	gotRates, err := catalog.FxRates(dst.DB())
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	if len(gotRates) != 1 || !gotRates[0].Value.Equal(decimal.NewFromInt(1000)) || gotRates[0].Source != catalog.Close {
		t.Errorf("FxRates() = %+v, want the imported 1000 close", gotRates)
	}

	gotHouse, err := catalog.FxHouse(dst.DB())
	if err != nil {
		t.Fatalf("FxHouse() unexpected error: %v", err)
	}
	if gotHouse != domain.MEP {
		t.Errorf("FxHouse() = %v, want the imported MEP setting", gotHouse)
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
	if _, err := catalog.CreateCategory(s.DB(), "Home", 0); err != nil {
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
