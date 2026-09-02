package tui

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/list"
)

// listsLoadedMsg is the result of loadLists' Cmd, delivered back to
// Update once the database read completes.
type listsLoadedMsg struct {
	lists []catalog.List
	err   error
}

// loadLists returns a Cmd that reads every list off the Update loop.
func loadLists(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		l, err := catalog.Lists(db)
		return listsLoadedMsg{lists: l, err: err}
	}
}

// listSavedMsg is the result of a list body or closed-state write,
// which always triggers a reload so the rendered list reflects it.
type listSavedMsg struct {
	err error
}

func saveListBody(db *sql.DB, id int64, body string) tea.Cmd {
	return func() tea.Msg {
		return listSavedMsg{err: catalog.SetListBody(db, id, body)}
	}
}

func setListClosed(db *sql.DB, id int64, closedAt *time.Time) tea.Cmd {
	return func() tea.Msg {
		return listSavedMsg{err: catalog.SetListClosed(db, id, closedAt)}
	}
}

func createList(db *sql.DB, name string, period domain.Period) tea.Cmd {
	return func() tea.Msg {
		_, err := catalog.CreateList(db, catalog.List{Name: name, Period: period})
		return listSavedMsg{err: err}
	}
}

// newListFormValues is the huh-bound value for the new-list prompt.
type newListFormValues struct {
	name   string
	period string
}

type newListFormState struct {
	form   *huh.Form
	values *newListFormValues
}

func newListForm(theme Theme, width, height int) *newListFormState {
	v := &newListFormValues{}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("List name").Value(&v.name).Validate(huh.ValidateNotEmpty()),
			huh.NewInput().Title("Period").Description("blank = unassigned (YYYY-MM)").
				Value(&v.period).Validate(validateOptionalPeriod),
		),
	).WithTheme(themeFor(theme)).WithWidth(width - 6).WithHeight(formHeight(height))
	return &newListFormState{form: form, values: v}
}

// startNewList opens a name+period prompt; the list starts bodyless,
// filled in afterward with 'e'.
func (m Model) startNewList() (Model, tea.Cmd) {
	m.newList = newListForm(m.theme, m.width, m.height)
	return m, m.newList.form.Init()
}

func (m Model) updateNewList(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.newList = nil
		return m, nil
	}
	return m.forwardNewList(msg)
}

// forwardNewList drives the form with any tea.Msg, not just key presses
// — see forwardConceptForm's comment for why.
func (m Model) forwardNewList(msg tea.Msg) (Model, tea.Cmd) {
	updated, cmd := m.newList.form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.newList.form = f
	}

	switch m.newList.form.State {
	case huh.StateCompleted:
		name := m.newList.values.name
		var period domain.Period
		if m.newList.values.period != "" {
			period, _ = domain.ParsePeriod(m.newList.values.period)
		}
		m.newList = nil
		return m, tea.Batch(cmd, createList(m.db, name, period))
	case huh.StateAborted:
		m.newList = nil
		return m, nil
	}
	return m, cmd
}

// listEditState is the full-body textarea edit in progress, keyed by
// list so a reload mid-edit can't desync it from the list it targets.
type listEditState struct {
	listID   int64
	textarea textarea.Model
}

// listRow is one cursor-addressable unit in the Pending/Closed list:
// either a checkbox inside a list's body, or — for a list with none —
// the list itself, so a pure-prose list stays reachable.
type listRow struct {
	list     catalog.List
	checkbox int // index into list.Checkboxes(body.BodyMD); -1 for the list itself
}

func (m Model) listList() []catalog.List {
	if m.showClosed {
		return list.Closed(m.lists)
	}
	return list.Pending(m.lists, m.period)
}

func (m Model) listRows() []listRow {
	var rows []listRow
	for _, l := range m.listList() {
		boxes := list.Checkboxes(l.BodyMD)
		if len(boxes) == 0 {
			rows = append(rows, listRow{list: l, checkbox: -1})
			continue
		}
		for i := range boxes {
			rows = append(rows, listRow{list: l, checkbox: i})
		}
	}
	return rows
}

func (m Model) moveListCursor(delta int) int {
	n := len(m.listRows())
	if n == 0 {
		return 0
	}
	cursor := m.listCursor + delta
	if cursor < 0 {
		return 0
	}
	if cursor >= n {
		return n - 1
	}
	return cursor
}

// cursorListRow reports the row under the cursor, if the list isn't empty.
func (m Model) cursorListRow() (listRow, bool) {
	rows := m.listRows()
	if m.listCursor >= len(rows) {
		return listRow{}, false
	}
	return rows[m.listCursor], true
}

// toggleListCheckbox flips the checkbox under the cursor and persists
// it. A cursor on a checkbox-less list's row is a no-op — there's
// nothing to tick.
func (m Model) toggleListCheckbox() (Model, tea.Cmd) {
	row, ok := m.cursorListRow()
	if !ok || row.checkbox < 0 {
		return m, nil
	}
	body := list.Toggle(row.list.BodyMD, row.checkbox)
	return m, saveListBody(m.db, row.list.ID, body)
}

func (m Model) startListEdit() (Model, tea.Cmd) {
	row, ok := m.cursorListRow()
	if !ok {
		return m, nil
	}
	ta := textarea.New()
	ta.SetValue(row.list.BodyMD)
	ta.SetWidth(m.width - 6)
	ta.SetHeight(m.height - 10)
	ta.Focus()
	m.listEditing = &listEditState{listID: row.list.ID, textarea: ta}
	return m, nil
}

func (m Model) updateListEditing(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.listEditing = nil
		return m, nil
	case "ctrl+s":
		return m.commitListEdit()
	}
	var cmd tea.Cmd
	m.listEditing.textarea, cmd = m.listEditing.textarea.Update(msg)
	return m, cmd
}

func (m Model) commitListEdit() (Model, tea.Cmd) {
	id := m.listEditing.listID
	body := m.listEditing.textarea.Value()
	m.listEditing = nil
	return m, saveListBody(m.db, id, body)
}

// toggleListClosed closes the cursor's open list, or reopens it if
// it's already closed. Closing is unconditional — it never checks progress.
// Either way the list leaves the current list on reload, so the cursor
// resets rather than pointing at whatever row happens to land there next.
func (m Model) toggleListClosed() (Model, tea.Cmd) {
	row, ok := m.cursorListRow()
	if !ok {
		return m, nil
	}
	m.listCursor = 0
	if row.list.ClosedAt != nil {
		return m, setListClosed(m.db, row.list.ID, nil)
	}
	now := time.Now()
	return m, setListClosed(m.db, row.list.ID, &now)
}

func setListPeriod(db *sql.DB, id int64, period domain.Period) tea.Cmd {
	return func() tea.Msg {
		return listSavedMsg{err: catalog.SetListPeriod(db, id, period)}
	}
}

// periodAssignFormValues is the huh-bound value for (re)assigning the
// cursor's list to a period.
type periodAssignFormValues struct {
	period string
}

type periodAssignFormState struct {
	form   *huh.Form
	values *periodAssignFormValues
	listID int64
}

func newPeriodAssignForm(theme Theme, width, height int, listID int64, current domain.Period) *periodAssignFormState {
	v := &periodAssignFormValues{period: periodOrBlank(current)}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Period").Description("blank = unassigned (YYYY-MM)").
				Value(&v.period).Validate(validateOptionalPeriod),
		).Title("Assign period"),
	).WithTheme(themeFor(theme)).WithWidth(width - 6).WithHeight(formHeight(height))
	return &periodAssignFormState{form: form, values: v, listID: listID}
}

func (m Model) startPeriodAssign() (Model, tea.Cmd) {
	row, ok := m.cursorListRow()
	if !ok {
		return m, nil
	}
	m.periodAssignForm = newPeriodAssignForm(m.theme, m.width, m.height, row.list.ID, row.list.Period)
	return m, m.periodAssignForm.form.Init()
}

func (m Model) updatePeriodAssignForm(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.periodAssignForm = nil
		return m, nil
	}
	return m.forwardPeriodAssignForm(msg)
}

// forwardPeriodAssignForm drives the form with any tea.Msg — see
// forwardConceptForm's comment for why.
func (m Model) forwardPeriodAssignForm(msg tea.Msg) (Model, tea.Cmd) {
	updated, cmd := m.periodAssignForm.form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.periodAssignForm.form = f
	}

	switch m.periodAssignForm.form.State {
	case huh.StateCompleted:
		id := m.periodAssignForm.listID
		var period domain.Period
		if m.periodAssignForm.values.period != "" {
			period, _ = domain.ParsePeriod(m.periodAssignForm.values.period)
		}
		m.periodAssignForm = nil
		return m, tea.Batch(cmd, setListPeriod(m.db, id, period))
	case huh.StateAborted:
		m.periodAssignForm = nil
		return m, nil
	}
	return m, cmd
}

func (m Model) renderLists() string {
	if m.listEditing != nil {
		return m.renderListEditor()
	}

	var b strings.Builder
	label := "Pending"
	if m.showClosed {
		label = "Closed"
	}
	b.WriteString(m.theme.Muted.Render(m.view.String() + " · " + label))

	if m.newList != nil {
		b.WriteString("\n\n")
		b.WriteString(m.newList.form.View())
		return b.String()
	}
	if m.periodAssignForm != nil {
		b.WriteString("\n\n")
		b.WriteString(m.periodAssignForm.form.View())
		return b.String()
	}

	if m.listsErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("failed to load: " + m.listsErr.Error()))
		return b.String()
	}

	shown := m.listList()
	if len(shown) == 0 {
		empty := "no open lists"
		if m.showClosed {
			empty = "no closed lists"
		}
		b.WriteString("\n")
		b.WriteString(m.centerInBox(m.theme.Muted.Render(empty)))
		return b.String()
	}

	row, _ := m.cursorListRow()
	for _, l := range shown {
		b.WriteString("\n\n")
		b.WriteString(m.renderListHeader(l, row.list.ID == l.ID && row.checkbox < 0))
		target := -1
		if row.list.ID == l.ID {
			target = row.checkbox
		}
		if body := m.renderListBody(l, target); body != "" {
			b.WriteString("\n")
			b.WriteString(body)
		}
	}

	if m.listSaveErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("failed to save: " + m.listSaveErr.Error()))
	}
	return b.String()
}

func (m Model) renderListHeader(l catalog.List, selected bool) string {
	cursor := "  "
	if selected {
		cursor = "> "
	}
	done, total := list.Progress(l.BodyMD)
	return fmt.Sprintf("%s%s  %d/%d  %s", cursor, m.theme.Title.Render(l.Name), done, total, m.listBadge(l))
}

func (m Model) listBadge(l catalog.List) string {
	switch {
	case l.ClosedAt != nil:
		return m.theme.Muted.Render("closed " + l.ClosedAt.Format("2006-01-02"))
	case l.Period.IsZero():
		return m.theme.Muted.Render("unassigned")
	case l.Period.Before(m.period):
		return m.theme.Muted.Render(l.Period.String() + " overdue")
	default:
		return m.theme.Muted.Render(l.Period.String())
	}
}

// renderListBody renders l's body via Glamour and marks the target-th
// checkbox (in document order) with a cursor gutter, or nothing if target
// is negative.
func (m Model) renderListBody(l catalog.List, target int) string {
	rendered, err := renderMarkdown(l.BodyMD, m.width-6, m.theme.Dark)
	if err != nil {
		return m.theme.Muted.Render("markdown error: " + err.Error())
	}
	return markCheckboxCursor(rendered, target)
}

func (m Model) renderListEditor() string {
	var b strings.Builder
	b.WriteString(m.theme.Muted.Render(m.view.String() + " · editing"))
	b.WriteString("\n\n")
	b.WriteString(m.listEditing.textarea.View())
	return b.String()
}
