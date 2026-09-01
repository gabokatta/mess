package catalog

import (
	"database/sql"

	"github.com/gabokatta/mess/internal/domain"
)

// Chore is a monthly checklist item: a done state per period, no amount.
type Chore struct {
	ID          int64
	Name        string
	MonthMask   domain.Cadence
	DueDay      int // 0 means unset
	SortOrder   int
	ActiveFrom  domain.Period
	ActiveUntil domain.Period // zero value means still active
}

func CreateChore(db *sql.DB, c Chore) (Chore, error) {
	res, err := db.Exec(`
		INSERT INTO chore (name, month_mask, due_day, active_from, active_until, sort_order)
		VALUES (?, ?, ?, ?, ?, ?)`,
		c.Name, int(c.MonthMask), nullableDueDay(c.DueDay), c.ActiveFrom.String(), nullablePeriod(c.ActiveUntil), c.SortOrder)
	if err != nil {
		return Chore{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Chore{}, err
	}
	c.ID = id
	return c, nil
}

func Chores(db *sql.DB) ([]Chore, error) {
	rows, err := db.Query(`
		SELECT id, name, month_mask, due_day, active_from, active_until, sort_order
		FROM chore ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chores []Chore
	for rows.Next() {
		c, err := scanChore(rows)
		if err != nil {
			return nil, err
		}
		chores = append(chores, c)
	}
	return chores, rows.Err()
}

func UpdateChore(db *sql.DB, c Chore) error {
	_, err := db.Exec(`
		UPDATE chore
		SET name = ?, month_mask = ?, due_day = ?, active_from = ?, active_until = ?, sort_order = ?
		WHERE id = ?`,
		c.Name, int(c.MonthMask), nullableDueDay(c.DueDay), c.ActiveFrom.String(), nullablePeriod(c.ActiveUntil), c.SortOrder, c.ID)
	return err
}

func scanChore(row *sql.Rows) (Chore, error) {
	var (
		c           Chore
		monthMask   int
		dueDay      sql.NullInt64
		activeFrom  string
		activeUntil sql.NullString
	)
	if err := row.Scan(&c.ID, &c.Name, &monthMask, &dueDay, &activeFrom, &activeUntil, &c.SortOrder); err != nil {
		return Chore{}, err
	}

	c.MonthMask = domain.Cadence(monthMask)
	c.DueDay = int(dueDay.Int64)
	var err error
	if c.ActiveFrom, err = domain.ParsePeriod(activeFrom); err != nil {
		return Chore{}, err
	}
	if activeUntil.Valid {
		if c.ActiveUntil, err = domain.ParsePeriod(activeUntil.String); err != nil {
			return Chore{}, err
		}
	}
	return c, nil
}
