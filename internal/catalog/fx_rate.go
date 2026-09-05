package catalog

import (
	"database/sql"
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
)

// FxSource separates a month's real close from a rate set by hand. Nothing
// automatic ever replaces a Manual row.
type FxSource int

const (
	Close FxSource = iota
	Manual
)

func (s FxSource) String() string {
	switch s {
	case Close:
		return "Close"
	case Manual:
		return "Manual"
	default:
		return fmt.Sprintf("FxSource(%d)", int(s))
	}
}

func ParseFxSource(s string) (FxSource, error) {
	switch s {
	case "Close":
		return Close, nil
	case "Manual":
		return Manual, nil
	default:
		return 0, fmt.Errorf("catalog: invalid fx source %q", s)
	}
}

// FxRate stores a completed month's close or a manual override. House records
// a close's source even after the setting changes; manual rates have no house.
type FxRate struct {
	Period domain.Period
	Value  decimal.Decimal
	Source FxSource
	House  *domain.FxHouse
}

func SetManualFxRate(db *sql.DB, period domain.Period, value decimal.Decimal) error {
	_, err := db.Exec(`
		INSERT INTO fx_rate (period, value, source, house) VALUES (?, ?, 'Manual', NULL)
		ON CONFLICT (period) DO UPDATE SET
			value = excluded.value, source = excluded.source, house = excluded.house`,
		period.String(), value.String())
	return err
}

// SaveFxClose leaves an existing rate for that period alone, so backfill
// never overwrites and a Manual row survives it.
func SaveFxClose(db *sql.DB, period domain.Period, value decimal.Decimal, house domain.FxHouse) error {
	_, err := db.Exec(`
		INSERT INTO fx_rate (period, value, source, house) VALUES (?, ?, 'Close', ?)
		ON CONFLICT (period) DO NOTHING`,
		period.String(), value.String(), house.String())
	return err
}

// ClearFxRate deletes one period's rate, which is how a mistyped manual value
// and a close fetched at a house no longer in use are both undone: the row
// goes, and backfill treats the month as missing again.
func ClearFxRate(db *sql.DB, period domain.Period) error {
	_, err := db.Exec(`DELETE FROM fx_rate WHERE period = ?`, period.String())
	return err
}

func FxRates(db *sql.DB) ([]FxRate, error) {
	rows, err := db.Query(`SELECT period, value, source, house FROM fx_rate ORDER BY period`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rates []FxRate
	for rows.Next() {
		var (
			r                              FxRate
			periodStr, valueStr, sourceStr string
			houseStr                       sql.NullString
		)
		if err := rows.Scan(&periodStr, &valueStr, &sourceStr, &houseStr); err != nil {
			return nil, err
		}
		var err error
		if r.Period, err = domain.ParsePeriod(periodStr); err != nil {
			return nil, err
		}
		if r.Value, err = decimal.NewFromString(valueStr); err != nil {
			return nil, err
		}
		if r.Source, err = ParseFxSource(sourceStr); err != nil {
			return nil, err
		}
		if houseStr.Valid {
			house, err := domain.ParseFxHouse(houseStr.String)
			if err != nil {
				return nil, err
			}
			r.House = &house
		}
		rates = append(rates, r)
	}
	return rates, rows.Err()
}
