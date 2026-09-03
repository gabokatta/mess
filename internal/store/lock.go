package store

import (
	"fmt"
	"os"
)

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
