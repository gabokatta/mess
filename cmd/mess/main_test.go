package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/fixture"
	"github.com/gabokatta/mess/internal/store"
)

func TestExportImportRoundTripsThroughTheCLI(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.db")
	dstPath := filepath.Join(dir, "dst.db")

	src, err := store.Open(srcPath)
	if err != nil {
		t.Fatalf("store.Open() unexpected error: %v", err)
	}
	fixture.MustLoad(t, src.DB(), fixture.World{
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

// Cancelling the gate leaves the database exactly as it was.
func TestImportCancelledLeavesTheDatabaseAlone(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mess.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() unexpected error: %v", err)
	}
	fixture.MustLoad(t, db.DB(), fixture.World{
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

func replace(string) (bool, error) { return true, nil }
func cancel(string) (bool, error)  { return false, nil }
