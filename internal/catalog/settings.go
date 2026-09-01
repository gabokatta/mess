package catalog

import (
	"database/sql"
	"errors"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
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

// OpeningBalances anchors the net-worth and leftover-pesos folds. A zero
// Period means the settings row doesn't exist yet — nothing to fold, just
// the zero anchors.
type OpeningBalances struct {
	Period        domain.Period
	LeftoverPesos decimal.Decimal
	CashUSD       decimal.Decimal
	InvestedUSD   decimal.Decimal
}

// LoadOpeningBalances returns the settings row's opening balances, or the
// zero value when the settings row hasn't been created yet.
func LoadOpeningBalances(db *sql.DB) (OpeningBalances, error) {
	var period, leftover, cash, invested string
	err := db.QueryRow(`
		SELECT opening_period, opening_leftover_pesos, opening_cash_usd, opening_invested_usd
		FROM settings WHERE id = 1`).Scan(&period, &leftover, &cash, &invested)
	if errors.Is(err, sql.ErrNoRows) {
		return OpeningBalances{}, nil
	}
	if err != nil {
		return OpeningBalances{}, err
	}

	var ob OpeningBalances
	if ob.Period, err = domain.ParsePeriod(period); err != nil {
		return OpeningBalances{}, err
	}
	if ob.LeftoverPesos, err = decimal.NewFromString(leftover); err != nil {
		return OpeningBalances{}, err
	}
	if ob.CashUSD, err = decimal.NewFromString(cash); err != nil {
		return OpeningBalances{}, err
	}
	if ob.InvestedUSD, err = decimal.NewFromString(invested); err != nil {
		return OpeningBalances{}, err
	}
	return ob, nil
}
