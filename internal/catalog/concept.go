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
	Chore
)

func (k ConceptKind) String() string {
	switch k {
	case Income:
		return "Income"
	case Expense:
		return "Expense"
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
	case "Chore":
		return Chore, nil
	default:
		return 0, fmt.Errorf("catalog: invalid concept kind %q", s)
	}
}

// MoneyDetails is a concept's monetary attributes: what currency it's in and
// what share of the household cost is yours. A Chore carries neither, so
// this lives behind a pointer rather than as fields every Concept has —
// nil is how "this concept has no money" gets represented, not a currency
// and share that just happen to be zero.
type MoneyDetails struct {
	Currency domain.Currency
	Share    domain.Percent
}

// Concept is a catalog entry: what a line is, not what it's worth in any
// given month. A money concept's amount lives in base_amount/month_entry,
// resolved per period elsewhere; a Chore has none to resolve.
type Concept struct {
	ID          int64
	Name        string
	CategoryID  int64
	Kind        ConceptKind
	Money       *MoneyDetails // nil for Kind: Chore
	MonthMask   domain.Cadence
	DueDay      int // 0 means unset
	SortOrder   int
	ActiveFrom  domain.Period
	ActiveUntil domain.Period // zero value means still active
}

// fullShare is the default a money concept's Share gets when left at its
// zero value: most concepts are entirely yours.
var fullShare = domain.NewPercent(100)

func CreateConcept(db *sql.DB, c Concept) (Concept, error) {
	c.normalizeShare()
	currency, share := c.nullableMoney()
	res, err := db.Exec(`
		INSERT INTO concept
			(name, category_id, kind, currency, month_mask, share, due_day, sort_order, active_from, active_until)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.CategoryID, c.Kind.String(), currency, int(c.MonthMask), share,
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

// normalizeShare fills the default 100% share on a money concept left at
// its zero value. A no-op for a Chore, which has no Money to default.
func (c *Concept) normalizeShare() {
	if c.Money != nil && c.Money.Share.Fraction().IsZero() {
		c.Money.Share = fullShare
	}
}

// nullableMoney reports c's currency and share as SQL-ready values, NULL for
// a Chore — the SQL boundary where Concept.Money's nil-ness becomes the
// column's NULL-ness.
func (c Concept) nullableMoney() (currency, share any) {
	if c.Money == nil {
		return nil, nil
	}
	return c.Money.Currency.String(), c.Money.Share.Fraction().String()
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
	c.normalizeShare()
	currency, share := c.nullableMoney()
	_, err := db.Exec(`
		UPDATE concept
		SET name = ?, category_id = ?, kind = ?, currency = ?, month_mask = ?, share = ?,
		    due_day = ?, sort_order = ?, active_from = ?, active_until = ?
		WHERE id = ?`,
		c.Name, c.CategoryID, c.Kind.String(), currency, int(c.MonthMask), share,
		nullableDueDay(c.DueDay), c.SortOrder, c.ActiveFrom.String(), nullablePeriod(c.ActiveUntil), c.ID)
	return err
}

// scanConcept parses a concept row into Concept, the boundary where a
// nullable currency/share column becomes Money's nil-ness — a Chore with a
// currency is a state that doesn't exist once past this point, even though
// the columns underneath still allow it.
func scanConcept(row *sql.Rows) (Concept, error) {
	var (
		c           Concept
		kind        string
		currency    sql.NullString
		monthMask   int
		share       sql.NullString
		dueDay      sql.NullInt64
		activeFrom  string
		activeUntil sql.NullString
	)
	if err := row.Scan(&c.ID, &c.Name, &c.CategoryID, &kind, &currency, &monthMask,
		&share, &dueDay, &c.SortOrder, &activeFrom, &activeUntil); err != nil {
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
		shareFraction, err := decimal.NewFromString(share.String)
		if err != nil {
			return Concept{}, err
		}
		c.Money = &MoneyDetails{Currency: cur, Share: domain.NewPercentFromFraction(shareFraction)}
	}
	c.MonthMask = domain.Cadence(monthMask)
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
