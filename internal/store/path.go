package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultPath is os.UserConfigDir()/mess/mess.db, used when --db is not set.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("store: resolve config dir: %w", err)
	}
	return filepath.Join(dir, "mes", "mess.db"), nil
}
