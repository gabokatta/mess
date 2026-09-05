package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// A category's colour used to be its position in the list. Moving it into a
// column has to leave every existing database looking exactly as it did, so
// the upgrade is invisible and the first colour change is a deliberate one.
func TestCategoryColourBackfillKeepsExistingColours(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "mess.db"))
	if err != nil {
		t.Fatalf("sql.Open() unexpected error: %v", err)
	}
	defer db.Close()

	// A database as it stood before the colour column existed: schema 0002
	// applied, categories in it, nothing that knows about colour.
	script, err := migrationFiles.ReadFile("migrations/0002_schema.sql")
	if err != nil {
		t.Fatalf("read 0002: %v", err)
	}
	if _, err := db.Exec(string(script)); err != nil {
		t.Fatalf("apply 0002: %v", err)
	}
	if _, err := db.Exec("PRAGMA user_version = 2"); err != nil {
		t.Fatalf("set version: %v", err)
	}

	// Nine categories, so the ninth is the one that used to collide with the
	// first and has to keep colliding rather than being quietly moved.
	names := []string{"Earnings", "Home", "Utilities", "Cards", "Food", "Health", "Transport", "Debt", "Savings"}
	for i, name := range names {
		if _, err := db.Exec(`INSERT INTO category (name, sort_order) VALUES (?, ?)`, name, i); err != nil {
			t.Fatalf("seed %q: %v", name, err)
		}
	}

	if err := migrate(db); err != nil {
		t.Fatalf("migrate() unexpected error: %v", err)
	}

	rows, err := db.Query(`SELECT name, color_index FROM category ORDER BY sort_order, name`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	for position := 0; rows.Next(); position++ {
		var name string
		var index int
		if err := rows.Scan(&name, &index); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if want := position % 8; index != want {
			t.Errorf("%s kept colour %d, want its old position %d", name, index, want)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
}
