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
	if !c.MonthMask.Occurs(p) {
		return false
	}
	if p.Before(c.ActiveFrom) {
		return false
	}
	if !c.ActiveUntil.IsZero() && p.After(c.ActiveUntil) {
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

// Load resolves every active concept for period, reading the catalog and
// that period's overrides from db.
func Load(db *sql.DB, period domain.Period) ([]Line, error) {
	concepts, err := catalog.Concepts(db)
	if err != nil {
		return nil, err
	}
	bases, err := catalog.AllBaseAmounts(db)
	if err != nil {
		return nil, err
	}
	entryRows, err := catalog.MonthEntries(db, period)
	if err != nil {
		return nil, err
	}
	entries := make(map[int64]catalog.MonthEntry, len(entryRows))
	for _, e := range entryRows {
		entries[e.ConceptID] = e
	}
	return Resolve(period, concepts, bases, entries), nil
}
