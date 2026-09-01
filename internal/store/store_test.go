package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mess.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() unexpected error: %v", err)
	}
	defer s.Close()

	tables := []string{
		"category", "concept", "base_amount", "month_entry",
		"chore", "chore_entry", "saving_allocation", "project",
		"fx_rate", "settings",
	}
	for _, name := range tables {
		var got string
		err := s.DB().QueryRow("SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&got)
		if err != nil {
			t.Errorf("table %s: %v", name, err)
		}
	}
}

func TestOpenCreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "mess.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() unexpected error: %v", err)
	}
	defer s.Close()

	if _, err := os.Stat(path); err != nil {
		t.Errorf("db file not created: %v", err)
	}
}

func TestOpenIsIdempotentAcrossRestarts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mess.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("first Open() unexpected error: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close() unexpected error: %v", err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open() unexpected error: %v", err)
	}
	defer s2.Close()
}

func TestOpenEnforcesForeignKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mess.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() unexpected error: %v", err)
	}
	defer s.Close()

	_, err = s.DB().Exec(`INSERT INTO concept
		(name, category_id, kind, currency, month_mask, active_from)
		VALUES ('Alquiler', 999, 'FixedExpense', 'ARS', 4095, '2026-01')`)
	if err == nil {
		t.Error("insert with dangling category_id should have failed foreign key check")
	}
}

func TestOpenSecondInstanceFailsWhileLocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mess.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() unexpected error: %v", err)
	}
	defer s.Close()

	if _, err := Open(path); err == nil {
		t.Error("second Open() on a locked database should have failed")
	}
}

func TestOpenAfterCloseReleasesLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mess.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() unexpected error: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close() unexpected error: %v", err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("Open() after Close() should succeed, got: %v", err)
	}
	s2.Close()
}
