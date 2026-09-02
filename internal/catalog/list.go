package catalog

import (
	"database/sql"
	"time"

	"github.com/gabokatta/mess/internal/domain"
)

// List is a name plus a markdown body: a todo list, notes, or both. Its
// storage is a single text field — checkboxes inside body_md are the only
// structure the app imposes on it.
type List struct {
	ID       int64
	Name     string
	BodyMD   string
	Period   domain.Period // zero value means unassigned
	ClosedAt *time.Time    // nil means open
}

func CreateList(db *sql.DB, l List) (List, error) {
	res, err := db.Exec(`
		INSERT INTO list (name, body_md, period, closed_at)
		VALUES (?, ?, ?, ?)`,
		l.Name, l.BodyMD, nullablePeriod(l.Period), nullableTime(l.ClosedAt))
	if err != nil {
		return List{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return List{}, err
	}
	l.ID = id
	return l, nil
}

func Lists(db *sql.DB) ([]List, error) {
	rows, err := db.Query(`SELECT id, name, body_md, period, closed_at FROM list ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lists []List
	for rows.Next() {
		l, err := scanList(rows)
		if err != nil {
			return nil, err
		}
		lists = append(lists, l)
	}
	return lists, rows.Err()
}

// SetListBody overwrites a list's markdown body — the write behind
// both checkbox toggling and the full textarea edit.
func SetListBody(db *sql.DB, id int64, bodyMD string) error {
	_, err := db.Exec(`UPDATE list SET body_md = ? WHERE id = ?`, bodyMD, id)
	return err
}

// SetListClosed sets or clears a list's closed_at. Closing is
// unconditional — it never checks whether every checkbox is ticked.
func SetListClosed(db *sql.DB, id int64, closedAt *time.Time) error {
	_, err := db.Exec(`UPDATE list SET closed_at = ? WHERE id = ?`, nullableTime(closedAt), id)
	return err
}

// SetListPeriod assigns or reassigns which period a list is stamped
// to. The zero Period clears it back to unassigned.
func SetListPeriod(db *sql.DB, id int64, period domain.Period) error {
	_, err := db.Exec(`UPDATE list SET period = ? WHERE id = ?`, nullablePeriod(period), id)
	return err
}

func scanList(row *sql.Rows) (List, error) {
	var (
		l        List
		period   sql.NullString
		closedAt sql.NullString
	)
	if err := row.Scan(&l.ID, &l.Name, &l.BodyMD, &period, &closedAt); err != nil {
		return List{}, err
	}

	if period.Valid {
		var err error
		if l.Period, err = domain.ParsePeriod(period.String); err != nil {
			return List{}, err
		}
	}
	if closedAt.Valid {
		t, err := time.Parse(time.RFC3339, closedAt.String)
		if err != nil {
			return List{}, err
		}
		l.ClosedAt = &t
	}
	return l, nil
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}
