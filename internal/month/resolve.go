package month

import (
	"database/sql"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mes/internal/catalog"
	"github.com/gabokatta/mes/internal/domain"
)

// Line is one concept resolved for a period, with whether its amount was
// confirmed (an override) or projected (from the base).
type Line struct {
	Concept   catalog.Concept
	Amount    decimal.Decimal
	Confirmed bool
	Done      bool
}

// Resolve projects concepts onto period per the resolution rules: occursIn
// gates inclusion, resolveAmount picks the value.
func Resolve(period domain.Period, concepts []catalog.Concept, bases map[int64][]catalog.BaseAmount, entries map[int64]catalog.MonthEntry) []Line {
	var lines []Line
	for _, c := range concepts {
		if !occursIn(c, period) {
			continue
		}
		entry := entries[c.ID]
		amt, confirmed := resolveAmount(period, bases[c.ID], entry)
		lines = append(lines, Line{Concept: c, Amount: amt, Confirmed: confirmed, Done: entry.Done})
	}
	return lines
}

// occursIn reports whether c generates a line at all in p, per its
// month_mask and active range.
func occursIn(c catalog.Concept, p domain.Period) bool {
	return occursInRange(c.MonthMask, c.ActiveFrom, c.ActiveUntil, p)
}

// choreOccursIn is occursIn's counterpart for chores, which carry the same
// month_mask/active range shape but aren't a Concept.
func choreOccursIn(c catalog.Chore, p domain.Period) bool {
	return occursInRange(c.MonthMask, c.ActiveFrom, c.ActiveUntil, p)
}

func occursInRange(mask domain.Cadence, from, until, p domain.Period) bool {
	if !mask.Occurs(p) {
		return false
	}
	if p.Before(from) {
		return false
	}
	if !until.IsZero() && p.After(until) {
		return false
	}
	return true
}

// resolveAmount picks the override if present, else the latest base at or
// before period. bases must be sorted ascending by EffectiveFrom.
func resolveAmount(p domain.Period, bases []catalog.BaseAmount, entry catalog.MonthEntry) (decimal.Decimal, bool) {
	if entry.Amount != nil {
		return *entry.Amount, true
	}
	var amount decimal.Decimal
	for _, b := range bases {
		if b.EffectiveFrom.After(p) {
			break
		}
		amount = b.Amount
	}
	return amount, false
}

// ChoreLine is one chore resolved for a period. Chores carry no amount, so
// they don't share Line's shape with concepts.
type ChoreLine struct {
	Chore catalog.Chore
	Done  bool
}

// ResolveChores projects chores onto period the same way Resolve projects
// concepts: choreOccursIn gates inclusion, the entry (if any) supplies Done.
func ResolveChores(period domain.Period, chores []catalog.Chore, entries map[int64]catalog.ChoreEntry) []ChoreLine {
	var lines []ChoreLine
	for _, c := range chores {
		if !choreOccursIn(c, period) {
			continue
		}
		lines = append(lines, ChoreLine{Chore: c, Done: entries[c.ID].Done})
	}
	return lines
}

// Month is a period's resolved view: concept lines and chore lines, the two
// kinds the month view merges.
type Month struct {
	Lines  []Line
	Chores []ChoreLine
}

// Load resolves every active concept and chore for period, reading the
// catalog and that period's entries from db.
func Load(db *sql.DB, period domain.Period) (Month, error) {
	concepts, err := catalog.Concepts(db)
	if err != nil {
		return Month{}, err
	}
	bases, err := catalog.AllBaseAmounts(db)
	if err != nil {
		return Month{}, err
	}
	entryRows, err := catalog.MonthEntries(db, period)
	if err != nil {
		return Month{}, err
	}
	entries := make(map[int64]catalog.MonthEntry, len(entryRows))
	for _, e := range entryRows {
		entries[e.ConceptID] = e
	}

	chores, err := catalog.Chores(db)
	if err != nil {
		return Month{}, err
	}
	choreEntryRows, err := catalog.ChoreEntries(db, period)
	if err != nil {
		return Month{}, err
	}
	choreEntries := make(map[int64]catalog.ChoreEntry, len(choreEntryRows))
	for _, e := range choreEntryRows {
		choreEntries[e.ChoreID] = e
	}

	return Month{
		Lines:  Resolve(period, concepts, bases, entries),
		Chores: ResolveChores(period, chores, choreEntries),
	}, nil
}
