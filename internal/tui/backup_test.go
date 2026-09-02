package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/huh/v2"

	"github.com/gabokatta/mess/internal/backup"
	"github.com/gabokatta/mess/internal/catalog"
)

func TestXKeyOpensExportFormPrefilledWithADefaultPath(t *testing.T) {
	m := settingsModel(t, openTestStore(t))

	updated, cmd := m.Update(key("x"))
	m = updated.(Model)
	if m.exportForm == nil {
		t.Fatal("exportForm = nil, want a form opened")
	}
	if m.exportForm.values.path == "" {
		t.Error("values.path = \"\", want a default destination prefilled")
	}
	m = settle(t, m, cmd)

	content := m.View().Content
	if !strings.Contains(content, "Export") {
		t.Errorf("content = %q, want the export form's title", content)
	}
}

func TestCompletingExportFormWritesTheBackupFile(t *testing.T) {
	db := openTestStore(t)
	if _, err := catalog.CreateCategory(db, "Servicios", 0); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	m := settingsModel(t, db)
	dest := filepath.Join(t.TempDir(), "out.json")

	updated, _ := m.Update(key("x"))
	m = updated.(Model)
	m.exportForm.values.path = dest
	m.exportForm.form.State = huh.StateCompleted

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	if m.exportForm != nil {
		t.Fatal("a completed form should close")
	}
	m = settle(t, m, cmd)

	raw, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("os.ReadFile(%s) unexpected error: %v", dest, err)
	}
	var data backup.Data
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}
	if rows := data.Tables["category"]; len(rows) != 1 || rows[0]["name"] != "Servicios" {
		t.Errorf("Tables[category] = %+v, want the Servicios row", rows)
	}
}

func TestIKeyOpensImportFormRequiringConfirmation(t *testing.T) {
	db := openTestStore(t)
	if _, err := catalog.CreateCategory(db, "Stale", 0); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	src, err := backup.Export(db)
	if err != nil {
		t.Fatalf("backup.Export() unexpected error: %v", err)
	}
	raw, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}
	file := filepath.Join(t.TempDir(), "backup.json")
	if err := os.WriteFile(file, raw, 0o644); err != nil {
		t.Fatalf("os.WriteFile() unexpected error: %v", err)
	}

	m := settingsModel(t, db)
	m = m.WithDBPath(filepath.Join(t.TempDir(), "mess.db"))

	updated, _ := m.Update(key("i"))
	m = updated.(Model)
	if m.importForm == nil {
		t.Fatal("importForm = nil, want a form opened")
	}
	m.importForm.values.path = file
	m.importForm.values.confirmed = false
	m.importForm.form.State = huh.StateCompleted

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	m = settle(t, m, cmd)

	got, err := catalog.Categories(db)
	if err != nil {
		t.Fatalf("Categories() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Stale" {
		t.Errorf("Categories() = %+v, want the original row untouched (confirm was declined)", got)
	}
}

func TestConfirmingImportReplacesTheDatabase(t *testing.T) {
	src := openTestStore(t)
	if _, err := catalog.CreateCategory(src, "Servicios", 0); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	data, err := backup.Export(src)
	if err != nil {
		t.Fatalf("backup.Export() unexpected error: %v", err)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}
	file := filepath.Join(t.TempDir(), "backup.json")
	if err := os.WriteFile(file, raw, 0o644); err != nil {
		t.Fatalf("os.WriteFile() unexpected error: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "mess.db")
	dst := openTestStoreAt(t, dbPath)
	if _, err := catalog.CreateCategory(dst, "Stale row that import must remove", 0); err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}

	m := settingsModel(t, dst)
	m = m.WithDBPath(dbPath)

	updated, _ := m.Update(key("i"))
	m = updated.(Model)
	m.importForm.values.path = file
	m.importForm.values.confirmed = true
	m.importForm.form.State = huh.StateCompleted

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	m = settle(t, m, cmd)

	got, err := catalog.Categories(dst)
	if err != nil {
		t.Fatalf("Categories() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Servicios" {
		t.Fatalf("Categories() = %+v, want only the imported Servicios row", got)
	}

	matches, err := filepath.Glob(dbPath + ".*.bak")
	if err != nil {
		t.Fatalf("filepath.Glob() unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("snapshot files = %v, want exactly one .bak written before the import", matches)
	}
}

func TestEscCancelsImportFormWithoutWriting(t *testing.T) {
	m := settingsModel(t, openTestStore(t))

	updated, _ := m.Update(key("i"))
	m = updated.(Model)
	m.importForm.values.path = "irrelevant.json"

	updated, cmd := m.Update(keyEsc())
	m = updated.(Model)
	if m.importForm != nil {
		t.Error("esc should close the form")
	}
	if cmd != nil {
		t.Error("esc should not write anything")
	}
}
