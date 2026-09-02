package catalog

import (
	"testing"
	"time"

	"github.com/gabokatta/mess/internal/domain"
)

func TestCreateAndListLists(t *testing.T) {
	db := openTestStore(t).DB()

	created, err := CreateList(db, List{
		Name:   "Venezuela trip",
		BodyMD: "## ARG\n- [ ] book flights",
		Period: domain.NewPeriod(2026, time.September),
	})
	if err != nil {
		t.Fatalf("CreateList() unexpected error: %v", err)
	}
	if created.ID == 0 {
		t.Error("CreateList() should assign a non-zero ID")
	}

	got, err := Lists(db)
	if err != nil {
		t.Fatalf("Lists() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Venezuela trip" || got[0].BodyMD != "## ARG\n- [ ] book flights" {
		t.Fatalf("Lists() = %+v, want a single Venezuela trip row", got)
	}
	if got[0].Period != domain.NewPeriod(2026, time.September) {
		t.Errorf("Lists()[0].Period = %v, want 2026-09", got[0].Period)
	}
	if got[0].ClosedAt != nil {
		t.Errorf("Lists()[0].ClosedAt = %v, want nil (open)", got[0].ClosedAt)
	}
}

func TestCreateListWithNoPeriodIsUnassigned(t *testing.T) {
	db := openTestStore(t).DB()

	if _, err := CreateList(db, List{Name: "Groceries template"}); err != nil {
		t.Fatalf("CreateList() unexpected error: %v", err)
	}

	got, err := Lists(db)
	if err != nil {
		t.Fatalf("Lists() unexpected error: %v", err)
	}
	if len(got) != 1 || !got[0].Period.IsZero() {
		t.Fatalf("Lists() = %+v, want an unassigned (zero) period", got)
	}
}

func TestSetListBody(t *testing.T) {
	db := openTestStore(t).DB()
	p, err := CreateList(db, List{Name: "Buy list", BodyMD: "- [ ] milk"})
	if err != nil {
		t.Fatalf("CreateList() unexpected error: %v", err)
	}

	if err := SetListBody(db, p.ID, "- [x] milk"); err != nil {
		t.Fatalf("SetListBody() unexpected error: %v", err)
	}

	got, err := Lists(db)
	if err != nil {
		t.Fatalf("Lists() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].BodyMD != "- [x] milk" {
		t.Fatalf("Lists() = %+v, want the rewritten body", got)
	}
}

func TestSetListClosedSetsAndClears(t *testing.T) {
	db := openTestStore(t).DB()
	p, err := CreateList(db, List{Name: "Buy list"})
	if err != nil {
		t.Fatalf("CreateList() unexpected error: %v", err)
	}

	closedAt := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	if err := SetListClosed(db, p.ID, &closedAt); err != nil {
		t.Fatalf("SetListClosed() unexpected error: %v", err)
	}

	got, err := Lists(db)
	if err != nil {
		t.Fatalf("Lists() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ClosedAt == nil || !got[0].ClosedAt.Equal(closedAt) {
		t.Fatalf("Lists() = %+v, want closed_at set to %v", got, closedAt)
	}

	if err := SetListClosed(db, p.ID, nil); err != nil {
		t.Fatalf("SetListClosed() unexpected error: %v", err)
	}
	got, err = Lists(db)
	if err != nil {
		t.Fatalf("Lists() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ClosedAt != nil {
		t.Fatalf("Lists() = %+v, want closed_at cleared", got)
	}
}

func TestSetListPeriodAssignsAndClears(t *testing.T) {
	db := openTestStore(t).DB()
	p, err := CreateList(db, List{Name: "Someday list"})
	if err != nil {
		t.Fatalf("CreateList() unexpected error: %v", err)
	}

	sept := domain.NewPeriod(2026, time.September)
	if err := SetListPeriod(db, p.ID, sept); err != nil {
		t.Fatalf("SetListPeriod() unexpected error: %v", err)
	}
	got, err := Lists(db)
	if err != nil {
		t.Fatalf("Lists() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Period != sept {
		t.Fatalf("Lists() = %+v, want the period assigned to 2026-09", got)
	}

	if err := SetListPeriod(db, p.ID, domain.Period{}); err != nil {
		t.Fatalf("SetListPeriod() unexpected error: %v", err)
	}
	got, err = Lists(db)
	if err != nil {
		t.Fatalf("Lists() unexpected error: %v", err)
	}
	if len(got) != 1 || !got[0].Period.IsZero() {
		t.Fatalf("Lists() = %+v, want the period cleared back to unassigned", got)
	}
}
