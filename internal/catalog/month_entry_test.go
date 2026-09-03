package catalog_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/fixture"
)

func TestMonthEntriesFiltersByPeriod(t *testing.T) {
	db := fixture.DB(t)
	loaded := fixture.MustLoad(t, db, fixture.World{
		Concepts: []fixture.Concept{{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "785000"}},
	})
	concept := loaded.Concepts["Rent"]
	sept, oct := fixture.Period, fixture.Period.AddMonths(1)

	amount := decimal.NewFromInt(800000)
	if err := catalog.SetMonthEntryAmount(db, concept.ID, sept, &amount); err != nil {
		t.Fatalf("SetMonthEntryAmount() unexpected error: %v", err)
	}
	if err := catalog.SetMonthEntryDone(db, concept.ID, sept, true); err != nil {
		t.Fatalf("SetMonthEntryDone() unexpected error: %v", err)
	}
	if err := catalog.SetMonthEntryDone(db, concept.ID, oct, false); err != nil {
		t.Fatalf("SetMonthEntryDone() unexpected error: %v", err)
	}

	got, err := catalog.MonthEntries(db, sept)
	if err != nil {
		t.Fatalf("MonthEntries(sept) unexpected error: %v", err)
	}
	if diff := cmp.Diff([]catalog.MonthEntry{{ConceptID: concept.ID, Period: sept, Amount: &amount, Done: true}}, got); diff != "" {
		t.Errorf("MonthEntries(sept) mismatch (-want +got):\n%s", diff)
	}

	got, err = catalog.MonthEntries(db, oct)
	if err != nil {
		t.Fatalf("MonthEntries(oct) unexpected error: %v", err)
	}
	if diff := cmp.Diff([]catalog.MonthEntry{{ConceptID: concept.ID, Period: oct, Done: false}}, got); diff != "" {
		t.Errorf("MonthEntries(oct) mismatch (-want +got):\n%s", diff)
	}
}

// setAmount "" skips the write and "clear" passes nil; wantAmount "" means
// no override is stored.
type step struct {
	setAmount string
	setDone   *bool

	wantAmount string
	wantDone   bool
}

func done(b bool) *bool { return &b }

// Done is the tick and the tick is what makes a line count, so typing a
// figure ticks with it: a correction that did not count would be the same
// trap as a base that could not be accepted. Everything else about the two
// columns stays independent, and unticking stays an explicit act.
func TestMonthEntryAmountAndDone(t *testing.T) {
	tests := []struct {
		name  string
		steps []step
	}{
		{
			name: "typing an amount ticks the line",
			steps: []step{
				{setAmount: "800000", wantAmount: "800000", wantDone: true},
			},
		},
		{
			name: "typing an amount keeps an existing tick",
			steps: []step{
				{setDone: done(true), wantDone: true},
				{setAmount: "800000", wantAmount: "800000", wantDone: true},
			},
		},
		{
			name: "ticking preserves an existing amount",
			steps: []step{
				{setAmount: "800000", wantAmount: "800000", wantDone: true},
				{setDone: done(true), wantAmount: "800000", wantDone: true},
			},
		},
		{
			name: "clearing back to the base leaves the tick standing",
			steps: []step{
				{setAmount: "800000", wantAmount: "800000", wantDone: true},
				{setAmount: "clear", wantDone: true},
			},
		},
		{
			name: "unticking a typed line stays unticked",
			steps: []step{
				{setAmount: "800000", wantAmount: "800000", wantDone: true},
				{setDone: done(false), wantAmount: "800000"},
			},
		},
		{
			name: "ticking alone stores no amount",
			steps: []step{
				{setDone: done(true), wantDone: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := fixture.DB(t)
			loaded := fixture.MustLoad(t, db, fixture.World{
				Concepts: []fixture.Concept{{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "785000"}},
			})
			concept := loaded.Concepts["Rent"]

			for i, s := range tt.steps {
				switch s.setAmount {
				case "":
				case "clear":
					if err := catalog.SetMonthEntryAmount(db, concept.ID, fixture.Period, nil); err != nil {
						t.Fatalf("step %d: SetMonthEntryAmount(nil) unexpected error: %v", i, err)
					}
				default:
					amount := decimal.RequireFromString(s.setAmount)
					if err := catalog.SetMonthEntryAmount(db, concept.ID, fixture.Period, &amount); err != nil {
						t.Fatalf("step %d: SetMonthEntryAmount() unexpected error: %v", i, err)
					}
				}
				if s.setDone != nil {
					if err := catalog.SetMonthEntryDone(db, concept.ID, fixture.Period, *s.setDone); err != nil {
						t.Fatalf("step %d: SetMonthEntryDone() unexpected error: %v", i, err)
					}
				}

				got, err := catalog.MonthEntries(db, fixture.Period)
				if err != nil {
					t.Fatalf("step %d: MonthEntries() unexpected error: %v", i, err)
				}
				if len(got) != 1 {
					t.Fatalf("step %d: MonthEntries() = %+v, want a single row", i, got)
				}

				want := catalog.MonthEntry{ConceptID: concept.ID, Period: fixture.Period, Done: s.wantDone}
				if s.wantAmount != "" {
					amount := decimal.RequireFromString(s.wantAmount)
					want.Amount = &amount
				}
				if diff := cmp.Diff(want, got[0]); diff != "" {
					t.Errorf("step %d: row mismatch (-want +got):\n%s", i, diff)
				}
			}
		})
	}
}
