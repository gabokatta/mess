// Package store owns the SQLite database: opening it, applying migrations,
// and holding the single-instance lock.
package store

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	db   *sql.DB
	lock *lock
}

// Open takes the instance lock and migrates path, creating the database and
// its parent directory if needed.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("store: create %s: %w", filepath.Dir(path), err)
	}

	l, err := acquireLock(path)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		l.release()
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		l.release()
		return nil, err
	}

	return &Store{db: db, lock: l}, nil
}

// dsn carries the pragmas in the connection string rather than running them
// once after opening. foreign_keys is per connection, and database/sql keeps a
// pool: a single Exec sets it on whichever connection happened to serve that
// call and leaves every later one without it, which quietly turns every
// foreign key in the schema off.
func dsn(path string) string {
	u := url.URL{Scheme: "file", Opaque: path, RawQuery: url.Values{
		"_pragma": {"foreign_keys(1)", "journal_mode(WAL)"},
	}.Encode()}
	return u.String()
}

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) Close() error {
	dbErr := s.db.Close()
	lockErr := s.lock.release()
	if dbErr != nil {
		return dbErr
	}
	return lockErr
}
