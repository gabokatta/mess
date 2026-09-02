package month

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func concept(id int64, mask domain.Cadence, from, until domain.Period) catalog.Concept {
	return catalog.Concept{
		ID:          id,
		Name:        "Alquiler",
		Kind:        catalog.Expense,
		Currency:    domain.ARS,
		MonthMask:   mask,
		ActiveFrom:  from,
		ActiveUntil: until,
	}
}

func amount(v int64) decimal.Decimal { return decimal.NewFromInt(v) }

func TestResolveUsesLatestBaseAtOrBeforePeriod(t *testing.T) {
	sept := domain.NewPeriod(2026, time.September)
	c := concept(1, domain.Monthly, domain.NewPeriod(2026, time.January), domain.Period{})
	bases := map[int64][]catalog.BaseAmount{
		1: {
			{ConceptID: 1, EffectiveFrom: domain.NewPeriod(2026, time.January), Amount: amount(785000)},
			{ConceptID: 1, EffectiveFrom: domain.NewPeriod(2026, time.June), Amount: amount(850000)},
			{ConceptID: 1, EffectiveFrom: domain.NewPeriod(2026, time.December), Amount: amount(900000)},
		},
	}

	lines := Resolve(sept, []catalog.Concept{c}, bases, nil)

	if len(lines) != 1 {
		t.Fatalf("Resolve() = %d lines, want 1", len(lines))
	}
	if !lines[0].Amount.Equal(amount(850000)) {
		t.Errorf("Amount = %s, want 850000 (June base, latest at or before September)", lines[0].Amount)
	}
	if lines[0].Confirmed {
		t.Error("Confirmed = true, want false (no override, resolved from base)")
	}
}

func TestResolveOverrideWinsOverBase(t *testing.T) {
	sept := domain.NewPeriod(2026, time.September)
	c := concept(1, domain.Monthly, domain.NewPeriod(2026, time.January), domain.Period{})
	bases := map[int64][]catalog.BaseAmount{
		1: {{ConceptID: 1, EffectiveFrom: domain.NewPeriod(2026, time.January), Amount: amount(785000)}},
	}
	override := amount(792000)
	entries := map[int64]catalog.MonthEntry{
		1: {ConceptID: 1, Period: sept, Amount: &override, Done: true},
	}

	lines := Resolve(sept, []catalog.Concept{c}, bases, entries)

	if len(lines) != 1 {
		t.Fatalf("Resolve() = %d lines, want 1", len(lines))
	}
	if !lines[0].Amount.Equal(override) {
		t.Errorf("Amount = %s, want %s (override)", lines[0].Amount, override)
	}
	if !lines[0].Confirmed {
		t.Error("Confirmed = false, want true (an override is present)")
	}
	if !lines[0].Done {
		t.Error("Done = false, want true")
	}
}

func TestResolveNoBaseYetsResolvesToZeroUnconfirmed(t *testing.T) {
	sept := domain.NewPeriod(2026, time.September)
	c := concept(1, domain.Monthly, domain.NewPeriod(2026, time.January), domain.Period{})

	lines := Resolve(sept, []catalog.Concept{c}, nil, nil)

	if len(lines) != 1 {
		t.Fatalf("Resolve() = %d lines, want 1", len(lines))
	}
	if !lines[0].Amount.IsZero() {
		t.Errorf("Amount = %s, want 0 (no base, no override)", lines[0].Amount)
	}
	if lines[0].Confirmed {
		t.Error("Confirmed = true, want false")
	}
}

func TestResolveExcludesConceptsOutsideMonthMask(t *testing.T) {
	sept := domain.NewPeriod(2026, time.September)
	c := concept(1, domain.Aguinaldo, domain.NewPeriod(2026, time.January), domain.Period{})

	lines := Resolve(sept, []catalog.Concept{c}, nil, nil)

	if len(lines) != 0 {
		t.Errorf("Resolve() = %d lines, want 0 (September is not June/December)", len(lines))
	}
}

func TestResolveExcludesConceptsBeforeActiveFrom(t *testing.T) {
	sept := domain.NewPeriod(2026, time.September)
	c := concept(1, domain.Monthly, domain.NewPeriod(2026, time.October), domain.Period{})

	lines := Resolve(sept, []catalog.Concept{c}, nil, nil)

	if len(lines) != 0 {
		t.Errorf("Resolve() = %d lines, want 0 (concept starts in October)", len(lines))
	}
}

func TestResolveExcludesConceptsAfterActiveUntil(t *testing.T) {
	sept := domain.NewPeriod(2026, time.September)
	c := concept(1, domain.Monthly, domain.NewPeriod(2026, time.January), domain.NewPeriod(2026, time.August))

	lines := Resolve(sept, []catalog.Concept{c}, nil, nil)

	if len(lines) != 0 {
		t.Errorf("Resolve() = %d lines, want 0 (concept retired in August)", len(lines))
	}
}

func TestResolveIncludesConceptOnActiveUntilBoundary(t *testing.T) {
	march := domain.NewPeriod(2026, time.March)
	c := concept(1, domain.Monthly, march, march)

	lines := Resolve(march, []catalog.Concept{c}, nil, nil)

	if len(lines) != 1 {
		t.Errorf("Resolve() = %d lines, want 1 (a one-off month is its own start and end)", len(lines))
	}
}

func TestResolveIgnoresFutureBases(t *testing.T) {
	march := domain.NewPeriod(2026, time.March)
	c := concept(1, domain.Monthly, domain.NewPeriod(2026, time.January), domain.Period{})
	bases := map[int64][]catalog.BaseAmount{
		1: {{ConceptID: 1, EffectiveFrom: domain.NewPeriod(2026, time.June), Amount: amount(850000)}},
	}

	lines := Resolve(march, []catalog.Concept{c}, bases, nil)

	if len(lines) != 1 {
		t.Fatalf("Resolve() = %d lines, want 1", len(lines))
	}
	if !lines[0].Amount.IsZero() {
		t.Errorf("Amount = %s, want 0 (only base is in the future)", lines[0].Amount)
	}
}
