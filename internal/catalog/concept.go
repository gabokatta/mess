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
)

func (k ConceptKind) String() string {
	switch k {
	case Income:
		return "Income"
	case Expense:
		return "Expense"
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
	default:
		return 0, fmt.Errorf("catalog: invalid concept kind %q", s)
	}
}

// Concept is a catalog entry: what a line is, not what it's worth in any
// given month. Its amount lives in base_amount/month_entry, resolved
// per period elsewhere.
type Concept struct {
	ID          int64
	Name        string
	CategoryID  int64
	Kind        ConceptKind
	Currency    domain.Currency
	MonthMask   domain.Cadence
	Share       domain.Percent
	DueDay      int // 0 means unset
	SortOrder   int
	ActiveFrom  domain.Period
	ActiveUntil domain.Period // zero value means still active
}

// fullShare is the default a concept's Share gets when left at its zero
// value: most concepts are entirely yours.
var fullShare = domain.NewPercent(100)

func CreateConcept(db *sql.DB, c Concept) (Concept, error) {
	if c.Share.Fraction().IsZero() {
		c.Share = fullShare
	}
	res, err := db.Exec(`
		INSERT INTO concept
			(name, category_id, kind, currency, month_mask, share, due_day, sort_order, active_from, active_until)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.CategoryID, c.Kind.String(), c.Currency.String(), int(c.MonthMask), c.Share.Fraction().String(),
		nullableDueDay(c.DueDay), c.SortOrder, c.ActiveFrom.String(), nullablePeriod(c.ActiveUntil))
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

func Concepts(db *sql.DB) ([]Concept, error) {
	rows, err := db.Query(`
		SELECT id, name, category_id, kind, currency, month_mask, share, due_day, sort_order, active_from, active_until
		FROM concept ORDER BY sort_order, name`)
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

func UpdateConcept(db *sql.DB, c Concept) error {
	if c.Share.Fraction().IsZero() {
		c.Share = fullShare
	}
	_, err := db.Exec(`
		UPDATE concept
		SET name = ?, category_id = ?, kind = ?, currency = ?, month_mask = ?, share = ?,
		    due_day = ?, sort_order = ?, active_from = ?, active_until = ?
		WHERE id = ?`,
		c.Name, c.CategoryID, c.Kind.String(), c.Currency.String(), int(c.MonthMask), c.Share.Fraction().String(),
		nullableDueDay(c.DueDay), c.SortOrder, c.ActiveFrom.String(), nullablePeriod(c.ActiveUntil), c.ID)
	return err
}

func scanConcept(row *sql.Rows) (Concept, error) {
	var (
		c              Concept
		kind, currency string
		monthMask      int
		share          string
		dueDay         sql.NullInt64
		activeFrom     string
		activeUntil    sql.NullString
	)
	if err := row.Scan(&c.ID, &c.Name, &c.CategoryID, &kind, &currency, &monthMask,
		&share, &dueDay, &c.SortOrder, &activeFrom, &activeUntil); err != nil {
		return Concept{}, err
	}

	var err error
	if c.Kind, err = ParseConceptKind(kind); err != nil {
		return Concept{}, err
	}
	if c.Currency, err = domain.ParseCurrency(currency); err != nil {
		return Concept{}, err
	}
	c.MonthMask = domain.Cadence(monthMask)
	shareFraction, err := decimal.NewFromString(share)
	if err != nil {
		return Concept{}, err
	}
	c.Share = domain.NewPercentFromFraction(shareFraction)
	c.DueDay = int(dueDay.Int64)
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

func nullableDueDay(d int) any {
	if d == 0 {
		return nil
	}
	return d
}

func nullablePeriod(p domain.Period) any {
	if p.IsZero() {
		return nil
	}
	return p.String()
}
