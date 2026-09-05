package catalog

import (
	"database/sql"
	"errors"

	"github.com/gabokatta/mess/internal/domain"
)

type Settings struct {
	FxHouse domain.FxHouse
}

// LoadSettings falls back to Blue when unconfigured.
func LoadSettings(db *sql.DB) (Settings, error) {
	var house string
	err := db.QueryRow(`SELECT fx_house FROM settings WHERE id = 1`).Scan(&house)
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
	return s, nil
}

func SetFxHouse(db *sql.DB, house domain.FxHouse) error {
	_, err := db.Exec(`
		INSERT INTO settings (id, fx_house) VALUES (1, ?)
		ON CONFLICT (id) DO UPDATE SET fx_house = excluded.fx_house`,
		house.String())
	return err
}

func FxHouse(db *sql.DB) (domain.FxHouse, error) {
	s, err := LoadSettings(db)
	return s.FxHouse, err
}
