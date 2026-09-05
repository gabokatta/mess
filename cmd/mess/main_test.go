package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
	"github.com/gabokatta/mess/internal/month"
	"github.com/gabokatta/mess/internal/store"
	"github.com/gabokatta/mess/internal/testutil"
)

func TestExportImportRoundTripsThroughTheCLI(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.db")
	dstPath := filepath.Join(dir, "dst.db")

	src, err := store.Open(srcPath)
	if err != nil {
		t.Fatalf("store.Open() unexpected error: %v", err)
	}
	testutil.MustLoad(t, src.DB(), fixture.World{
		Concepts: []fixture.Concept{{Name: "Rent", Category: "Utilities", Kind: catalog.Expense, Base: "1"}},
	})
	if err := src.Close(); err != nil {
		t.Fatalf("Close() unexpected error: %v", err)
	}

	var out bytes.Buffer
	if err := run([]string{"export", "--db", srcPath}, &out); err != nil {
		t.Fatalf("run(export) unexpected error: %v", err)
	}

	backupFile := filepath.Join(dir, "backup.json")
	if err := os.WriteFile(backupFile, out.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile() unexpected error: %v", err)
	}

	if err := runImport([]string{"--db", dstPath, backupFile}, &bytes.Buffer{}, replace); err != nil {
		t.Fatalf("runImport() unexpected error: %v", err)
	}

	dst, err := store.Open(dstPath)
	if err != nil {
		t.Fatalf("store.Open(dst) unexpected error: %v", err)
	}
	defer dst.Close()

	got, err := catalog.Categories(dst.DB())
	if err != nil {
		t.Fatalf("Categories() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Utilities" {
		t.Errorf("Categories() = %+v, want the Utilities row round-tripped through export/import", got)
	}
}

func TestImportRequiresExactlyOneFileArgument(t *testing.T) {
	dir := t.TempDir()
	err := runImport([]string{"--db", filepath.Join(dir, "mess.db")}, &bytes.Buffer{}, replace)
	if err == nil {
		t.Error("runImport() with no file argument should fail")
	}
}

func TestInvalidFlagsReturnAnError(t *testing.T) {
	if err := run([]string{"export", "--unknown"}, &bytes.Buffer{}); err == nil {
		t.Fatal("run(export) accepted an unknown flag")
	}
}

func TestImportCancelledLeavesTheDatabaseAlone(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mess.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() unexpected error: %v", err)
	}
	testutil.MustLoad(t, db.DB(), fixture.World{
		Concepts: []fixture.Concept{{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "785000"}},
	})
	if err := db.Close(); err != nil {
		t.Fatalf("Close() unexpected error: %v", err)
	}

	empty := filepath.Join(filepath.Dir(dbPath), "empty.json")
	if err := os.WriteFile(empty, []byte(`{"tables":{}}`), 0o644); err != nil {
		t.Fatalf("WriteFile() unexpected error: %v", err)
	}

	if err := runImport([]string{"--db", dbPath, empty}, &bytes.Buffer{}, cancel); err != nil {
		t.Fatalf("runImport() unexpected error: %v", err)
	}

	reopened, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() unexpected error: %v", err)
	}
	defer reopened.Close()

	got, err := catalog.Concepts(reopened.DB())
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Rent" {
		t.Errorf("Concepts() = %+v, want the row a cancelled import never touched", got)
	}
}

func TestSeedConvergesOnTheSameRowsEveryRun(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mess.db")

	var out bytes.Buffer
	if err := run([]string{"seed", "--db", dbPath}, &out); err != nil {
		t.Fatalf("run(seed) unexpected error: %v", err)
	}
	if out.Len() == 0 {
		t.Error("run(seed) printed no summary")
	}
	first := readCatalog(t, dbPath)

	out.Reset()
	if err := run([]string{"seed", "--db", dbPath}, &out); err != nil {
		t.Fatalf("run(seed) second run unexpected error: %v", err)
	}
	second := readCatalog(t, dbPath)

	if diff := cmp.Diff(first, second); diff != "" {
		t.Errorf("seed is not idempotent (-first +second):\n%s", diff)
	}
}

func TestSeedRespectsTheDatabaseLock(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mess.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() unexpected error: %v", err)
	}
	defer s.Close()
	if _, err := catalog.CreateCategory(s.DB(), "Keep", 0, 0); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"seed", "--db", dbPath}, &bytes.Buffer{}); err == nil {
		t.Fatal("seed accepted an open database")
	}
	categories, err := catalog.Categories(s.DB())
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) != 1 || categories[0].Name != "Keep" {
		t.Fatalf("seed changed the open database: %+v", categories)
	}
}

func TestSeedRequiresAnExplicitDatabase(t *testing.T) {
	if err := run([]string{"seed"}, &bytes.Buffer{}); err == nil {
		t.Fatal("seed accepted the default database")
	}
}

func TestSeedPeriodFlagPinsTheAnchor(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mess.db")

	if err := run([]string{"seed", "--db", dbPath, "--period", "2020-01"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run(seed) unexpected error: %v", err)
	}

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() unexpected error: %v", err)
	}
	defer s.Close()

	pinned := domain.NewPeriod(2020, time.January)
	m, err := month.Load(s.DB(), pinned)
	if err != nil {
		t.Fatalf("month.Load() unexpected error: %v", err)
	}
	if n := len(m.Lines); n < 20 {
		t.Errorf("month.Load(%s) returned %d lines, want the anchor month's worth", pinned, n)
	}
}

type catalogSnapshot struct {
	Concepts   []catalog.Concept
	Categories []catalog.Category
}

func readCatalog(t *testing.T, dbPath string) catalogSnapshot {
	t.Helper()
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() unexpected error: %v", err)
	}
	defer s.Close()

	concepts, err := catalog.Concepts(s.DB())
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	categories, err := catalog.Categories(s.DB())
	if err != nil {
		t.Fatalf("Categories() unexpected error: %v", err)
	}
	return catalogSnapshot{Concepts: concepts, Categories: categories}
}

func replace(string) (bool, error) { return true, nil }
func cancel(string) (bool, error)  { return false, nil }
