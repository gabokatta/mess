package catalog

import (
	"database/sql"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
)

// MonthEntry is the override for one concept in one period. Its presence is
// what makes an amount "confirmed" rather than projected from the base.
type MonthEntry struct {
	ConceptID int64
	Period    domain.Period
	Amount    *decimal.Decimal // nil means unconfirmed: resolve from the base instead
	Done      bool
}

// SetMonthEntryAmount sets the override for concept in period, or clears it
// when amount is nil, leaving the row's done state untouched.
func SetMonthEntryAmount(db *sql.DB, conceptID int64, period domain.Period, amount *decimal.Decimal) error {
	_, err := db.Exec(`
		INSERT INTO month_entry (concept_id, period, amount, done)
		VALUES (?, ?, ?, 0)
		ON CONFLICT (concept_id, period) DO UPDATE SET amount = excluded.amount`,
		conceptID, period.String(), nullableAmount(amount))
	return err
}

// SetMonthEntryDone sets the done state for concept in period, leaving the
// row's amount override untouched.
func SetMonthEntryDone(db *sql.DB, conceptID int64, period domain.Period, done bool) error {
	_, err := db.Exec(`
		INSERT INTO month_entry (concept_id, period, amount, done)
		VALUES (?, ?, NULL, ?)
		ON CONFLICT (concept_id, period) DO UPDATE SET done = excluded.done`,
		conceptID, period.String(), done)
	return err
}

func nullableAmount(amount *decimal.Decimal) any {
	if amount == nil {
		return nil
	}
	return amount.String()
}

func MonthEntries(db *sql.DB, period domain.Period) ([]MonthEntry, error) {
	rows, err := db.Query(`
		SELECT concept_id, period, amount, done FROM month_entry WHERE period = ?`, period.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []MonthEntry
	for rows.Next() {
		var (
			e         MonthEntry
			periodStr string
			amount    sql.NullString
			done      int
		)
		if err := rows.Scan(&e.ConceptID, &periodStr, &amount, &done); err != nil {
			return nil, err
		}
		var err error
		if e.Period, err = domain.ParsePeriod(periodStr); err != nil {
			return nil, err
		}
		if amount.Valid {
			d, err := decimal.NewFromString(amount.String)
			if err != nil {
				return nil, err
			}
			e.Amount = &d
		}
		e.Done = done != 0
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
