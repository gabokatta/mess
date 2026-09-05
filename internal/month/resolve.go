package month

import (
	"database/sql"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

// Overridden records a typed amount; Line.Done determines whether it counts.
type LineMoney struct {
	Amount     domain.Money
	Overridden bool
}

// Money is nil for chores. Done confirms either the base or the override.
type Line struct {
	Concept catalog.Concept
	Money   *LineMoney
	Done    bool
}

type Month struct {
	Lines []Line
}

func Resolve(period domain.Period, concepts []catalog.Concept, entries map[int64]catalog.MonthEntry) []Line {
	var lines []Line
	for _, c := range concepts {
		if !occursIn(c, period) {
			continue
		}
		entry := entries[c.ID]
		line := Line{Concept: c, Done: entry.Done}
		if c.Money != nil {
			amount, overridden := c.Money.Base, false
			if entry.Amount != nil {
				amount, overridden = *entry.Amount, true
			}
			line.Money = &LineMoney{
				Amount:     domain.NewMoney(amount, c.Money.Currency),
				Overridden: overridden,
			}
		}
		lines = append(lines, line)
	}
	return lines
}

func occursIn(c catalog.Concept, p domain.Period) bool {
	if !c.MonthMask.Occurs(p) {
		return false
	}
	if p.Before(c.ActiveFrom) {
		return false
	}
	return c.ActiveUntil.IsZero() || !p.After(c.ActiveUntil)
}

func DoneCount(lines []Line) (done, total int) {
	for _, l := range lines {
		total++
		if l.Done {
			done++
		}
	}
	return done, total
}

func Load(db *sql.DB, period domain.Period) (Month, error) {
	concepts, err := catalog.Concepts(db)
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
	return Month{Lines: Resolve(period, concepts, entries)}, nil
}
