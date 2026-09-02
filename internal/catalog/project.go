package catalog

import (
	"database/sql"
	"time"

	"github.com/gabokatta/mess/internal/domain"
)

// Project is a name plus a markdown body: a todo list, notes, or both. Its
// storage is a single text field — checkboxes inside body_md are the only
// structure the app imposes on it.
type Project struct {
	ID       int64
	Name     string
	BodyMD   string
	Period   domain.Period // zero value means unassigned
	ClosedAt *time.Time    // nil means open
}

func CreateProject(db *sql.DB, p Project) (Project, error) {
	res, err := db.Exec(`
		INSERT INTO project (name, body_md, period, closed_at)
		VALUES (?, ?, ?, ?)`,
		p.Name, p.BodyMD, nullablePeriod(p.Period), nullableTime(p.ClosedAt))
	if err != nil {
		return Project{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Project{}, err
	}
	p.ID = id
	return p, nil
}

func Projects(db *sql.DB) ([]Project, error) {
	rows, err := db.Query(`SELECT id, name, body_md, period, closed_at FROM project ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// SetProjectBody overwrites a project's markdown body — the write behind
// both checkbox toggling and the full textarea edit.
func SetProjectBody(db *sql.DB, id int64, bodyMD string) error {
	_, err := db.Exec(`UPDATE project SET body_md = ? WHERE id = ?`, bodyMD, id)
	return err
}

// SetProjectClosed sets or clears a project's closed_at. Closing is
// unconditional — it never checks whether every checkbox is ticked.
func SetProjectClosed(db *sql.DB, id int64, closedAt *time.Time) error {
	_, err := db.Exec(`UPDATE project SET closed_at = ? WHERE id = ?`, nullableTime(closedAt), id)
	return err
}

// SetProjectPeriod assigns or reassigns which period a project is stamped
// to. The zero Period clears it back to unassigned.
func SetProjectPeriod(db *sql.DB, id int64, period domain.Period) error {
	_, err := db.Exec(`UPDATE project SET period = ? WHERE id = ?`, nullablePeriod(period), id)
	return err
}

func scanProject(row *sql.Rows) (Project, error) {
	var (
		p        Project
		period   sql.NullString
		closedAt sql.NullString
	)
	if err := row.Scan(&p.ID, &p.Name, &p.BodyMD, &period, &closedAt); err != nil {
		return Project{}, err
	}

	if period.Valid {
		var err error
		if p.Period, err = domain.ParsePeriod(period.String); err != nil {
			return Project{}, err
		}
	}
	if closedAt.Valid {
		t, err := time.Parse(time.RFC3339, closedAt.String)
		if err != nil {
			return Project{}, err
		}
		p.ClosedAt = &t
	}
	return p, nil
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}
