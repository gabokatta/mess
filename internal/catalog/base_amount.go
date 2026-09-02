package catalog

import (
	"database/sql"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
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
	return scanBaseAmounts(rows)
}

// AllBaseAmounts loads every concept's base history in one query, grouped by
// concept ID and ordered by effective_from — avoids a query per concept.
func AllBaseAmounts(db *sql.DB) (map[int64][]BaseAmount, error) {
	rows, err := db.Query(`
		SELECT concept_id, effective_from, amount FROM base_amount
		ORDER BY concept_id, effective_from`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	amounts, err := scanBaseAmounts(rows)
	if err != nil {
		return nil, err
	}
	byConcept := make(map[int64][]BaseAmount)
	for _, b := range amounts {
		byConcept[b.ConceptID] = append(byConcept[b.ConceptID], b)
	}
	return byConcept, nil
}

// LatestBaseAmount returns the most recently dated entry in amounts, which
// must already be ordered by effective_from ascending (as BaseAmounts and
// AllBaseAmounts return them). It is not period-aware — callers that need
// the value effective as of a given period should use the month package's
// resolution instead.
func LatestBaseAmount(amounts []BaseAmount) (BaseAmount, bool) {
	if len(amounts) == 0 {
		return BaseAmount{}, false
	}
	return amounts[len(amounts)-1], true
}

func scanBaseAmounts(rows *sql.Rows) ([]BaseAmount, error) {
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
