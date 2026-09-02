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
	if _, err := catalog.CreateCategory(s.DB(), "Servicios", 1); err != nil {
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
	if rows[0]["name"] != "Servicios" {
		t.Errorf(`Tables["category"][0]["name"] = %v (%T), want "Servicios" as a plain string`, rows[0]["name"], rows[0]["name"])
	}

	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}
	if !jsonContains(raw, "Servicios") {
		t.Errorf("marshaled export = %s, want the plain text \"Servicios\", not a base64 blob", raw)
	}
}

func TestImportWipesAndReloadsFromData(t *testing.T) {
	src := openTestStore(t)
	if _, err := catalog.CreateCategory(src.DB(), "Servicios", 1); err != nil {
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
	if len(got) != 1 || got[0].Name != "Servicios" {
		t.Fatalf("Categories() = %+v, want only the imported Servicios row", got)
	}
}

func TestExportImportRoundTripsEveryTable(t *testing.T) {
	src := openTestStore(t)
	db := src.DB()
	period := domain.NewPeriod(2026, time.January)

	cat, err := catalog.CreateCategory(db, "Servicios", 1)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	concept, err := catalog.CreateConcept(db, catalog.Concept{
		Name: "Alquiler", CategoryID: cat.ID, Kind: catalog.Expense,
		Money: &catalog.MoneyDetails{Currency: domain.ARS}, MonthMask: domain.Monthly, ActiveFrom: period,
	})
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}
	if err := catalog.SetBaseAmount(db, concept.ID, period, decimal.NewFromInt(785000)); err != nil {
		t.Fatalf("SetBaseAmount() unexpected error: %v", err)
	}
	if err := catalog.SetMonthEntryDone(db, concept.ID, period, true); err != nil {
		t.Fatalf("SetMonthEntryDone() unexpected error: %v", err)
	}
	chore, err := catalog.CreateConcept(db, catalog.Concept{
		Name: "Sacar la basura", CategoryID: cat.ID, Kind: catalog.Chore, MonthMask: domain.Monthly, ActiveFrom: period,
	})
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}
	if err := catalog.SetMonthEntryDone(db, chore.ID, period, true); err != nil {
		t.Fatalf("SetMonthEntryDone() unexpected error: %v", err)
	}
	if _, err := catalog.CreateSavingAllocation(db, catalog.SavingAllocation{
		Period: period, Destination: catalog.Invested, Amount: decimal.NewFromInt(100), Currency: domain.USD,
	}); err != nil {
		t.Fatalf("CreateSavingAllocation() unexpected error: %v", err)
	}
	if _, err := catalog.CreateList(db, catalog.List{Name: "Buy list", BodyMD: "- [x] milk", Period: period}); err != nil {
		t.Fatalf("CreateList() unexpected error: %v", err)
	}
	if err := catalog.SetFxRate(db, period, decimal.NewFromInt(1000)); err != nil {
		t.Fatalf("SetFxRate() unexpected error: %v", err)
	}
	if err := catalog.SaveSettings(db, catalog.Settings{
		FxHouse: domain.Blue,
		Opening: catalog.OpeningBalances{Period: period},
	}); err != nil {
		t.Fatalf("SaveSettings() unexpected error: %v", err)
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
	if len(gotCats) != 1 || gotCats[0].Name != "Servicios" {
		t.Errorf("Categories() = %+v, want the imported Servicios row", gotCats)
	}

	gotConcepts, err := catalog.Concepts(dst.DB())
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	if len(gotConcepts) != 2 {
		t.Fatalf("Concepts() = %+v, want the money concept and the chore both imported", gotConcepts)
	}

	gotBases, err := catalog.AllBaseAmounts(dst.DB())
	if err != nil {
		t.Fatalf("AllBaseAmounts() unexpected error: %v", err)
	}
	if bases := gotBases[concept.ID]; len(bases) != 1 || !bases[0].Amount.Equal(decimal.NewFromInt(785000)) {
		t.Errorf("AllBaseAmounts() = %+v, want a 785000 base for concept %d", gotBases, concept.ID)
	}

	gotEntries, err := catalog.MonthEntries(dst.DB(), period)
	if err != nil {
		t.Fatalf("MonthEntries() unexpected error: %v", err)
	}
	if len(gotEntries) != 2 {
		t.Fatalf("MonthEntries() = %+v, want both the money entry and the chore entry imported", gotEntries)
	}

	gotAllocations, err := catalog.AllSavingAllocations(dst.DB())
	if err != nil {
		t.Fatalf("AllSavingAllocations() unexpected error: %v", err)
	}
	if len(gotAllocations) != 1 || gotAllocations[0].Destination != catalog.Invested {
		t.Errorf("AllSavingAllocations() = %+v, want the imported Invested allocation", gotAllocations)
	}

	gotLists, err := catalog.Lists(dst.DB())
	if err != nil {
		t.Fatalf("Lists() unexpected error: %v", err)
	}
	if len(gotLists) != 1 || gotLists[0].BodyMD != "- [x] milk" {
		t.Errorf("Lists() = %+v, want the imported Buy list body", gotLists)
	}

	gotRates, err := catalog.FxRates(dst.DB())
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	if len(gotRates) != 1 || !gotRates[0].Value.Equal(decimal.NewFromInt(1000)) {
		t.Errorf("FxRates() = %+v, want the imported 1000 rate", gotRates)
	}

	gotHouse, err := catalog.FxHouse(dst.DB())
	if err != nil {
		t.Fatalf("FxHouse() unexpected error: %v", err)
	}
	if gotHouse != domain.Blue {
		t.Errorf("FxHouse() = %v, want the imported Blue setting", gotHouse)
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
	if _, err := catalog.CreateCategory(s.DB(), "Servicios", 0); err != nil {
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
	if len(got) != 1 || got[0].Name != "Servicios" {
		t.Errorf("Categories() from the snapshot = %+v, want the Servicios row captured before import", got)
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
