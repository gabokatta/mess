package catalog

import (
	"testing"
	"time"

	"github.com/gabokatta/mes/internal/domain"
)

func TestCreateAndListChores(t *testing.T) {
	db := openTestStore(t).DB()

	c := Chore{
		Name:        "Sacar la basura",
		MonthMask:   domain.Monthly,
		DueDay:      1,
		SortOrder:   1,
		ActiveFrom:  domain.NewPeriod(2026, time.January),
		ActiveUntil: domain.Period{},
	}

	created, err := CreateChore(db, c)
	if err != nil {
		t.Fatalf("CreateChore() unexpected error: %v", err)
	}
	if created.ID == 0 {
		t.Error("CreateChore() should assign a non-zero ID")
	}

	got, err := Chores(db)
	if err != nil {
		t.Fatalf("Chores() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Sacar la basura" || got[0].DueDay != 1 {
		t.Errorf("Chores() = %+v, want a single Sacar la basura row", got)
	}
}

func TestUpdateChore(t *testing.T) {
	db := openTestStore(t).DB()

	c, err := CreateChore(db, Chore{
		Name:       "Regar plantas",
		MonthMask:  domain.Monthly,
		ActiveFrom: domain.NewPeriod(2026, time.January),
	})
	if err != nil {
		t.Fatalf("CreateChore() unexpected error: %v", err)
	}

	c.Name = "Regar las plantas"
	c.ActiveUntil = domain.NewPeriod(2026, time.December)
	if err := UpdateChore(db, c); err != nil {
		t.Fatalf("UpdateChore() unexpected error: %v", err)
	}

	got, err := Chores(db)
	if err != nil {
		t.Fatalf("Chores() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Regar las plantas" || got[0].ActiveUntil != domain.NewPeriod(2026, time.December) {
		t.Errorf("Chores() = %+v, want the updated name and active_until", got)
	}
}
