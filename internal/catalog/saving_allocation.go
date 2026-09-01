package catalog

import (
	"database/sql"
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
)

// Destination is where an allocation goes. Net worth folds Cash and
// Invested separately; nothing else exists to allocate to.
type Destination int

const (
	Cash Destination = iota
	Invested
)

func (d Destination) String() string {
	switch d {
	case Cash:
		return "Cash"
	case Invested:
		return "Invested"
	default:
		return fmt.Sprintf("Destination(%d)", int(d))
	}
}

func ParseDestination(s string) (Destination, error) {
	switch s {
	case "Cash":
		return Cash, nil
	case "Invested":
		return Invested, nil
	default:
		return 0, fmt.Errorf("catalog: invalid destination %q", s)
	}
}

// SavingAllocation is one move of a period's available-to-save into Cash
// or Invested. Amount and Currency are free-form — it may exceed the
// remainder or go negative; the app shows the gap rather than blocking it.
type SavingAllocation struct {
	ID          int64
	Period      domain.Period
	Destination Destination
	Amount      decimal.Decimal
	Currency    domain.Currency
}

func CreateSavingAllocation(db *sql.DB, a SavingAllocation) (SavingAllocation, error) {
	res, err := db.Exec(`
		INSERT INTO saving_allocation (period, destination, amount, currency)
		VALUES (?, ?, ?, ?)`,
		a.Period.String(), a.Destination.String(), a.Amount.String(), a.Currency.String())
	if err != nil {
		return SavingAllocation{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return SavingAllocation{}, err
	}
	a.ID = id
	return a, nil
}

func SavingAllocations(db *sql.DB, period domain.Period) ([]SavingAllocation, error) {
	rows, err := db.Query(`
		SELECT id, period, destination, amount, currency FROM saving_allocation
		WHERE period = ? ORDER BY id`, period.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSavingAllocations(rows)
}

// AllSavingAllocations loads every allocation ever recorded, ordered by
// period, for folding into net worth and leftover pesos.
func AllSavingAllocations(db *sql.DB) ([]SavingAllocation, error) {
	rows, err := db.Query(`
		SELECT id, period, destination, amount, currency FROM saving_allocation
		ORDER BY period, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSavingAllocations(rows)
}

func scanSavingAllocations(rows *sql.Rows) ([]SavingAllocation, error) {
	var allocations []SavingAllocation
	for rows.Next() {
		var (
			a                                     SavingAllocation
			periodStr, destStr, amountStr, curStr string
		)
		if err := rows.Scan(&a.ID, &periodStr, &destStr, &amountStr, &curStr); err != nil {
			return nil, err
		}
		var err error
		if a.Period, err = domain.ParsePeriod(periodStr); err != nil {
			return nil, err
		}
		if a.Destination, err = ParseDestination(destStr); err != nil {
			return nil, err
		}
		if a.Amount, err = decimal.NewFromString(amountStr); err != nil {
			return nil, err
		}
		if a.Currency, err = domain.ParseCurrency(curStr); err != nil {
			return nil, err
		}
		allocations = append(allocations, a)
	}
	return allocations, rows.Err()
}
