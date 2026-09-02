package catalog

import (
	"database/sql"
	"errors"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
)

// OpeningBalances anchors the net-worth and leftover-pesos folds. A zero
// Period means the settings row doesn't exist yet — nothing to fold, just
// the zero anchors.
type OpeningBalances struct {
	Period        domain.Period
	LeftoverPesos decimal.Decimal
	CashUSD       decimal.Decimal
	InvestedUSD   decimal.Decimal
}

// Settings is the singleton settings row: the fx house quotes are drawn
// from, and the opening balances that anchor the net-worth fold.
type Settings struct {
	FxHouse domain.FxHouse
	Opening OpeningBalances
}

// LoadSettings returns the settings row, or Settings{FxHouse: domain.Blue}
// — every other field at its zero value — when the row doesn't exist yet.
func LoadSettings(db *sql.DB) (Settings, error) {
	var house, leftover, cash, invested string
	var period sql.NullString
	err := db.QueryRow(`
		SELECT fx_house, opening_period, opening_leftover_pesos, opening_cash_usd, opening_invested_usd
		FROM settings WHERE id = 1`).
		Scan(&house, &period, &leftover, &cash, &invested)
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
	if period.Valid {
		if s.Opening.Period, err = domain.ParsePeriod(period.String); err != nil {
			return Settings{}, err
		}
	}
	if s.Opening.LeftoverPesos, err = decimal.NewFromString(leftover); err != nil {
		return Settings{}, err
	}
	if s.Opening.CashUSD, err = decimal.NewFromString(cash); err != nil {
		return Settings{}, err
	}
	if s.Opening.InvestedUSD, err = decimal.NewFromString(invested); err != nil {
		return Settings{}, err
	}
	return s, nil
}

// SaveSettings upserts the singleton settings row.
func SaveSettings(db *sql.DB, s Settings) error {
	_, err := db.Exec(`
		INSERT INTO settings
			(id, fx_house, opening_period, opening_leftover_pesos, opening_cash_usd, opening_invested_usd)
		VALUES (1, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			fx_house = excluded.fx_house,
			opening_period = excluded.opening_period,
			opening_leftover_pesos = excluded.opening_leftover_pesos,
			opening_cash_usd = excluded.opening_cash_usd,
			opening_invested_usd = excluded.opening_invested_usd`,
		s.FxHouse.String(), nullablePeriod(s.Opening.Period),
		s.Opening.LeftoverPesos.String(), s.Opening.CashUSD.String(), s.Opening.InvestedUSD.String())
	return err
}

// FxHouse returns the configured fx house, defaulting to Blue when the
// settings row hasn't been created yet.
func FxHouse(db *sql.DB) (domain.FxHouse, error) {
	s, err := LoadSettings(db)
	return s.FxHouse, err
}

// LoadOpeningBalances returns the settings row's opening balances, or the
// zero value when the settings row hasn't been created yet.
func LoadOpeningBalances(db *sql.DB) (OpeningBalances, error) {
	s, err := LoadSettings(db)
	return s.Opening, err
}
