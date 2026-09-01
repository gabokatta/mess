package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/gabokatta/mess/internal/catalog"
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
	if _, err := catalog.CreateCategory(src.DB(), "Servicios", 0); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
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

	if err := run([]string{"import", "--db", dstPath, backupFile}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run(import) unexpected error: %v", err)
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
	if len(got) != 1 || got[0].Name != "Servicios" {
		t.Errorf("Categories() = %+v, want the Servicios row round-tripped through export/import", got)
	}
}

func TestImportRequiresExactlyOneFileArgument(t *testing.T) {
	dir := t.TempDir()
	err := run([]string{"import", "--db", filepath.Join(dir, "mess.db")}, &bytes.Buffer{})
	if err == nil {
		t.Error("run(import) with no file argument should fail")
	}
}
