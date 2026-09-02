package month

import (
	"database/sql"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

// LineMoney is a resolved line's amount and whether it was confirmed (an
// override) or projected (from the base) — nil on a Chore line, which has
// nothing to resolve an amount for.
type LineMoney struct {
	Amount    decimal.Decimal
	Confirmed bool
}

// Line is one concept resolved for a period. Money is nil for a Chore line;
// Done applies to every kind alike, since ticking a line already means "I
// did this" regardless of whether it also carries an amount.
type Line struct {
	Concept catalog.Concept
	Money   *LineMoney
	Done    bool
}

// Resolve projects concepts onto period per the resolution rules: occursIn
// gates inclusion, resolveAmount picks the value for a money concept. One
// pipeline for every kind — a Chore differs only in carrying no Money.
func Resolve(period domain.Period, concepts []catalog.Concept, bases map[int64][]catalog.BaseAmount, entries map[int64]catalog.MonthEntry) []Line {
	var lines []Line
	for _, c := range concepts {
		if !occursIn(c, period) {
			continue
		}
		entry := entries[c.ID]
		line := Line{Concept: c, Done: entry.Done}
		if c.Money != nil {
			amt, confirmed := resolveAmount(period, bases[c.ID], entry)
			line.Money = &LineMoney{Amount: amt, Confirmed: confirmed}
		}
		lines = append(lines, line)
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

// UnfinishedChores counts Chore-kind lines that never got marked done — the
// "Last month: N unfinished" the following month's view surfaces.
func UnfinishedChores(lines []Line) int {
	n := 0
	for _, l := range lines {
		if l.Concept.Kind == catalog.Chore && !l.Done {
			n++
		}
	}
	return n
}

// ChoresDone counts how many of this period's Chore-kind lines are done,
// out of how many there are — the "X of Y chores done" the month header
// surfaces alongside the money lines' confirmed count.
func ChoresDone(lines []Line) (done, total int) {
	for _, l := range lines {
		if l.Concept.Kind != catalog.Chore {
			continue
		}
		total++
		if l.Done {
			done++
		}
	}
	return done, total
}

// Month is a period's resolved view: every active concept's line, money and
// chore alike.
type Month struct {
	Lines []Line
}

// Load resolves every active concept for period, reading the catalog and
// that period's entries from db.
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

	return Month{Lines: Resolve(period, concepts, bases, entries)}, nil
}
