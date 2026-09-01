package catalog

import (
	"database/sql"
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
)

// FxSource marks whether a period's rate came from the app fetching a quote
// or from you overriding it by hand.
type FxSource int

const (
	Fetched FxSource = iota
	Manual
)

func (s FxSource) String() string {
	switch s {
	case Fetched:
		return "Fetched"
	case Manual:
		return "Manual"
	default:
		return fmt.Sprintf("FxSource(%d)", int(s))
	}
}

func ParseFxSource(s string) (FxSource, error) {
	switch s {
	case "Fetched":
		return Fetched, nil
	case "Manual":
		return Manual, nil
	default:
		return 0, fmt.Errorf("catalog: invalid fx source %q", s)
	}
}

// FxRate is one period's dollar rate: one row per period, never more.
type FxRate struct {
	Period domain.Period
	Value  decimal.Decimal
	Source FxSource
}

// SetFxRate records a manual override for period, replacing whatever rate —
// fetched or manual — was stored for it.
func SetFxRate(db *sql.DB, period domain.Period, value decimal.Decimal) error {
	_, err := db.Exec(`
		INSERT INTO fx_rate (period, value, source) VALUES (?, ?, 'Manual')
		ON CONFLICT (period) DO UPDATE SET value = excluded.value, source = excluded.source`,
		period.String(), value.String())
	return err
}

// FillFetchedFxRate stores value as period's rate only if no rate exists yet
// for it. The app calls this on open with today's quote; it never overwrites
// a rate you already have, fetched or manual.
func FillFetchedFxRate(db *sql.DB, period domain.Period, value decimal.Decimal) error {
	_, err := db.Exec(`
		INSERT INTO fx_rate (period, value, source) VALUES (?, ?, 'Fetched')
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
			r                   FxRate
			periodStr, valueStr string
			sourceStr           string
		)
		if err := rows.Scan(&periodStr, &valueStr, &sourceStr); err != nil {
			return nil, err
		}
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
