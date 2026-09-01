package catalog

import (
	"database/sql"

	"github.com/gabokatta/mes/internal/domain"
)

// ChoreEntry is the done state for one chore in one period.
type ChoreEntry struct {
	ChoreID int64
	Period  domain.Period
	Done    bool
}

func SetChoreEntryDone(db *sql.DB, choreID int64, period domain.Period, done bool) error {
	_, err := db.Exec(`
		INSERT INTO chore_entry (chore_id, period, done)
		VALUES (?, ?, ?)
		ON CONFLICT (chore_id, period) DO UPDATE SET done = excluded.done`,
		choreID, period.String(), done)
	return err
}

func ChoreEntries(db *sql.DB, period domain.Period) ([]ChoreEntry, error) {
	rows, err := db.Query(`
		SELECT chore_id, period, done FROM chore_entry WHERE period = ?`, period.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ChoreEntry
	for rows.Next() {
		var (
			e         ChoreEntry
			periodStr string
			done      int
		)
		if err := rows.Scan(&e.ChoreID, &periodStr, &done); err != nil {
			return nil, err
		}
		if e.Period, err = domain.ParsePeriod(periodStr); err != nil {
			return nil, err
		}
		e.Done = done != 0
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
