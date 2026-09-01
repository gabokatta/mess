package store

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// migrationVersion is the number before a migration filename's first '_'.
// Versions come from that number, not file order, so the sequence
// tolerates gaps.
func migrationVersion(name string) (int, error) {
	sep := strings.IndexByte(name, '_')
	if sep < 0 {
		return 0, fmt.Errorf("store: migration %q missing '_' separator", name)
	}
	n, err := strconv.Atoi(name[:sep])
	if err != nil {
		return 0, fmt.Errorf("store: migration %q has a non-numeric version: %w", name, err)
	}
	return n, nil
}

// migrate applies every embedded migration newer than the database's
// current schema version, tracked in SQLite's PRAGMA user_version.
func migrate(db *sql.DB) error {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	sort.Strings(names)

	var current int
	if err := db.QueryRow("PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("store: read schema version: %w", err)
	}

	for _, name := range names {
		version, err := migrationVersion(name)
		if err != nil {
			return err
		}
		if version <= current {
			continue
		}

		script, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}

		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(script)); err != nil {
			tx.Rollback()
			return fmt.Errorf("store: apply %s: %w", name, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
			tx.Rollback()
			return fmt.Errorf("store: record version for %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
