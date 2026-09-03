package month

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func ars(n int64) decimal.Decimal { return decimal.NewFromInt(n) }

func concept(id int64, kind catalog.ConceptKind, base int64, opts ...func(*catalog.Concept)) catalog.Concept {
	c := catalog.Concept{
		ID:         id,
		Name:       "concept",
		Kind:       kind,
		MonthMask:  domain.Monthly,
		ActiveFrom: domain.NewPeriod(2026, time.January),
	}
	if kind != catalog.Chore {
		c.Money = &catalog.MoneyDetails{Currency: domain.ARS, Base: ars(base)}
	}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

func TestResolveUsesTheOverrideWhenPresent(t *testing.T) {
	period := domain.NewPeriod(2026, time.September)
	typed := ars(812000)

	lines := Resolve(period,
		[]catalog.Concept{concept(1, catalog.Expense, 785000)},
		map[int64]catalog.MonthEntry{1: {ConceptID: 1, Period: period, Amount: &typed, Done: true}},
	)

	if len(lines) != 1 {
		t.Fatalf("Resolve() returned %d lines, want 1", len(lines))
	}
	if !lines[0].Money.Amount.Amount().Equal(typed) {
		t.Errorf("amount = %s, want the override 812000", lines[0].Money.Amount.Amount())
	}
	if !lines[0].Money.Confirmed {
		t.Error("a line backed by an override is confirmed")
	}
	if !lines[0].Done {
		t.Error("Done should come from the entry")
	}
}

// Without an override the line shows the concept's base amount, and it is
// not confirmed: the presence of the override is what "confirmed" means.
func TestResolveFallsBackToTheBaseAmount(t *testing.T) {
	period := domain.NewPeriod(2026, time.September)

	lines := Resolve(period, []catalog.Concept{concept(1, catalog.Expense, 785000)}, nil)

	if !lines[0].Money.Amount.Amount().Equal(ars(785000)) {
		t.Errorf("amount = %s, want the base 785000", lines[0].Money.Amount.Amount())
	}
	if lines[0].Money.Confirmed {
		t.Error("a line with no override is not confirmed")
	}
}

func TestResolveGivesAChoreNoMoney(t *testing.T) {
	lines := Resolve(domain.NewPeriod(2026, time.September),
		[]catalog.Concept{concept(1, catalog.Chore, 0)}, nil)

	if lines[0].Money != nil {
		t.Errorf("chore line Money = %+v, want nil", lines[0].Money)
	}
}

// Ticking means "I did this" for every kind, so done travels with a chore
// exactly as it does with a bill.
func TestResolveMarksAChoreDone(t *testing.T) {
	period := domain.NewPeriod(2026, time.September)

	lines := Resolve(period, []catalog.Concept{concept(1, catalog.Chore, 0)},
		map[int64]catalog.MonthEntry{1: {ConceptID: 1, Period: period, Done: true}})

	if !lines[0].Done {
		t.Error("a chore with a done entry should resolve as done")
	}
}

func TestResolveOccurrence(t *testing.T) {
	june := domain.NewPeriod(2026, time.June)
	september := domain.NewPeriod(2026, time.September)

	tests := []struct {
		name    string
		concept catalog.Concept
		period  domain.Period
		want    bool
	}{
		{
			name:    "monthly occurs every month",
			concept: concept(1, catalog.Expense, 100),
			period:  september,
			want:    true,
		},
		{
			name: "aguinaldo skips a month outside its mask",
			concept: concept(1, catalog.Income, 100, func(c *catalog.Concept) {
				c.MonthMask = domain.Aguinaldo
			}),
			period: september,
			want:   false,
		},
		{
			name: "aguinaldo occurs in june",
			concept: concept(1, catalog.Income, 100, func(c *catalog.Concept) {
				c.MonthMask = domain.Aguinaldo
			}),
			period: june,
			want:   true,
		},
		{
			name: "before active_from",
			concept: concept(1, catalog.Expense, 100, func(c *catalog.Concept) {
				c.ActiveFrom = domain.NewPeriod(2026, time.October)
			}),
			period: september,
			want:   false,
		},
		{
			name: "after active_until",
			concept: concept(1, catalog.Expense, 100, func(c *catalog.Concept) {
				c.ActiveUntil = domain.NewPeriod(2026, time.June)
			}),
			period: september,
			want:   false,
		},
		{
			name: "a one-off is its own single month",
			concept: concept(1, catalog.Saving, 100, func(c *catalog.Concept) {
				c.MonthMask = domain.NewCadence(time.September)
				c.ActiveFrom = september
				c.ActiveUntil = september
			}),
			period: september,
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := Resolve(tt.period, []catalog.Concept{tt.concept}, nil)
			if got := len(lines) == 1; got != tt.want {
				t.Errorf("occurs in %s = %v, want %v", tt.period, got, tt.want)
			}
		})
	}
}

func TestDoneCountCoversEveryKind(t *testing.T) {
	period := domain.NewPeriod(2026, time.September)
	lines := Resolve(period,
		[]catalog.Concept{
			concept(1, catalog.Expense, 100),
			concept(2, catalog.Chore, 0),
			concept(3, catalog.Saving, 400),
		},
		map[int64]catalog.MonthEntry{
			1: {ConceptID: 1, Done: true},
			2: {ConceptID: 2, Done: true},
		})

	done, total := DoneCount(lines)
	if done != 2 {
		t.Errorf("DoneCount() done = %d, want 2", done)
	}
	if total != 3 {
		t.Errorf("DoneCount() total = %d, want 3", total)
	}
}
