package catalog

import (
	"database/sql"
	"errors"

	"github.com/gabokatta/mes/internal/domain"
)

// FxHouse returns the configured fx house, defaulting to Blue — its zero
// value — when the settings row hasn't been created yet.
func FxHouse(db *sql.DB) (domain.FxHouse, error) {
	var house string
	err := db.QueryRow(`SELECT fx_house FROM settings WHERE id = 1`).Scan(&house)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Blue, nil
	}
	if err != nil {
		return 0, err
	}
	return domain.ParseFxHouse(house)
}
