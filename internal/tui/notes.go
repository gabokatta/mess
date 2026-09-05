package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/note"
)

const (
	readableColumn = 76

	noteTitleWidth  = 30
	noteMinProg     = 5
	noteStatusWidth = 6
	noteFromWidth   = 7

	// The list is fixed and the pane takes what is left beside it: the list
	// holds labels and short figures, the pane holds someone's prose.
	notesGap      = 6
	notePaneFloor = 50
)

type notesFocusArea int

const (
	focusList notesFocusArea = iota
	focusBody
)

func (m Model) noteProgWidth() int {
	width := noteMinProg
	for _, n := range m.shownNotes() {
		if done, total := note.Progress(n.BodyMD); total > 0 {
			width = max(width, lipgloss.Width(formatProgress(done, total)))
		}
	}
	return width
}

func (m Model) noteListWidth() int {
	return gutterWidth + noteTitleWidth + colGap + m.noteProgWidth() +
		colGap + noteStatusWidth + colGap + noteFromWidth
}

func formatProgress(done, total int) string { return fmt.Sprintf("%d / %d", done, total) }

// The list has fixed columns; the body takes the remaining readable width.
func (m Model) notePaneWidth() int {
	room := m.contentWidth() - m.noteListWidth() - notesGap
	return min(max(room, notePaneFloor), readableColumn)
}

func (m Model) editorWidth() int { return min(m.contentWidth(), readableColumn) }

// noteGroup is one labelled block of the list. Membership is a query, so a
// write can move a note from one group to another, or out of the list.
type noteGroup struct {
	label string
	notes []catalog.Note
}

func (m Model) noteGroups() []noteGroup {
	carried := m.selectNotes(m.carried)
	// Oldest first: the thing avoided longest leads the block.
	sort.SliceStable(carried, func(i, j int) bool { return carried[i].Period.Before(carried[j].Period) })

	candidates := []noteGroup{
		{label: "PINNED", notes: m.selectNotes(func(n catalog.Note) bool { return n.Period.IsZero() })},
		{label: "THIS MONTH", notes: m.selectNotes(func(n catalog.Note) bool { return n.Period.Equal(m.period) })},
		{label: "CARRIED OVER", notes: carried},
	}

	// An empty group is dropped rather than rendered blank, so this is the
	// shape of the list and not a list of the shapes it might take.
	var groups []noteGroup
	for _, g := range candidates {
		if len(g.notes) > 0 {
			groups = append(groups, g)
		}
	}
	return groups
}

// Closing an older note removes it from this period's carried-over group.
func (m Model) carried(n catalog.Note) bool {
	return !n.Done && !n.Period.IsZero() && n.Period.Before(m.period)
}

func (m Model) selectNotes(keep func(catalog.Note) bool) []catalog.Note {
	var out []catalog.Note
	for _, n := range m.notes {
		if keep(n) {
			out = append(out, n)
		}
	}
	return out
}

// Flatten the displayed groups in cursor order.
func (m Model) shownNotes() []catalog.Note {
	var out []catalog.Note
	for _, g := range m.noteGroups() {
		out = append(out, g.notes...)
	}
	return out
}

func (m Model) cursorNote() (catalog.Note, bool) {
	shown := m.shownNotes()
	if m.notesList.cursor >= len(shown) {
		return catalog.Note{}, false
	}
	return shown[m.notesList.cursor], true
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
		m.notesFocus = focusBody
		m.detail.cursor = 0
	case "space":
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

func (m Model) handleNoteBodyKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	n, ok := m.cursorNote()
	if !ok {
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.notesFocus = focusList
	case "e":
		return m.openModal(m.newNoteEditor(n))
	case "space":
		// The cursor is clamped on render, but a reload can shorten the note
		// under it between one keypress and the next.
		lines := m.noteBodyLines()
		if m.detail.cursor >= len(lines) {
			return m, nil
		}
		box, ok := lines[m.detail.cursor].ticks()
		if !ok {
			return m, nil
		}
		body := note.Toggle(n.BodyMD, box)
		return m, write(func() error { return catalog.SetNoteBody(m.db, n.ID, body) })
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
	area.SetWidth(m.editorWidth())
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
	if len(m.shownNotes()) == 0 {
		return m.periodHeading() + "\n\n" +
			m.centerInBox(m.theme.Muted.Render("no notes here — press n to write one"), 2)
	}

	list := m.periodHeading() + "\n\n" + m.noteColumnHeader() + "\n" + m.notesList.View() +
		m.scrollHint(m.notesList, gutterWidth) + "\n\n" + m.notesMeta()
	pane := m.renderNotePane()

	listLead, paneLead := m.notesLeading(lipgloss.Height(list), lipgloss.Height(pane))
	card := lipgloss.JoinHorizontal(lipgloss.Top,
		strings.Repeat("\n", listLead)+list,
		strings.Repeat(" ", notesGap),
		strings.Repeat("\n", paneLead)+pane)

	top := max(0, (m.bodyHeight(0)-lipgloss.Height(card))/2)
	left := max(0, (m.contentWidth()-lipgloss.Width(card))/2)
	return lipgloss.NewStyle().MarginLeft(left).Render(strings.Repeat("\n", top) + card)
}

func (m Model) renderNotePane() string {
	n, ok := m.cursorNote()
	if !ok {
		return ""
	}
	width := m.notePaneWidth()
	title := m.theme.Title.Render(ansi.Truncate(strings.ToUpper(n.Title), width, "…"))
	facts := m.theme.Muted.Render(ansi.Truncate(strings.Join(m.noteFacts(n), " · "), width, "…"))
	return title + "\n" + facts + "\n" + m.detail.View() + m.scrollHint(m.detail, gutterWidth)
}

func (m Model) noteFacts(n catalog.Note) []string {
	facts := []string{noteStatusWord(n)}
	switch {
	case n.Period.IsZero():
		facts = append(facts, "pinned")
	case m.carried(n):
		facts = append(facts, "from "+n.Period.String())
	}
	if done, total := note.Progress(n.BodyMD); total > 0 {
		facts = append(facts, fmt.Sprintf("%d / %d ticked", done, total))
	}
	return facts
}

func (m Model) noteColumnHeader() string {
	row := strings.Repeat(" ", gutterWidth) + leftCol(noteTitleWidth, "NOTE") +
		lipgloss.NewStyle().Width(m.noteProgWidth()).Align(lipgloss.Right).Render("PROG") +
		strings.Repeat(" ", colGap) + leftCol(noteStatusWidth, "STATUS") + "FROM"
	return m.theme.Muted.Render(row)
}

func (m Model) notesMeta() string {
	shown := m.shownNotes()
	done, carried := 0, 0
	for _, n := range shown {
		if n.Done {
			done++
		}
		if m.carried(n) {
			carried++
		}
	}

	lines := []string{m.theme.Muted.Render(fmt.Sprintf("done  %d / %d", done, len(shown)))}
	if carried > 0 {
		lines = append(lines, m.theme.Alert.Render(fmt.Sprintf("carried  %d", carried)))
	}
	return strings.Join(lines, "\n")
}

func (m Model) notesAvailHeight() int {
	const headingLine, blankLine = 1, 1
	return max(m.bodyHeight(headingLine+blankLine), 1)
}

func (m Model) notesListHeight(rows int) int {
	const columnHeaderLine, metaBlank = 1, 1
	return min(columnHeaderLine+rows+metaBlank+lipgloss.Height(m.notesMeta()), m.notesAvailHeight())
}

// Keep the list's position stable while a taller body uses the space above and below it.
func (m Model) notesLeading(list, pane int) (listLead, paneLead int) {
	const headingLines = 2
	if extra := pane - list; extra > 0 {
		return extra / 2, 0
	}
	return 0, headingLines
}

func (m Model) listViewHeight(rows int) int {
	const columnHeaderLine, metaBlank = 1, 1
	return max(m.notesListHeight(rows)-columnHeaderLine-metaBlank-lipgloss.Height(m.notesMeta()), 1)
}

func (m Model) paneViewHeight() int {
	const titleLine, factsLine = 1, 1
	return max(m.bodyHeight(0)-titleLine-factsLine, 1)
}

func (m Model) noteRows() ([]string, []int) {
	labelled := m.noteGroups()
	groups := make([]group, len(labelled))
	index := 0
	for i, l := range labelled {
		rendered := make([]string, len(l.notes))
		for j, n := range l.notes {
			rendered[j] = m.renderNoteRow(n, index == m.notesList.cursor)
			index++
		}
		groups[i] = group{label: m.ruleHeader(l.label, m.noteListWidth()), rows: rendered}
	}
	return groupedRows(groups)
}

func (m Model) renderNoteRow(n catalog.Note, selected bool) string {
	cursor := strings.Repeat(" ", gutterWidth)
	if selected {
		style := m.theme.Accent
		if m.notesFocus == focusBody {
			style = m.theme.Muted
		}
		cursor = style.Render("> ")
	}
	// Truncated before Style.Width sees it: Width wraps the overflow onto a
	// second line, which desyncs the scroller's one-line-per-row cursor math.
	title := m.theme.Bright.Width(noteTitleWidth).
		Render(ansi.Truncate(n.Title, noteTitleWidth, "…"))

	progress := ""
	if done, total := note.Progress(n.BodyMD); total > 0 {
		progress = formatProgress(done, total)
	}
	prog := m.theme.Muted.Width(m.noteProgWidth()).Align(lipgloss.Right).Render(progress)

	// Only a carried row needs its month named; for every other group the
	// heading above the row already says which one it is.
	from := ""
	if m.carried(n) {
		from = n.Period.String()
	}

	return cursor + title + strings.Repeat(" ", colGap) +
		prog + strings.Repeat(" ", colGap) + m.renderNoteStatus(n) +
		strings.Repeat(" ", colGap) + m.theme.Muted.Width(noteFromWidth).Render(from)
}

func (m Model) renderNoteStatus(n catalog.Note) string {
	style := m.theme.Bright
	if n.Done {
		style = m.theme.Muted
	}
	return style.Width(noteStatusWidth).Render(noteStatusWord(n))
}

func noteStatusWord(n catalog.Note) string {
	if n.Done {
		return "done"
	}
	return "open"
}

// box is the source checkbox index, or -1 for a non-actionable display line.
type bodyLine struct {
	text string
	box  int
}

// Only a checkbox's first display line is actionable, not wrapped continuations.
func (l bodyLine) ticks() (int, bool) { return l.box, l.box >= 0 }

func (m Model) noteBodyLines() []bodyLine {
	n, ok := m.cursorNote()
	if !ok {
		return nil
	}
	if strings.TrimSpace(n.BodyMD) == "" {
		return []bodyLine{{text: m.theme.Muted.Render("empty — press e to write"), box: -1}}
	}

	rendered, err := renderMarkdown(n.BodyMD, m.notePaneWidth()-gutterWidth, m.theme.Dark)
	if err != nil {
		return []bodyLine{{text: m.theme.Muted.Render("markdown error: " + err.Error()), box: -1}}
	}

	boxes := len(note.Checkboxes(n.BodyMD))
	box := -1

	var lines []bodyLine
	for _, text := range strings.Split(strings.TrimRight(rendered, "\n"), "\n") {
		ordinal := -1
		// Only an item's first line is actionable: a wrapped continuation
		// carries no glyph, and the muted gutter on it says so.
		if box+1 < boxes && startsWithCheckbox(text) {
			box++
			ordinal = box
		}
		lines = append(lines, bodyLine{text: text, box: ordinal})
	}
	return lines
}

func (m Model) noteBodyRows(lines []bodyLine) ([]string, []int) {
	rows := make([]string, len(lines))
	for i, l := range lines {
		gutter := strings.Repeat(" ", gutterWidth)
		if i == m.detail.cursor && m.notesFocus == focusBody {
			gutter = m.theme.Muted.Render("> ")
			if _, ok := l.ticks(); ok {
				gutter = m.theme.Accent.Render("> ")
			}
		}
		rows[i] = gutter + l.text
	}
	return rows, rowAnchors(len(rows))
}
