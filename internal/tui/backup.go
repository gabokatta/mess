package tui

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/gabokatta/mess/internal/backup"
)

// backupDoneMsg is the result of an export or import action, which always
// triggers a settings reload since an import can change that row too.
type backupDoneMsg struct {
	message string
	err     error
}

func exportBackup(db *sql.DB, path string) tea.Cmd {
	return func() tea.Msg {
		data, err := backup.Export(db)
		if err != nil {
			return backupDoneMsg{err: err}
		}
		raw, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return backupDoneMsg{err: err}
		}
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			return backupDoneMsg{err: err}
		}
		return backupDoneMsg{message: "exported to " + path}
	}
}

// importBackup snapshots the current database beside dbPath before
// replacing it wholesale — the safety net a wholesale replace needs.
func importBackup(db *sql.DB, dbPath, path string) tea.Cmd {
	return func() tea.Msg {
		raw, err := os.ReadFile(path)
		if err != nil {
			return backupDoneMsg{err: err}
		}
		var data backup.Data
		if err := json.Unmarshal(raw, &data); err != nil {
			return backupDoneMsg{err: err}
		}
		if _, err := backup.Snapshot(db, dbPath); err != nil {
			return backupDoneMsg{err: err}
		}
		if err := backup.Import(db, data); err != nil {
			return backupDoneMsg{err: err}
		}
		return backupDoneMsg{message: "imported from " + path}
	}
}

func defaultBackupPath() string {
	return fmt.Sprintf("mess-%s.json", time.Now().Format("2006-01-02"))
}

type exportFormValues struct {
	path string
}

type exportFormState struct {
	form   *huh.Form
	values *exportFormValues
}

func newExportForm(theme Theme, width, height int) *exportFormState {
	v := &exportFormValues{path: defaultBackupPath()}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Export to file").Value(&v.path).Validate(huh.ValidateNotEmpty()),
		).Title("Export"),
	).WithTheme(themeFor(theme)).WithWidth(width - 6).WithHeight(formHeight(height))
	return &exportFormState{form: form, values: v}
}

func (m Model) startExport() (Model, tea.Cmd) {
	m.exportForm = newExportForm(m.theme, m.width, m.height)
	return m, m.exportForm.form.Init()
}

func (m Model) updateExportForm(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.exportForm = nil
		return m, nil
	}
	return m.forwardExportForm(msg)
}

// forwardExportForm drives the form with any tea.Msg — see
// forwardConceptForm's comment for why.
func (m Model) forwardExportForm(msg tea.Msg) (Model, tea.Cmd) {
	updated, cmd := m.exportForm.form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.exportForm.form = f
	}

	switch m.exportForm.form.State {
	case huh.StateCompleted:
		path := m.exportForm.values.path
		m.exportForm = nil
		return m, tea.Batch(cmd, exportBackup(m.db, path))
	case huh.StateAborted:
		m.exportForm = nil
		return m, nil
	}
	return m, cmd
}

// importFormValues gates the replace behind an explicit confirm — Continue
// left at "No" completes the form without importing anything.
type importFormValues struct {
	path      string
	confirmed bool
}

type importFormState struct {
	form   *huh.Form
	values *importFormValues
}

func newImportForm(theme Theme, width, height int) *importFormState {
	v := &importFormValues{}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Import from file").Value(&v.path).Validate(huh.ValidateNotEmpty()),
			huh.NewConfirm().Title("This replaces every table with the backup. Continue?").
				Affirmative("Yes").Negative("No").Value(&v.confirmed),
		).Title("Import"),
	).WithTheme(themeFor(theme)).WithWidth(width - 6).WithHeight(formHeight(height))
	return &importFormState{form: form, values: v}
}

func (m Model) startImport() (Model, tea.Cmd) {
	m.importForm = newImportForm(m.theme, m.width, m.height)
	return m, m.importForm.form.Init()
}

func (m Model) updateImportForm(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.importForm = nil
		return m, nil
	}
	return m.forwardImportForm(msg)
}

// forwardImportForm drives the form with any tea.Msg — see
// forwardConceptForm's comment for why.
func (m Model) forwardImportForm(msg tea.Msg) (Model, tea.Cmd) {
	updated, cmd := m.importForm.form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.importForm.form = f
	}

	switch m.importForm.form.State {
	case huh.StateCompleted:
		v := m.importForm.values
		m.importForm = nil
		if !v.confirmed {
			return m, cmd
		}
		return m, tea.Batch(cmd, importBackup(m.db, m.dbPath, v.path))
	case huh.StateAborted:
		m.importForm = nil
		return m, nil
	}
	return m, cmd
}

func (m Model) renderBackupStatus() string {
	var b strings.Builder
	if m.backupErr != nil {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render("failed: " + m.backupErr.Error()))
	} else if m.backupMsg != "" {
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render(m.backupMsg))
	}
	return b.String()
}
