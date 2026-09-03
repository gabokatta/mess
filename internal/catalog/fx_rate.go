package catalog

import (
	"database/sql"
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
)

// FxSource separates a month's real close from a rate you set by hand.
// Nothing automatic ever replaces a Manual row.
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

// FxRate is a completed month: the current month is never stored, so a row
// here never needs refetching.
type FxRate struct {
	Period domain.Period
	Value  decimal.Decimal
	Source FxSource
}

func SetManualFxRate(db *sql.DB, period domain.Period, value decimal.Decimal) error {
	_, err := db.Exec(`
		INSERT INTO fx_rate (period, value, source) VALUES (?, ?, 'Manual')
		ON CONFLICT (period) DO UPDATE SET value = excluded.value, source = excluded.source`,
		period.String(), value.String())
	return err
}

// SaveFxClose leaves any rate already recorded for that period alone, so
// backfill never overwrites and a Manual row survives it.
func SaveFxClose(db *sql.DB, period domain.Period, value decimal.Decimal) error {
	_, err := db.Exec(`
		INSERT INTO fx_rate (period, value, source) VALUES (?, ?, 'Close')
		ON CONFLICT (period) DO NOTHING`,
		period.String(), value.String())
	return err
}

func FxRates(db *sql.DB) ([]FxRate, error) {
	rows, err := db.Query(`SELECT period, value, source FROM fx_rate ORDER BY period`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rates []FxRate
	for rows.Next() {
		var (
			r                              FxRate
			periodStr, valueStr, sourceStr string
		)
		if err := rows.Scan(&periodStr, &valueStr, &sourceStr); err != nil {
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
		rates = append(rates, r)
	}
	return rates, rows.Err()
}
