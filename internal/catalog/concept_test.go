package catalog

import (
	"database/sql"
	"testing"
	"time"

	"github.com/gabokatta/mes/internal/domain"
)

func mustCategory(t *testing.T, db *sql.DB) Category {
	t.Helper()
	cat, err := CreateCategory(db, "Servicios", 0)
	if err != nil {
		t.Fatalf("CreateCategory() unexpected error: %v", err)
	}
	return cat
}

func TestCreateAndListConcepts(t *testing.T) {
	db := openTestStore(t).DB()
	cat := mustCategory(t, db)

	c := Concept{
		Name:        "Alquiler",
		CategoryID:  cat.ID,
		Kind:        FixedExpense,
		Currency:    domain.ARS,
		MonthMask:   domain.Monthly,
		Share:       domain.NewPercent(50),
		DueDay:      10,
		SortOrder:   1,
		ActiveFrom:  domain.NewPeriod(2026, time.January),
		ActiveUntil: domain.Period{},
	}

	created, err := CreateConcept(db, c)
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}
	if created.ID == 0 {
		t.Error("CreateConcept() should assign a non-zero ID")
	}

	got, err := Concepts(db)
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Concepts() returned %d rows, want 1", len(got))
	}

	want := created
	g := got[0]
	if g.ID != want.ID || g.Name != want.Name || g.CategoryID != want.CategoryID ||
		g.Kind != want.Kind || g.Currency != want.Currency || g.MonthMask != want.MonthMask ||
		g.DueDay != want.DueDay || g.SortOrder != want.SortOrder ||
		!g.ActiveFrom.Equal(want.ActiveFrom) || !g.ActiveUntil.Equal(want.ActiveUntil) ||
		!g.Share.Fraction().Equal(want.Share.Fraction()) {
		t.Errorf("Concepts()[0] = %+v, want %+v", g, want)
	}
	if !got[0].ActiveUntil.IsZero() {
		t.Error("ActiveUntil should round-trip as zero (unbounded) when never set")
	}
}

func TestCreateConceptDefaultsShareToFull(t *testing.T) {
	db := openTestStore(t).DB()
	cat := mustCategory(t, db)

	c := Concept{
		Name:       "Internet",
		CategoryID: cat.ID,
		Kind:       FixedExpense,
		Currency:   domain.ARS,
		MonthMask:  domain.Monthly,
		ActiveFrom: domain.NewPeriod(2026, time.January),
	}

	created, err := CreateConcept(db, c)
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}
	if !created.Share.Fraction().Equal(domain.NewPercent(100).Fraction()) {
		t.Errorf("CreateConcept() with Share unset = %s, want %s (100%%)", created.Share.Fraction(), domain.NewPercent(100).Fraction())
	}

	got, err := Concepts(db)
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	if len(got) != 1 || !got[0].Share.Fraction().Equal(domain.NewPercent(100).Fraction()) {
		t.Errorf("Concepts()[0].Share = %s, want %s (100%%)", got[0].Share.Fraction(), domain.NewPercent(100).Fraction())
	}
}

func TestCreateConceptRequiresExistingCategory(t *testing.T) {
	db := openTestStore(t).DB()

	c := Concept{
		Name:       "Alquiler",
		CategoryID: 999,
		Kind:       FixedExpense,
		Currency:   domain.ARS,
		MonthMask:  domain.Monthly,
		Share:      domain.NewPercent(100),
		ActiveFrom: domain.NewPeriod(2026, time.January),
	}
	if _, err := CreateConcept(db, c); err == nil {
		t.Error("CreateConcept() with a dangling category ID should fail the foreign key check")
	}
}

func TestUpdateConceptRetiresViaActiveUntil(t *testing.T) {
	db := openTestStore(t).DB()
	cat := mustCategory(t, db)

	c := Concept{
		Name:       "Netflix",
		CategoryID: cat.ID,
		Kind:       VariableExpense,
		Currency:   domain.ARS,
		MonthMask:  domain.Monthly,
		Share:      domain.NewPercent(100),
		ActiveFrom: domain.NewPeriod(2026, time.January),
	}
	created, err := CreateConcept(db, c)
	if err != nil {
		t.Fatalf("CreateConcept() unexpected error: %v", err)
	}

	created.ActiveUntil = domain.NewPeriod(2026, time.June)
	if err := UpdateConcept(db, created); err != nil {
		t.Fatalf("UpdateConcept() unexpected error: %v", err)
	}

	got, err := Concepts(db)
	if err != nil {
		t.Fatalf("Concepts() unexpected error: %v", err)
	}
	if len(got) != 1 || !got[0].ActiveUntil.Equal(domain.NewPeriod(2026, time.June)) {
		t.Errorf("Concepts()[0].ActiveUntil = %v, want 2026-06", got[0].ActiveUntil)
	}
}

func TestParseConceptKind(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ConceptKind
		wantErr bool
	}{
		{"income", "Income", Income, false},
		{"fixed", "FixedExpense", FixedExpense, false},
		{"variable", "VariableExpense", VariableExpense, false},
		{"unknown", "Savings", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConceptKind(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseConceptKind(%q) = nil error, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseConceptKind(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseConceptKind(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
