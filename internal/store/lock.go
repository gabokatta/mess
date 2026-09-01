package store

import (
	"fmt"
	"os"
)

// lock is an exclusive-create file beside the database that stops a second
// instance opening it. A crash leaves it behind; clearing it is a manual
// step rather than automatic stale-lock detection.
type lock struct {
	path string
}

func acquireLock(dbPath string) (*lock, error) {
	path := dbPath + ".lock"
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("store: %s is already open (delete %s if this is stale)", dbPath, path)
		}
		return nil, fmt.Errorf("store: create lock %s: %w", path, err)
	}
	f.Close()
	return &lock{path: path}, nil
}

func (l *lock) release() error {
	return os.Remove(l.path)
}
