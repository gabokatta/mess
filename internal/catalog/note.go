package catalog

import (
	"database/sql"

	"github.com/gabokatta/mess/internal/domain"
)

// Note carries a zero Period when pinned, which shows it in every period
// rather than none.
type Note struct {
	ID     int64
	Title  string
	BodyMD string
	Period domain.Period // zero value means pinned
	Done   bool
}

func CreateNote(db *sql.DB, n Note) (Note, error) {
	res, err := db.Exec(`
		INSERT INTO note (title, body_md, period, done) VALUES (?, ?, ?, ?)`,
		n.Title, n.BodyMD, nullablePeriod(n.Period), n.Done)
	if err != nil {
		return Note{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Note{}, err
	}
	n.ID = id
	return n, nil
}

func Notes(db *sql.DB) ([]Note, error) {
	rows, err := db.Query(`SELECT id, title, body_md, period, done FROM note ORDER BY title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var (
			n      Note
			period sql.NullString
			done   int
		)
		if err := rows.Scan(&n.ID, &n.Title, &n.BodyMD, &period, &done); err != nil {
			return nil, err
		}
		if period.Valid {
			var err error
			if n.Period, err = domain.ParsePeriod(period.String); err != nil {
				return nil, err
			}
		}
		n.Done = done != 0
		notes = append(notes, n)
	}
	return notes, rows.Err()
}

func SetNoteBody(db *sql.DB, id int64, bodyMD string) error {
	_, err := db.Exec(`UPDATE note SET body_md = ? WHERE id = ?`, bodyMD, id)
	return err
}

func SetNoteDone(db *sql.DB, id int64, done bool) error {
	_, err := db.Exec(`UPDATE note SET done = ? WHERE id = ?`, done, id)
	return err
}

// SetNotePeriod pins the note when period is zero.
func SetNotePeriod(db *sql.DB, id int64, period domain.Period) error {
	_, err := db.Exec(`UPDATE note SET period = ? WHERE id = ?`, nullablePeriod(period), id)
	return err
}
