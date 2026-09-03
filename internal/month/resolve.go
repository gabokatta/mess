package month

import (
	"database/sql"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

// LineMoney's Confirmed is the presence of a month_entry override and
// nothing else; unconfirmed means the amount is still the concept's base.
type LineMoney struct {
	Amount    domain.Money
	Confirmed bool
}

// Line is one concept resolved for a period. Money is nil on a Chore; Done
// applies to every kind.
type Line struct {
	Concept catalog.Concept
	Money   *LineMoney
	Done    bool
}

type Month struct {
	Lines []Line
}

// Resolve runs one pipeline for every kind: month_mask and the active range
// decide whether a concept generates a line, the override decides its amount.
func Resolve(period domain.Period, concepts []catalog.Concept, entries map[int64]catalog.MonthEntry) []Line {
	var lines []Line
	for _, c := range concepts {
		if !occursIn(c, period) {
			continue
		}
		entry := entries[c.ID]
		line := Line{Concept: c, Done: entry.Done}
		if c.Money != nil {
			amount, confirmed := c.Money.Base, false
			if entry.Amount != nil {
				amount, confirmed = *entry.Amount, true
			}
			line.Money = &LineMoney{
				Amount:    domain.NewMoney(amount, c.Money.Currency),
				Confirmed: confirmed,
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

// DoneCount counts every kind alike.
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
