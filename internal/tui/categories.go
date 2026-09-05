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

func (m Model) conceptCounts() map[int64]int {
	counts := make(map[int64]int, len(m.categories))
	for _, c := range m.concepts {
		counts[c.CategoryID]++
	}
	return counts
}

// Copy before painting optimistic color changes so the modal does not mutate Model state.
func (l *categoryList) refresh(categories []catalog.Category, counts map[int64]int) {
	l.categories = append(l.categories[:0:0], categories...)
	l.counts = counts
	l.show()
}

func (l *categoryList) show() {
	rows := make([]string, len(l.categories))
	for i, c := range l.categories {
		rows[i] = l.row(c, i == l.list.cursor)
	}
	l.list = l.list.show(rows, rowAnchors(len(rows)), categoriesWidth,
		viewportHeight(len(rows), l.visibleRows()))
}

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

// Persist each press; the category modal has no separate save action.
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

func (l *categoryList) View() string {
	return l.theme.card(l.header() + "\n" + l.list.View() + l.theme.scrollHint(l.list, gutterWidth))
}

func (l *categoryList) header() string {
	return l.theme.Muted.Render(strings.Repeat(" ", gutterWidth) +
		leftCol(categoryNameWidth, "CATEGORY") + leftCol(categoryColorWidth, "COLOUR") +
		lipgloss.NewStyle().Width(categoryCountWidth).Align(lipgloss.Right).Render("CONCEPTS"))
}

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
