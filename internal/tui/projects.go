package tui

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/project"
)

// projectsLoadedMsg is the result of loadProjects' Cmd, delivered back to
// Update once the database read completes.
type projectsLoadedMsg struct {
	projects []catalog.Project
	err      error
}

// loadProjects returns a Cmd that reads every project off the Update loop.
func loadProjects(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		p, err := catalog.Projects(db)
		return projectsLoadedMsg{projects: p, err: err}
	}
}

// projectSavedMsg is the result of a project body or closed-state write,
// which always triggers a reload so the rendered list reflects it.
type projectSavedMsg struct {
	err error
}

func saveProjectBody(db *sql.DB, id int64, body string) tea.Cmd {
	return func() tea.Msg {
		return projectSavedMsg{err: catalog.SetProjectBody(db, id, body)}
	}
}

func setProjectClosed(db *sql.DB, id int64, closedAt *time.Time) tea.Cmd {
	return func() tea.Msg {
		return projectSavedMsg{err: catalog.SetProjectClosed(db, id, closedAt)}
	}
}

// projectEditState is the full-body textarea edit in progress, keyed by
// project so a reload mid-edit can't desync it from the project it targets.
type projectEditState struct {
	projectID int64
	textarea  textarea.Model
}

// projectRow is one cursor-addressable unit in the Pending/Closed list:
// either a checkbox inside a project's body, or — for a project with none —
// the project itself, so a pure-prose project stays reachable.
type projectRow struct {
	project  catalog.Project
	checkbox int // index into project.Checkboxes(body.BodyMD); -1 for the project itself
}

func (m Model) projectList() []catalog.Project {
	if m.showClosed {
		return project.Closed(m.projects)
	}
	return project.Pending(m.projects, m.period)
}

func (m Model) projectRows() []projectRow {
	var rows []projectRow
	for _, p := range m.projectList() {
		boxes := project.Checkboxes(p.BodyMD)
		if len(boxes) == 0 {
			rows = append(rows, projectRow{project: p, checkbox: -1})
			continue
		}
		for i := range boxes {
			rows = append(rows, projectRow{project: p, checkbox: i})
		}
	}
	return rows
}

func (m Model) moveProjectCursor(delta int) int {
	n := len(m.projectRows())
	if n == 0 {
		return 0
	}
	cursor := m.projectCursor + delta
	if cursor < 0 {
		return 0
	}
	if cursor >= n {
		return n - 1
	}
	return cursor
}

// cursorProjectRow reports the row under the cursor, if the list isn't empty.
func (m Model) cursorProjectRow() (projectRow, bool) {
	rows := m.projectRows()
	if m.projectCursor >= len(rows) {
		return projectRow{}, false
	}
	return rows[m.projectCursor], true
}

// toggleProjectCheckbox flips the checkbox under the cursor and persists
// it. A cursor on a checkbox-less project's row is a no-op — there's
// nothing to tick.
func (m Model) toggleProjectCheckbox() (Model, tea.Cmd) {
	row, ok := m.cursorProjectRow()
	if !ok || row.checkbox < 0 {
		return m, nil
	}
	body := project.Toggle(row.project.BodyMD, row.checkbox)
	return m, saveProjectBody(m.db, row.project.ID, body)
}

func (m Model) startProjectEdit() (Model, tea.Cmd) {
	row, ok := m.cursorProjectRow()
	if !ok {
		return m, nil
	}
	ta := textarea.New()
	ta.SetValue(row.project.BodyMD)
	ta.SetWidth(m.width - 6)
	ta.SetHeight(m.height - 10)
	ta.Focus()
	m.projectEditing = &projectEditState{projectID: row.project.ID, textarea: ta}
	return m, nil
}

func (m Model) updateProjectEditing(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.projectEditing = nil
		return m, nil
	case "ctrl+s":
		return m.commitProjectEdit()
	}
	var cmd tea.Cmd
	m.projectEditing.textarea, cmd = m.projectEditing.textarea.Update(msg)
	return m, cmd
}

func (m Model) commitProjectEdit() (Model, tea.Cmd) {
	id := m.projectEditing.projectID
	body := m.projectEditing.textarea.Value()
	m.projectEditing = nil
	return m, saveProjectBody(m.db, id, body)
}

// toggleProjectClosed closes the cursor's open project, or reopens it if
// it's already closed. Closing is unconditional — it never checks progress.
// Either way the project leaves the current list on reload, so the cursor
// resets rather than pointing at whatever row happens to land there next.
func (m Model) toggleProjectClosed() (Model, tea.Cmd) {
	row, ok := m.cursorProjectRow()
	if !ok {
		return m, nil
	}
	m.projectCursor = 0
	if row.project.ClosedAt != nil {
		return m, setProjectClosed(m.db, row.project.ID, nil)
	}
	now := time.Now()
	return m, setProjectClosed(m.db, row.project.ID, &now)
}

func (m Model) renderProjects() string {
	if m.projectEditing != nil {
		return m.renderProjectEditor()
	}

	var b strings.Builder
	label := "Pending"
	if m.showClosed {
		label = "Closed"
	}
	b.WriteString(m.theme.Muted.Render(m.view.String() + " · " + label))

	if m.projectsErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("failed to load: " + m.projectsErr.Error()))
		return b.String()
	}

	list := m.projectList()
	if len(list) == 0 {
		empty := "no open projects"
		if m.showClosed {
			empty = "no closed projects"
		}
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render(empty))
		return b.String()
	}

	row, _ := m.cursorProjectRow()
	for _, p := range list {
		b.WriteString("\n\n")
		b.WriteString(m.renderProjectHeader(p, row.project.ID == p.ID && row.checkbox < 0))
		target := -1
		if row.project.ID == p.ID {
			target = row.checkbox
		}
		if body := m.renderProjectBody(p, target); body != "" {
			b.WriteString("\n")
			b.WriteString(body)
		}
	}

	if m.projectSaveErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("failed to save: " + m.projectSaveErr.Error()))
	}
	return b.String()
}

func (m Model) renderProjectHeader(p catalog.Project, selected bool) string {
	cursor := "  "
	if selected {
		cursor = "> "
	}
	done, total := project.Progress(p.BodyMD)
	return fmt.Sprintf("%s%s  %d/%d  %s", cursor, m.theme.Title.Render(p.Name), done, total, m.projectBadge(p))
}

func (m Model) projectBadge(p catalog.Project) string {
	switch {
	case p.ClosedAt != nil:
		return m.theme.Muted.Render("closed " + p.ClosedAt.Format("2006-01-02"))
	case p.Period.IsZero():
		return m.theme.Muted.Render("unassigned")
	case p.Period.Before(m.period):
		return m.theme.Muted.Render(p.Period.String() + " overdue")
	default:
		return m.theme.Muted.Render(p.Period.String())
	}
}

// renderProjectBody renders p's body via Glamour and marks the target-th
// checkbox (in document order) with a cursor gutter, or nothing if target
// is negative.
func (m Model) renderProjectBody(p catalog.Project, target int) string {
	rendered, err := renderMarkdown(p.BodyMD, m.width-6, m.theme.Dark)
	if err != nil {
		return m.theme.Muted.Render("markdown error: " + err.Error())
	}
	return markCheckboxCursor(rendered, target)
}

func (m Model) renderProjectEditor() string {
	var b strings.Builder
	b.WriteString(m.theme.Muted.Render(m.view.String() + " · editing"))
	b.WriteString("\n\n")
	b.WriteString(m.projectEditing.textarea.View())
	return b.String()
}
