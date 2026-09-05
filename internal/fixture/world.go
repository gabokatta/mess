package fixture

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

const Year = 2026

var Period = domain.NewPeriod(Year, time.September)

// World is a database, described rather than built. Concepts are referenced
// by name, since their IDs do not exist until Load inserts them.
type World struct {
	Concepts   []Concept
	Entries    []Entry
	Notes      []catalog.Note
	Rates      []Rate
	FxHouse    domain.FxHouse
	LastExport *time.Time // nil means Load marks no export
}

// Concept defaults to monthly and to an active range starting in January of
// Year. Base is a decimal literal parsed by Load, empty on a Kind: Chore.
type Concept struct {
	Name     string
	Category string
	Kind     catalog.ConceptKind
	Currency domain.Currency
	Base     string
	Months   domain.Cadence
	From     domain.Period
	Until    domain.Period
}

// Entry is one concept's state in one period. Setting an Amount types it,
// which ticks it, so Done is only needed to tick a line at its base. An
// unmentioned concept leaves month_entry untouched, which is how a line
// nobody has reached yet is described.
type Entry struct {
	Concept string
	Period  domain.Period
	Amount  string
	Done    bool
}

// Rate is a decimal literal parsed by Load. Source zero (catalog.Close) is
// the common case; Manual marks a rate set by hand.
type Rate struct {
	Period domain.Period
	Value  string
	Source catalog.FxSource
}

// Loaded hands back every row Load created, keyed by the name a test used. A
// misspelled name yields a zero value whose ID 0 matches no real row.
type Loaded struct {
	Concepts   map[string]catalog.Concept
	Notes      map[string]catalog.Note
	Categories map[string]catalog.Category
}

// Load writes w through the real catalog functions. Categories are created on
// first mention in declaration order, which is a World's sort order.
func Load(db *sql.DB, w World) (Loaded, error) {
	loaded := Loaded{
		Concepts:   make(map[string]catalog.Concept, len(w.Concepts)),
		Notes:      make(map[string]catalog.Note, len(w.Notes)),
		Categories: make(map[string]catalog.Category),
	}

	for _, c := range w.Concepts {
		category, err := findOrCreateCategory(db, c.Category)
		if err != nil {
			return Loaded{}, fmt.Errorf("fixture: category %q: %w", c.Category, err)
		}
		loaded.Categories[c.Category] = category

		concept := catalog.Concept{
			Name:        c.Name,
			CategoryID:  category.ID,
			Kind:        c.Kind,
			MonthMask:   c.Months,
			ActiveFrom:  c.From,
			ActiveUntil: c.Until,
		}
		if concept.MonthMask == 0 {
			concept.MonthMask = domain.Monthly
		}
		if concept.ActiveFrom.IsZero() {
			concept.ActiveFrom = domain.NewPeriod(Year, time.January)
		}
		if c.Kind != catalog.Chore {
			literal := c.Base
			if literal == "" {
				literal = "0"
			}
			base, err := decimal.NewFromString(literal)
			if err != nil {
				return Loaded{}, fmt.Errorf("fixture: concept %q base %q: %w", c.Name, c.Base, err)
			}
			concept.Money = &catalog.MoneyDetails{Currency: c.Currency, Base: base}
		}

		created, err := catalog.CreateConcept(db, concept)
		if err != nil {
			return Loaded{}, fmt.Errorf("fixture: concept %q: %w", c.Name, err)
		}
		loaded.Concepts[c.Name] = created
	}

	for _, e := range w.Entries {
		concept, ok := loaded.Concepts[e.Concept]
		if !ok {
			return Loaded{}, fmt.Errorf("fixture: entry references unknown concept %q", e.Concept)
		}
		if e.Amount != "" {
			amount, err := decimal.NewFromString(e.Amount)
			if err != nil {
				return Loaded{}, fmt.Errorf("fixture: entry amount for %q %q: %w", e.Concept, e.Amount, err)
			}
			if err := catalog.SetMonthEntryAmount(db, concept.ID, e.Period, &amount); err != nil {
				return Loaded{}, err
			}
		}
		if e.Done {
			if err := catalog.SetMonthEntryDone(db, concept.ID, e.Period, true); err != nil {
				return Loaded{}, err
			}
		}
	}

	for _, n := range w.Notes {
		created, err := catalog.CreateNote(db, n)
		if err != nil {
			return Loaded{}, fmt.Errorf("fixture: note %q: %w", n.Title, err)
		}
		loaded.Notes[n.Title] = created
	}

	for _, r := range w.Rates {
		value, err := decimal.NewFromString(r.Value)
		if err != nil {
			return Loaded{}, fmt.Errorf("fixture: rate %s value %q: %w", r.Period, r.Value, err)
		}
		if r.Source == catalog.Manual {
			err = catalog.SetManualFxRate(db, r.Period, value)
		} else {
			err = catalog.SaveFxClose(db, r.Period, value, w.FxHouse)
		}
		if err != nil {
			return Loaded{}, err
		}
	}

	if err := catalog.SetFxHouse(db, w.FxHouse); err != nil {
		return Loaded{}, err
	}

	if w.LastExport != nil {
		if err := catalog.MarkExported(db, *w.LastExport); err != nil {
			return Loaded{}, err
		}
	}

	return loaded, nil
}

// findOrCreateCategory lets a world name its categories instead of creating
// them first. That convenience belongs to fixtures: the app creates a category
// deliberately, through catalog.AppendCategory, and has one path for it.
func findOrCreateCategory(db *sql.DB, name string) (catalog.Category, error) {
	categories, err := catalog.Categories(db)
	if err != nil {
		return catalog.Category{}, err
	}
	for _, c := range categories {
		if c.Name == name {
			return c, nil
		}
	}
	return catalog.AppendCategory(db, name)
}
