package catalog

import (
	"database/sql"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
)

// Only Done entries contribute to totals. A nil Amount uses the concept's base.
type MonthEntry struct {
	ConceptID int64
	Period    domain.Period
	Amount    *decimal.Decimal
	Done      bool
}

// Setting an amount confirms the entry. Clearing it restores the base without
// changing Done; unconfirming requires SetMonthEntryDone.
func SetMonthEntryAmount(db *sql.DB, conceptID int64, period domain.Period, amount *decimal.Decimal) error {
	_, err := db.Exec(`
		INSERT INTO month_entry (concept_id, period, amount, done)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (concept_id, period) DO UPDATE SET
			amount = excluded.amount,
			done = month_entry.done OR excluded.done`,
		conceptID, period.String(), nullableAmount(amount), amount != nil)
	return err
}

// SetMonthEntryDone sets the done state for concept in period, leaving the
// amount override untouched.
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
	return MonthEntriesBetween(db, period, period)
}

// MonthEntriesBetween includes both endpoint months.
func MonthEntriesBetween(db *sql.DB, first, last domain.Period) ([]MonthEntry, error) {
	rows, err := db.Query(`
		SELECT concept_id, period, amount, done FROM month_entry
		WHERE period >= ? AND period <= ?`, first.String(), last.String())
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
