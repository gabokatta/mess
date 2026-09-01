package catalog

import (
	"testing"
	"time"

	"github.com/gabokatta/mess/internal/domain"
)

func TestCreateAndListProjects(t *testing.T) {
	db := openTestStore(t).DB()

	created, err := CreateProject(db, Project{
		Name:   "Venezuela trip",
		BodyMD: "## ARG\n- [ ] book flights",
		Period: domain.NewPeriod(2026, time.September),
	})
	if err != nil {
		t.Fatalf("CreateProject() unexpected error: %v", err)
	}
	if created.ID == 0 {
		t.Error("CreateProject() should assign a non-zero ID")
	}

	got, err := Projects(db)
	if err != nil {
		t.Fatalf("Projects() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Venezuela trip" || got[0].BodyMD != "## ARG\n- [ ] book flights" {
		t.Fatalf("Projects() = %+v, want a single Venezuela trip row", got)
	}
	if got[0].Period != domain.NewPeriod(2026, time.September) {
		t.Errorf("Projects()[0].Period = %v, want 2026-09", got[0].Period)
	}
	if got[0].ClosedAt != nil {
		t.Errorf("Projects()[0].ClosedAt = %v, want nil (open)", got[0].ClosedAt)
	}
}

func TestCreateProjectWithNoPeriodIsUnassigned(t *testing.T) {
	db := openTestStore(t).DB()

	if _, err := CreateProject(db, Project{Name: "Groceries template"}); err != nil {
		t.Fatalf("CreateProject() unexpected error: %v", err)
	}

	got, err := Projects(db)
	if err != nil {
		t.Fatalf("Projects() unexpected error: %v", err)
	}
	if len(got) != 1 || !got[0].Period.IsZero() {
		t.Fatalf("Projects() = %+v, want an unassigned (zero) period", got)
	}
}

func TestSetProjectBody(t *testing.T) {
	db := openTestStore(t).DB()
	p, err := CreateProject(db, Project{Name: "Buy list", BodyMD: "- [ ] milk"})
	if err != nil {
		t.Fatalf("CreateProject() unexpected error: %v", err)
	}

	if err := SetProjectBody(db, p.ID, "- [x] milk"); err != nil {
		t.Fatalf("SetProjectBody() unexpected error: %v", err)
	}

	got, err := Projects(db)
	if err != nil {
		t.Fatalf("Projects() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].BodyMD != "- [x] milk" {
		t.Fatalf("Projects() = %+v, want the rewritten body", got)
	}
}

func TestSetProjectClosedSetsAndClears(t *testing.T) {
	db := openTestStore(t).DB()
	p, err := CreateProject(db, Project{Name: "Buy list"})
	if err != nil {
		t.Fatalf("CreateProject() unexpected error: %v", err)
	}

	closedAt := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	if err := SetProjectClosed(db, p.ID, &closedAt); err != nil {
		t.Fatalf("SetProjectClosed() unexpected error: %v", err)
	}

	got, err := Projects(db)
	if err != nil {
		t.Fatalf("Projects() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ClosedAt == nil || !got[0].ClosedAt.Equal(closedAt) {
		t.Fatalf("Projects() = %+v, want closed_at set to %v", got, closedAt)
	}

	if err := SetProjectClosed(db, p.ID, nil); err != nil {
		t.Fatalf("SetProjectClosed() unexpected error: %v", err)
	}
	got, err = Projects(db)
	if err != nil {
		t.Fatalf("Projects() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ClosedAt != nil {
		t.Fatalf("Projects() = %+v, want closed_at cleared", got)
	}
}
