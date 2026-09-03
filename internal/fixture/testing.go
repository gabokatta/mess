package fixture

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/gabokatta/mess/internal/store"
)

// DB opens a freshly migrated database in t.TempDir(). Each call gets its
// own temp dir, so parallel tests never contend for store.Open's instance
// lock; a test that needs two databases must still give them different
// paths.
func DB(t *testing.T) *sql.DB {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "mess.db"))
	if err != nil {
		t.Fatalf("store.Open() unexpected error: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s.DB()
}

// MustLoad is Load for a test: any error fails it immediately, so a test
// body only ever sees a fully loaded World.
func MustLoad(t *testing.T, db *sql.DB, w World) Loaded {
	t.Helper()
	loaded, err := Load(db, w)
	if err != nil {
		t.Fatalf("fixture.Load() unexpected error: %v", err)
	}
	return loaded
}
