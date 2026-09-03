package catalog

import (
	"database/sql"
	"errors"
	"time"

	"github.com/gabokatta/mess/internal/domain"
)

type Settings struct {
	FxHouse    domain.FxHouse
	LastExport *time.Time // nil means never exported
}

// LoadSettings falls back to Blue and no export on a database that has
// never been configured.
func LoadSettings(db *sql.DB) (Settings, error) {
	var house string
	var lastExport sql.NullString
	err := db.QueryRow(`SELECT fx_house, last_export FROM settings WHERE id = 1`).Scan(&house, &lastExport)
	if errors.Is(err, sql.ErrNoRows) {
		return Settings{FxHouse: domain.Blue}, nil
	}
	if err != nil {
		return Settings{}, err
	}

	var s Settings
	if s.FxHouse, err = domain.ParseFxHouse(house); err != nil {
		return Settings{}, err
	}
	if lastExport.Valid {
		t, err := time.Parse(time.RFC3339, lastExport.String)
		if err != nil {
			return Settings{}, err
		}
		s.LastExport = &t
	}
	return s, nil
}

func SetFxHouse(db *sql.DB, house domain.FxHouse) error {
	_, err := db.Exec(`
		INSERT INTO settings (id, fx_house) VALUES (1, ?)
		ON CONFLICT (id) DO UPDATE SET fx_house = excluded.fx_house`,
		house.String())
	return err
}

func MarkExported(db *sql.DB, at time.Time) error {
	_, err := db.Exec(`
		INSERT INTO settings (id, fx_house, last_export) VALUES (1, ?, ?)
		ON CONFLICT (id) DO UPDATE SET last_export = excluded.last_export`,
		domain.Blue.String(), at.UTC().Format(time.RFC3339))
	return err
}

func FxHouse(db *sql.DB) (domain.FxHouse, error) {
	s, err := LoadSettings(db)
	return s.FxHouse, err
}
