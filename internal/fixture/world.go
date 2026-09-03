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

// World is a database, described rather than built. Concepts are
// referenced by name throughout, since their IDs do not exist until Load
// inserts them.
type World struct {
	Concepts   []Concept
	Entries    []Entry
	Notes      []catalog.Note
	Rates      []Rate
	FxHouse    domain.FxHouse
	LastExport *time.Time // nil means Load never marks an export
}

// Concept defaults to monthly and to an active range starting in January of
// Year, so a test states only the fields it cares about. Base is a decimal
// literal parsed by Load; empty on a Kind: Chore, which carries no money.
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

// Entry overrides one concept in one period. An empty Amount means no
// override on the amount; Done false means no override on done. A World
// that never mentions a concept for a period leaves month_entry untouched,
// which is the "no entry at all" state Entry's zero value cannot express on
// its own.
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

// Loaded hands back every row Load created, keyed by the name a test
// referenced it by. A misspelled name comes back as a zero value whose ID
// is 0, which no real row has, so the mistake fails the very next
// assertion instead of silently matching the wrong row.
type Loaded struct {
	Concepts   map[string]catalog.Concept
	Notes      map[string]catalog.Note
	Categories map[string]catalog.Category
}

// Load writes w through the real catalog functions and hands back what it
// created. Categories are created on first mention among Concepts, in
// declaration order, so that order is a World's sort order without a
// second list to keep in sync.
func Load(db *sql.DB, w World) (Loaded, error) {
	loaded := Loaded{
		Concepts:   make(map[string]catalog.Concept, len(w.Concepts)),
		Notes:      make(map[string]catalog.Note, len(w.Notes)),
		Categories: make(map[string]catalog.Category),
	}

	for _, c := range w.Concepts {
		category, err := catalog.FindOrCreateCategory(db, c.Category)
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
			err = catalog.SaveFxClose(db, r.Period, value)
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
