package tui

import (
	"database/sql"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/gabokatta/mess/internal/catalog"
)

// Column budget inside the box, left to right: cursor gutter, name, gap,
// colour, gap, count.
const (
	categoryNameWidth  = 18
	categoryColorWidth = 6
	categoryCountWidth = 8

	categoriesWidth = gutterWidth + categoryNameWidth + colGap +
		categoryColorWidth + colGap + categoryCountWidth
)

// categoryList is the catalog's other half: the one place a category is
// created, renamed, recoloured, or deleted.
//
// It holds its own copy of the catalog because a modal's View takes no
// arguments. sync refreshes that copy after every write, so the list is never
// reading state the database has moved past.
type categoryList struct {
	theme      Theme
	db         *sql.DB
	width      int
	height     int
	categories []catalog.Category
	counts     map[int64]int
	list       scroller
}

func (m Model) categoryList() *categoryList {
	list := &categoryList{theme: m.theme, db: m.db, width: m.width, height: m.height}
	list.refresh(m.categories, m.conceptCounts())
	return list
}

// conceptCounts says what each category is holding, which is what makes a
// delete refusal actionable and an empty category obvious.
func (m Model) conceptCounts() map[int64]int {
	counts := make(map[int64]int, len(m.categories))
	for _, c := range m.concepts {
		counts[c.CategoryID]++
	}
	return counts
}

// The list keeps its own copy rather than the Model's slice: it paints a
// colour locally as the key is pressed, and that write has no business
// reaching into state the Model owns.
func (l *categoryList) refresh(categories []catalog.Category, counts map[int64]int) {
	l.categories = append(l.categories[:0:0], categories...)
	l.counts = counts
	l.show()
}

// show hands the rows to the scroller, so a catalog longer than the terminal
// pans instead of spilling out of the box.
func (l *categoryList) show() {
	rows := make([]string, len(l.categories))
	for i, c := range l.categories {
		rows[i] = l.row(c, i == l.list.cursor)
	}
	l.list = l.list.show(rows, rowAnchors(len(rows)), categoriesWidth,
		viewportHeight(len(rows), l.visibleRows()))
}

// visibleRows is what the box can hold: the terminal less the app's frame, the
// modal's title, its border and padding, and the column header.
func (l *categoryList) visibleRows() int {
	const chrome = 12
	return max(l.height-chrome, 3)
}

func (l *categoryList) cursorCategory() (catalog.Category, bool) {
	if l.list.cursor >= len(l.categories) {
		return catalog.Category{}, false
	}
	return l.categories[l.list.cursor], true
}

func (l *categoryList) Update(msg tea.Msg) (modal, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return l, nil
	}
	switch key.String() {
	case "esc":
		return nil, nil
	case "up":
		l.list = l.list.move(-1, len(l.categories))
		l.show()
	case "down":
		l.list = l.list.move(1, len(l.categories))
		l.show()
	case "left":
		return l, l.shiftColor(-1)
	case "right":
		return l, l.shiftColor(1)
	case "n":
		return l.createForm(), nil
	case "r":
		if c, ok := l.cursorCategory(); ok {
			return l.renameForm(c), nil
		}
	case "d":
		if c, ok := l.cursorCategory(); ok {
			return l.deleteForm(c), nil
		}
	}
	return l, nil
}

// createForm is the only way a category is born. The concept form picks from
// a list and never makes one, so categories exist before concepts use them.
func (l *categoryList) createForm() *form {
	var name string
	return newForm(l.theme, l.width, l.height, []*huh.Group{
		huh.NewGroup(
			huh.NewInput().Title("New category").Value(&name).Validate(huh.ValidateNotEmpty()),
		),
	}, func() tea.Cmd {
		return write(func() error {
			_, err := catalog.AppendCategory(l.db, name)
			return err
		})
	})
}

// renameForm opens over the list and returns to it, which is what the modal
// stack is for. Every concept keeps its category_id, so a rename is one
// UPDATE and not a migration.
func (l *categoryList) renameForm(c catalog.Category) *form {
	name := c.Name
	return newForm(l.theme, l.width, l.height, []*huh.Group{
		huh.NewGroup(
			huh.NewInput().Title("Rename " + c.Name).Value(&name).Validate(huh.ValidateNotEmpty()),
		),
	}, func() tea.Cmd {
		return write(func() error { return catalog.RenameCategory(l.db, c.ID, name) })
	})
}

// shiftColor writes on every press. The index is a small integer on a tiny
// table, so cycling is cheap, and a write per press means there is no unsaved
// state to lose to an esc.
func (l *categoryList) shiftColor(delta int) tea.Cmd {
	c, ok := l.cursorCategory()
	if !ok {
		return nil
	}
	next := ((c.ColorIndex+delta)%catalog.PaletteSize + catalog.PaletteSize) % catalog.PaletteSize

	// Painted locally as well as written, so the swatch moves under the key
	// rather than a reload later.
	l.categories[l.list.cursor].ColorIndex = next
	l.show()
	return write(func() error { return catalog.SetCategoryColor(l.db, c.ID, next) })
}

// deleteForm is deleteConceptForm's shape, so the same gesture reads the same
// way whichever half of the catalog it is aimed at. Whether the delete is
// allowed is the schema's question, asked by running it.
func (l *categoryList) deleteForm(c catalog.Category) *form {
	var confirmed bool
	f := newForm(l.theme, l.width, l.height, []*huh.Group{
		huh.NewGroup(
			huh.NewConfirm().Title("Delete " + c.Name + "?").
				Description("A category is only deleted when nothing is in it.").
				Affirmative("Delete").Negative("Keep").Value(&confirmed),
		),
	}, func() tea.Cmd {
		if !confirmed {
			return nil
		}
		return write(func() error { return catalog.DeleteCategory(l.db, c.ID) })
	})
	f.help = "←/→ choose · enter confirm · esc cancel"
	return f
}

func (l *categoryList) Init() tea.Cmd { return nil }

func (l *categoryList) Help() string {
	return "↑/↓ · ←/→ colour · n new · r rename · d delete · esc close"
}

// The card takes its width from the rows, which are already fixed at
// categoriesWidth by their cells.
func (l *categoryList) View() string {
	return l.theme.card(l.header() + "\n" + l.list.View() + l.theme.scrollHint(l.list, gutterWidth))
}

func (l *categoryList) header() string {
	return l.theme.Muted.Render(strings.Repeat(" ", gutterWidth) +
		leftCol(categoryNameWidth, "CATEGORY") + leftCol(categoryColorWidth, "COLOUR") +
		lipgloss.NewStyle().Width(categoryCountWidth).Align(lipgloss.Right).Render("CONCEPTS"))
}

// The index beside the swatch is not decoration. The palette is Okabe-Ito
// because it survives colour blindness, and the one thing a swatch cannot say
// to a deuteranope is that two categories now share a colour.
func (l *categoryList) row(c catalog.Category, selected bool) string {
	cursor := strings.Repeat(" ", gutterWidth)
	if selected {
		cursor = l.theme.Accent.Render("> ")
	}
	swatch := lipgloss.NewStyle().Foreground(palette[c.ColorIndex]).Render("●")
	colour := lipgloss.NewStyle().Width(categoryColorWidth).
		Render(swatch + " " + l.theme.Bright.Render(fmt.Sprintf("%d", c.ColorIndex+1)))

	return cursor + l.theme.Bright.Width(categoryNameWidth).
		Render(ansi.Truncate(c.Name, categoryNameWidth, "…")) +
		strings.Repeat(" ", colGap) + colour + strings.Repeat(" ", colGap) +
		l.theme.Muted.Width(categoryCountWidth).Align(lipgloss.Right).
			Render(fmt.Sprintf("%d", l.counts[c.ID]))
}
