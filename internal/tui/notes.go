package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/note"
)

const (
	readableColumn = 76
	titleWidth     = 52
)

func (m Model) detailWidth() int { return min(m.contentWidth(), readableColumn) }

func (m Model) shownNotes() []catalog.Note {
	return append(m.pinnedNotes(), m.periodNotes()...)
}

func (m Model) pinnedNotes() []catalog.Note {
	return filterNotes(m.notes, func(n catalog.Note) bool { return n.Period.IsZero() })
}

func (m Model) periodNotes() []catalog.Note {
	return filterNotes(m.notes, func(n catalog.Note) bool { return n.Period.Equal(m.period) })
}

func filterNotes(notes []catalog.Note, keep func(catalog.Note) bool) []catalog.Note {
	var out []catalog.Note
	for _, n := range notes {
		if keep(n) {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out
}

func (m Model) cursorNote() (catalog.Note, bool) {
	shown := m.shownNotes()
	if m.notesList.cursor >= len(shown) {
		return catalog.Note{}, false
	}
	return shown[m.notesList.cursor], true
}

func reopen(open *catalog.Note, notes []catalog.Note) *catalog.Note {
	if open == nil {
		return nil
	}
	for _, n := range notes {
		if n.ID == open.ID {
			return &n
		}
	}
	return nil
}

func (m Model) handleNotesKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.String() == "n" {
		return m.openModal(m.newNoteForm())
	}

	n, ok := m.cursorNote()
	if !ok {
		return m, nil
	}
	switch msg.String() {
	case "enter":
		m.openNote = &n
		m.detail.cursor = 0
	case "c":
		return m, write(func() error { return catalog.SetNoteDone(m.db, n.ID, !n.Done) })
	case "p":
		period := m.period
		if n.Period.IsZero() {
			return m, write(func() error { return catalog.SetNotePeriod(m.db, n.ID, period) })
		}
		return m, write(func() error { return catalog.SetNotePeriod(m.db, n.ID, domain.Period{}) })
	}
	return m, nil
}

func (m Model) handleNoteDetailKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	open := *m.openNote
	switch msg.String() {
	case "esc":
		m.openNote = nil
	case "space":
		body := note.Toggle(open.BodyMD, m.detail.cursor)
		return m, write(func() error { return catalog.SetNoteBody(m.db, open.ID, body) })
	case "e":
		return m.openModal(m.newNoteEditor(open))
	}
	return m, nil
}

func (m Model) newNoteForm() *form {
	var title string
	period := m.period
	return newForm(m.theme, m.width, m.height,
		[]*huh.Group{
			huh.NewGroup(
				huh.NewInput().Title("Note title").Value(&title).Validate(huh.ValidateNotEmpty()),
			).Title("New note"),
		},
		func() tea.Cmd {
			return write(func() error {
				_, err := catalog.CreateNote(m.db, catalog.Note{Title: title, Period: period})
				return err
			})
		})
}

type noteEditor struct {
	area       textarea.Model
	original   string
	discarding bool
	save       func(string) tea.Cmd
}

func (m Model) newNoteEditor(n catalog.Note) *noteEditor {
	area := textarea.New()
	area.SetValue(n.BodyMD)
	area.SetWidth(m.detailWidth())
	area.SetHeight(m.bodyHeight(2))
	area.Focus()

	return &noteEditor{
		area:     area,
		original: n.BodyMD,
		save: func(body string) tea.Cmd {
			return write(func() error { return catalog.SetNoteBody(m.db, n.ID, body) })
		},
	}
}

func (e *noteEditor) Update(msg tea.Msg) (modal, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		e.area.SetWidth(min(size.Width-6, readableColumn))
		e.area.SetHeight(max(size.Height-9, 3))
		return e, nil
	}

	key, isKey := msg.(tea.KeyPressMsg)
	if isKey && e.discarding {
		switch key.String() {
		case "y":
			return nil, nil
		case "n", "esc":
			e.discarding = false
		}
		return e, nil
	}
	if isKey {
		switch key.String() {
		case "ctrl+s":
			return nil, e.save(e.area.Value())
		case "esc":
			if e.area.Value() == e.original {
				return nil, nil
			}
			e.discarding = true
			return e, nil
		}
	}

	var cmd tea.Cmd
	e.area, cmd = e.area.Update(msg)
	return e, cmd
}

func (e *noteEditor) Init() tea.Cmd { return nil }

func (e *noteEditor) View() string {
	if e.discarding {
		return "discard changes to this note?  y / n"
	}
	return e.area.View()
}

func (e *noteEditor) Help() string {
	if e.discarding {
		return "y discard · n keep editing"
	}
	return "ctrl+s save · esc cancel"
}

func (m Model) renderNotes() string {
	if m.openNote != nil {
		return m.renderNoteDetail()
	}
	title := m.theme.Muted.Render("Notes · " + m.period.String())
	if len(m.shownNotes()) == 0 {
		return title + "\n\n" + m.centerInBox(m.theme.Muted.Render("no notes here — press n to write one"), 2)
	}
	return title + "\n\n" + m.notesList.View()
}

func (m Model) renderNoteDetail() string {
	title := m.theme.Muted.Render("Notes · ") + m.theme.Title.Render(m.openNote.Title)
	body := lipgloss.PlaceHorizontal(m.contentWidth(), lipgloss.Center, m.detail.View())
	return title + "\n\n" + body
}

func (m Model) noteRows() ([]string, []int) {
	labelled := []struct {
		label string
		notes []catalog.Note
	}{
		{"PINNED", m.pinnedNotes()},
		{strings.ToUpper(m.period.String()), m.periodNotes()},
	}

	groups := make([]group, len(labelled))
	index := 0
	for i, l := range labelled {
		rendered := make([]string, len(l.notes))
		for j, n := range l.notes {
			rendered[j] = m.renderNoteRow(n, index == m.notesList.cursor)
			index++
		}
		groups[i] = group{label: groupStyle(i).Render(l.label), rows: rendered}
	}
	return groupedRows(groups)
}

func (m Model) renderNoteRow(n catalog.Note, selected bool) string {
	cursor := "  "
	if selected {
		cursor = m.theme.Accent.Render("> ")
	}
	check := "[ ] "
	if n.Done {
		check = "[x] "
	}

	style := m.theme.Bright
	if n.Done {
		style = m.theme.Muted
	}
	title := style.Width(titleWidth).MaxWidth(titleWidth).Render(n.Title)

	progress := ""
	if done, total := note.Progress(n.BodyMD); total > 0 {
		progress = m.theme.Muted.Render(fmt.Sprintf("%d/%d", done, total))
	}
	return cursor + check + title + " " + progress
}

func (m Model) noteDetailRows() (rows []string, anchors []int) {
	rendered, err := renderMarkdown(m.openNote.BodyMD, m.detailWidth()-2, m.theme.Dark)
	if err != nil {
		return []string{m.theme.Muted.Render("markdown error: " + err.Error())}, nil
	}

	boxes := len(note.Checkboxes(m.openNote.BodyMD))
	box := -1

	for _, line := range strings.Split(strings.TrimRight(rendered, "\n"), "\n") {
		gutter := "  "
		if box+1 < boxes && startsWithCheckbox(line) {
			box++
			if box == m.detail.cursor {
				gutter = m.theme.Accent.Render("> ")
			}
			anchors = append(anchors, len(rows))
		}
		rows = append(rows, gutter+line)
	}
	return rows, anchors
}
