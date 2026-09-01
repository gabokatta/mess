package catalog

import (
	"database/sql"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mes/internal/domain"
)

type BaseAmount struct {
	ConceptID     int64
	EffectiveFrom domain.Period
	Amount        decimal.Decimal
}

// SetBaseAmount records the amount effective from a given period, correcting
// the existing row in place if one already exists for that concept and date.
func SetBaseAmount(db *sql.DB, conceptID int64, effectiveFrom domain.Period, amount decimal.Decimal) error {
	_, err := db.Exec(`
		INSERT INTO base_amount (concept_id, effective_from, amount)
		VALUES (?, ?, ?)
		ON CONFLICT (concept_id, effective_from) DO UPDATE SET amount = excluded.amount`,
		conceptID, effectiveFrom.String(), amount.String())
	return err
}

func BaseAmounts(db *sql.DB, conceptID int64) ([]BaseAmount, error) {
	rows, err := db.Query(`
		SELECT concept_id, effective_from, amount FROM base_amount
		WHERE concept_id = ? ORDER BY effective_from`, conceptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var amounts []BaseAmount
	for rows.Next() {
		var (
			b                     BaseAmount
			effectiveFrom, amount string
		)
		if err := rows.Scan(&b.ConceptID, &effectiveFrom, &amount); err != nil {
			return nil, err
		}
		var err error
		if b.EffectiveFrom, err = domain.ParsePeriod(effectiveFrom); err != nil {
			return nil, err
		}
		if b.Amount, err = decimal.NewFromString(amount); err != nil {
			return nil, err
		}
		amounts = append(amounts, b)
	}
	return amounts, rows.Err()
}
