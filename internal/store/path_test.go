package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPathIsUnderUserConfigDir(t *testing.T) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		t.Skipf("no user config dir on this platform: %v", err)
	}

	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() unexpected error: %v", err)
	}

	want := filepath.Join(configDir, "mes", "mes.db")
	if got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}
