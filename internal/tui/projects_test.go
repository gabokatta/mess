package tui

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"charm.land/huh/v2"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func seedProject(t *testing.T, db *sql.DB, p catalog.Project) catalog.Project {
	t.Helper()
	created, err := catalog.CreateProject(db, p)
	if err != nil {
		t.Fatalf("CreateProject() unexpected error: %v", err)
	}
	return created
}

func projectsModel(t *testing.T, db *sql.DB, period domain.Period) Model {
	t.Helper()
	m := New(db)
	m.width, m.height = 100, 40
	m.period = period
	m.view = viewProjects
	projects, err := catalog.Projects(db)
	if err != nil {
		t.Fatalf("catalog.Projects() unexpected error: %v", err)
	}
	updated, _ := m.Update(projectsLoadedMsg{projects: projects})
	return updated.(Model)
}

func TestProjectsViewRendersPendingOrderedAndWithProgress(t *testing.T) {
	db := openTestStore(t)
	current := domain.NewPeriod(2026, time.September)
	seedProject(t, db, catalog.Project{
		Name:   "Itinerary",
		BodyMD: "## ARG\n- [x] book flights\n- [ ] renew passport",
		Period: current,
	})
	seedProject(t, db, catalog.Project{
		Name:   "Buy list",
		Period: domain.NewPeriod(2026, time.July),
		BodyMD: "- [ ] paint",
	})

	m := projectsModel(t, db, current)
	content := m.View().Content

	buyIdx := strings.Index(content, "Buy list")
	itinIdx := strings.Index(content, "Itinerary")
	if buyIdx == -1 || itinIdx == -1 || buyIdx > itinIdx {
		t.Errorf("content = %q, want overdue Buy list before the current-month Itinerary", content)
	}
	if !strings.Contains(content, "1/2") {
		t.Errorf("content missing Itinerary's 1/2 progress:\n%s", content)
	}
	if !strings.Contains(content, "overdue") {
		t.Errorf("content missing the overdue badge for Buy list:\n%s", content)
	}
}

func TestProjectsCursorMovesWithJKAndClamps(t *testing.T) {
	db := openTestStore(t)
	seedProject(t, db, catalog.Project{Name: "Buy list", BodyMD: "- [ ] milk\n- [ ] eggs"})
	current := domain.NewPeriod(2026, time.September)

	m := projectsModel(t, db, current)
	if m.projectCursor != 0 {
		t.Fatalf("projectCursor = %d, want 0 at load", m.projectCursor)
	}

	updated, _ := m.Update(key("j"))
	m = updated.(Model)
	if m.projectCursor != 1 {
		t.Fatalf("projectCursor = %d, want 1 after j", m.projectCursor)
	}

	updated, _ = m.Update(key("j"))
	m = updated.(Model)
	if m.projectCursor != 1 {
		t.Fatalf("projectCursor = %d, want clamped at 1 (last checkbox)", m.projectCursor)
	}

	updated, _ = m.Update(key("k"))
	m = updated.(Model)
	if m.projectCursor != 0 {
		t.Fatalf("projectCursor = %d, want 0 after k", m.projectCursor)
	}
}

func TestSpaceTogglesCheckboxUnderProjectCursor(t *testing.T) {
	db := openTestStore(t)
	seedProject(t, db, catalog.Project{Name: "Buy list", BodyMD: "- [ ] milk\n- [ ] eggs"})
	current := domain.NewPeriod(2026, time.September)

	m := projectsModel(t, db, current)
	updated, _ := m.Update(key("j"))
	m = updated.(Model)

	updated, cmd := m.Update(keySpace())
	m = updated.(Model)
	m = settle(t, m, cmd)

	projects, err := catalog.Projects(db)
	if err != nil {
		t.Fatalf("Projects() unexpected error: %v", err)
	}
	if len(projects) != 1 || projects[0].BodyMD != "- [ ] milk\n- [x] eggs" {
		t.Fatalf("Projects() = %+v, want only eggs ticked", projects)
	}
}

func TestEKeyOpensTextareaAndCtrlSCommits(t *testing.T) {
	db := openTestStore(t)
	p := seedProject(t, db, catalog.Project{Name: "Buy list", BodyMD: "- [ ] milk"})
	current := domain.NewPeriod(2026, time.September)

	m := projectsModel(t, db, current)
	updated, _ := m.Update(key("e"))
	m = updated.(Model)
	if m.projectEditing == nil || m.projectEditing.projectID != p.ID {
		t.Fatalf("projectEditing = %+v, want editing opened for %d", m.projectEditing, p.ID)
	}
	if got := m.projectEditing.textarea.Value(); got != "- [ ] milk" {
		t.Errorf("textarea value = %q, want prefilled with the body", got)
	}

	m.projectEditing.textarea.SetValue("- [ ] milk\n- [ ] bread")
	updated, cmd := m.Update(keyCtrlS())
	m = updated.(Model)
	if m.projectEditing != nil {
		t.Fatal("ctrl+s should close the edit")
	}
	m = settle(t, m, cmd)

	projects, err := catalog.Projects(db)
	if err != nil {
		t.Fatalf("Projects() unexpected error: %v", err)
	}
	if len(projects) != 1 || projects[0].BodyMD != "- [ ] milk\n- [ ] bread" {
		t.Fatalf("Projects() = %+v, want the edited body persisted", projects)
	}
}

func TestEscCancelsProjectEditWithoutWriting(t *testing.T) {
	db := openTestStore(t)
	seedProject(t, db, catalog.Project{Name: "Buy list", BodyMD: "- [ ] milk"})
	current := domain.NewPeriod(2026, time.September)

	m := projectsModel(t, db, current)
	updated, _ := m.Update(key("e"))
	m = updated.(Model)
	m.projectEditing.textarea.SetValue("wiped out")

	updated, cmd := m.Update(keyEsc())
	m = updated.(Model)
	if m.projectEditing != nil {
		t.Error("esc should close the edit")
	}
	if cmd != nil {
		t.Error("esc should not write anything")
	}

	projects, err := catalog.Projects(db)
	if err != nil {
		t.Fatalf("Projects() unexpected error: %v", err)
	}
	if projects[0].BodyMD != "- [ ] milk" {
		t.Errorf("Projects()[0].BodyMD = %q, want unchanged", projects[0].BodyMD)
	}
}

func TestCKeyClosesAndReopensProject(t *testing.T) {
	db := openTestStore(t)
	seedProject(t, db, catalog.Project{Name: "Buy list", BodyMD: "- [ ] milk"})
	current := domain.NewPeriod(2026, time.September)

	m := projectsModel(t, db, current)
	updated, cmd := m.Update(key("c"))
	m = updated.(Model)
	m = settle(t, m, cmd)

	projects, err := catalog.Projects(db)
	if err != nil {
		t.Fatalf("Projects() unexpected error: %v", err)
	}
	if projects[0].ClosedAt == nil {
		t.Fatal("Projects()[0].ClosedAt = nil, want it closed")
	}

	m.showClosed = true
	m.projectCursor = 0
	updated, cmd = m.Update(key("c"))
	m = updated.(Model)
	m = settle(t, m, cmd)

	projects, err = catalog.Projects(db)
	if err != nil {
		t.Fatalf("Projects() unexpected error: %v", err)
	}
	if projects[0].ClosedAt != nil {
		t.Fatal("Projects()[0].ClosedAt != nil, want it reopened")
	}
}

func TestClosingProjectResetsCursor(t *testing.T) {
	db := openTestStore(t)
	seedProject(t, db, catalog.Project{Name: "A", BodyMD: "- [ ] a"})
	seedProject(t, db, catalog.Project{Name: "B", BodyMD: "- [ ] b"})
	current := domain.NewPeriod(2026, time.September)

	m := projectsModel(t, db, current)
	updated, _ := m.Update(key("j"))
	m = updated.(Model)
	if m.projectCursor != 1 {
		t.Fatalf("projectCursor = %d, want 1 before closing", m.projectCursor)
	}

	updated, _ = m.Update(key("c"))
	m = updated.(Model)
	if m.projectCursor != 0 {
		t.Errorf("projectCursor = %d, want reset to 0 after closing", m.projectCursor)
	}
}

func TestNKeyOpensNewProjectFormAndRendersIt(t *testing.T) {
	db := openTestStore(t)
	current := domain.NewPeriod(2026, time.September)
	m := projectsModel(t, db, current)

	updated, cmd := m.Update(key("n"))
	m = updated.(Model)
	if m.newProject == nil {
		t.Fatal("newProject = nil, want a name prompt opened")
	}
	m = settle(t, m, cmd)

	content := m.View().Content
	if !strings.Contains(content, "Project name") {
		t.Errorf("content = %q, want the Project name field", content)
	}
}

func TestCompletingNewProjectFormCreatesUnassignedBodylessProject(t *testing.T) {
	db := openTestStore(t)
	current := domain.NewPeriod(2026, time.September)
	m := projectsModel(t, db, current)

	updated, _ := m.Update(key("n"))
	m = updated.(Model)
	m.newProject.values.name = "Buy list"
	m.newProject.form.State = huh.StateCompleted

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	if m.newProject != nil {
		t.Fatal("a completed form should close")
	}
	m = settle(t, m, cmd)

	projects, err := catalog.Projects(db)
	if err != nil {
		t.Fatalf("catalog.Projects() unexpected error: %v", err)
	}
	if len(projects) != 1 || projects[0].Name != "Buy list" || projects[0].BodyMD != "" || !projects[0].Period.IsZero() {
		t.Errorf("Projects() = %+v, want a single unassigned, bodyless Buy list", projects)
	}
}

func TestEnterWithBlankNameKeepsFormOpen(t *testing.T) {
	db := openTestStore(t)
	current := domain.NewPeriod(2026, time.September)
	m := projectsModel(t, db, current)

	updated, _ := m.Update(key("n"))
	m = updated.(Model)
	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	if m.newProject == nil {
		t.Fatal("enter with a blank required name should keep the form open")
	}
	m = settle(t, m, cmd)

	projects, err := catalog.Projects(db)
	if err != nil {
		t.Fatalf("catalog.Projects() unexpected error: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("Projects() = %+v, want none created", projects)
	}
}

func TestEscCancelsNewProjectFormWithoutWriting(t *testing.T) {
	db := openTestStore(t)
	current := domain.NewPeriod(2026, time.September)
	m := projectsModel(t, db, current)

	updated, _ := m.Update(key("n"))
	m = updated.(Model)
	m.newProject.values.name = "Buy list"
	updated, cmd := m.Update(keyEsc())
	m = updated.(Model)
	if m.newProject != nil {
		t.Error("esc should close the form")
	}
	if cmd != nil {
		t.Error("esc should not write anything")
	}

	projects, err := catalog.Projects(db)
	if err != nil {
		t.Fatalf("catalog.Projects() unexpected error: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("Projects() = %+v, want none created", projects)
	}
}

func TestFKeyTogglesClosedFilterAndResetsCursor(t *testing.T) {
	db := openTestStore(t)
	p := seedProject(t, db, catalog.Project{Name: "Buy list", BodyMD: "- [ ] milk"})
	current := domain.NewPeriod(2026, time.September)
	closedAt := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	if err := catalog.SetProjectClosed(db, p.ID, &closedAt); err != nil {
		t.Fatalf("SetProjectClosed() unexpected error: %v", err)
	}

	m := projectsModel(t, db, current)
	content := m.View().Content
	if !strings.Contains(content, "no open projects") {
		t.Fatalf("content = %q, want the empty pending state", content)
	}

	updated, _ := m.Update(key("f"))
	m = updated.(Model)
	if !m.showClosed {
		t.Fatal("showClosed = false, want true after f")
	}
	content = m.View().Content
	if !strings.Contains(content, "Buy list") {
		t.Errorf("content = %q, want the closed Buy list visible", content)
	}
}
