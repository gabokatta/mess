package catalog

import (
	"database/sql"
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
)

type ConceptKind int

const (
	Income ConceptKind = iota
	Expense
	Saving
	Chore
)

func (k ConceptKind) String() string {
	switch k {
	case Income:
		return "Income"
	case Expense:
		return "Expense"
	case Saving:
		return "Saving"
	case Chore:
		return "Chore"
	default:
		return fmt.Sprintf("ConceptKind(%d)", int(k))
	}
}

func ParseConceptKind(s string) (ConceptKind, error) {
	switch s {
	case "Income":
		return Income, nil
	case "Expense":
		return Expense, nil
	case "Saving":
		return Saving, nil
	case "Chore":
		return Chore, nil
	default:
		return 0, fmt.Errorf("catalog: invalid concept kind %q", s)
	}
}

// MoneyDetails hangs off a pointer so nil, not a zero amount, represents a
// Chore's absence of money.
type MoneyDetails struct {
	Currency domain.Currency
	Base     decimal.Decimal
}

// Concept is what a line is, not what it cost: Base only prefills the edit
// box; the typed amount lives in month_entry.
type Concept struct {
	ID          int64
	Name        string
	CategoryID  int64
	Kind        ConceptKind
	Money       *MoneyDetails // nil for Kind: Chore
	MonthMask   domain.Cadence
	ActiveFrom  domain.Period
	ActiveUntil domain.Period // zero value means still active
}

func CreateConcept(db *sql.DB, c Concept) (Concept, error) {
	currency, base := c.nullableMoney()
	res, err := db.Exec(`
		INSERT INTO concept
			(name, category_id, kind, currency, base_amount, month_mask, active_from, active_until)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.CategoryID, c.Kind.String(), currency, base, int(c.MonthMask),
		c.ActiveFrom.String(), nullablePeriod(c.ActiveUntil))
	if err != nil {
		return Concept{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Concept{}, err
	}
	c.ID = id
	return c, nil
}

func UpdateConcept(db *sql.DB, c Concept) error {
	currency, base := c.nullableMoney()
	_, err := db.Exec(`
		UPDATE concept
		SET name = ?, category_id = ?, kind = ?, currency = ?, base_amount = ?,
		    month_mask = ?, active_from = ?, active_until = ?
		WHERE id = ?`,
		c.Name, c.CategoryID, c.Kind.String(), currency, base, int(c.MonthMask),
		c.ActiveFrom.String(), nullablePeriod(c.ActiveUntil), c.ID)
	return err
}

func DeleteConcept(db *sql.DB, id int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM month_entry WHERE concept_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM concept WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// Concepts orders by category then name, the order every view lists in.
func Concepts(db *sql.DB) ([]Concept, error) {
	rows, err := db.Query(`
		SELECT c.id, c.name, c.category_id, c.kind, c.currency, c.base_amount,
		       c.month_mask, c.active_from, c.active_until
		FROM concept c JOIN category cat ON cat.id = c.category_id
		ORDER BY cat.sort_order, cat.name, c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var concepts []Concept
	for rows.Next() {
		c, err := scanConcept(rows)
		if err != nil {
			return nil, err
		}
		concepts = append(concepts, c)
	}
	return concepts, rows.Err()
}

func (c Concept) nullableMoney() (currency, base any) {
	if c.Money == nil {
		return nil, nil
	}
	return c.Money.Currency.String(), c.Money.Base.String()
}

func scanConcept(row *sql.Rows) (Concept, error) {
	var (
		c           Concept
		kind        string
		currency    sql.NullString
		base        sql.NullString
		monthMask   int
		activeFrom  string
		activeUntil sql.NullString
	)
	if err := row.Scan(&c.ID, &c.Name, &c.CategoryID, &kind, &currency, &base,
		&monthMask, &activeFrom, &activeUntil); err != nil {
		return Concept{}, err
	}

	var err error
	if c.Kind, err = ParseConceptKind(kind); err != nil {
		return Concept{}, err
	}
	if currency.Valid {
		cur, err := domain.ParseCurrency(currency.String)
		if err != nil {
			return Concept{}, err
		}
		amount, err := decimal.NewFromString(base.String)
		if err != nil {
			return Concept{}, err
		}
		c.Money = &MoneyDetails{Currency: cur, Base: amount}
	}
	c.MonthMask = domain.Cadence(monthMask)
	if c.ActiveFrom, err = domain.ParsePeriod(activeFrom); err != nil {
		return Concept{}, err
	}
	if activeUntil.Valid {
		if c.ActiveUntil, err = domain.ParsePeriod(activeUntil.String); err != nil {
			return Concept{}, err
		}
	}
	return c, nil
}

func nullablePeriod(p domain.Period) any {
	if p.IsZero() {
		return nil
	}
	return p.String()
}
