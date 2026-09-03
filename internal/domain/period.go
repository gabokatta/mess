package domain

import (
	"fmt"
	"time"
)

// Period is a calendar month ("2026-09"), the app's unit of time.
// Periods are totally ordered.
type Period struct {
	year  int
	month time.Month
}

func NewPeriod(year int, month time.Month) Period {
	return Period{year: year, month: month}
}

func PeriodFromTime(t time.Time) Period {
	return Period{year: t.Year(), month: t.Month()}
}

func ParsePeriod(s string) (Period, error) {
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return Period{}, fmt.Errorf("domain: invalid period %q: %w", s, err)
	}
	return PeriodFromTime(t), nil
}

func (p Period) Year() int         { return p.year }
func (p Period) Month() time.Month { return p.month }

func (p Period) String() string {
	return fmt.Sprintf("%04d-%02d", p.year, int(p.month))
}

func (p Period) index() int { return p.year*12 + int(p.month) }

// IsZero reports whether p is the zero value, the "unbounded" sentinel for
// open-ended active ranges.
func (p Period) IsZero() bool { return p == Period{} }

func (p Period) Before(other Period) bool { return p.index() < other.index() }
func (p Period) After(other Period) bool  { return p.index() > other.index() }
func (p Period) Equal(other Period) bool  { return p.index() == other.index() }

// AddMonths returns the period n months away, n may be negative.
func (p Period) AddMonths(n int) Period {
	idx := p.index() + n - 1
	year := idx / 12
	month := time.Month(idx%12 + 1)
	return Period{year: year, month: month}
}
