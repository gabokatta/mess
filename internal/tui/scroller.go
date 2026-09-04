package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

// The app's list typography: "> " is what a cursor paints in the gutter, and
// one blank column is what separates two columns of a row. Both are facts
// about the drawing, not design choices a screen makes for itself, so they are
// shared. Column widths are not: those belong to the screen that draws them.
const (
	gutterWidth = 2
	colGap      = 2
)

type scroller struct {
	vp     viewport.Model
	cursor int
}

func (s scroller) show(rows []string, anchors []int, width, height int) scroller {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	s.vp.SetWidth(width)
	s.vp.SetHeight(height)
	s.vp.SetContent(strings.Join(rows, "\n"))

	s.cursor = clamp(s.cursor, len(anchors))
	if len(anchors) == 0 {
		s.vp.SetYOffset(0)
		return s
	}

	// One row of context above keeps the cursor's group header on screen.
	line := anchors[s.cursor]
	switch {
	case line-1 < s.vp.YOffset():
		s.vp.SetYOffset(max(line-1, 0))
	case line >= s.vp.YOffset()+height:
		s.vp.SetYOffset(line - height + 1)
	}
	return s
}

type group struct {
	label string
	rows  []string
}

func groupedRows(groups []group) (rows []string, anchors []int) {
	for _, g := range groups {
		if len(g.rows) == 0 {
			continue
		}
		if len(rows) > 0 {
			rows = append(rows, "")
		}
		rows = append(rows, g.label)
		for _, r := range g.rows {
			anchors = append(anchors, len(rows))
			rows = append(rows, r)
		}
	}
	return rows, anchors
}

// hidden counts the rows scrolled off each end of the viewport, so a view can
// tell the reader there is more list than there is screen.
func (s scroller) hidden() (above, below int) {
	above = s.vp.YOffset()
	return above, max(s.vp.TotalLineCount()-above-s.vp.Height(), 0)
}

// scrollHint says how much list is off screen, so a cut-off list reads as
// scrollable rather than as finished. It is empty when everything fits.
func (m Model) scrollHint(s scroller, indent int) string {
	above, below := s.hidden()
	if above == 0 && below == 0 {
		return ""
	}

	var marks []string
	if above > 0 {
		marks = append(marks, fmt.Sprintf("↑ %d", above))
	}
	if below > 0 {
		marks = append(marks, fmt.Sprintf("↓ %d", below))
	}
	return "\n" + m.theme.Muted.Render(strings.Repeat(" ", indent)+strings.Join(marks, " · ")+" more")
}

// viewportHeight is a list's viewport height: its own size when the content
// fits, otherwise the room available less a line kept back for the hint that
// says the list is cut.
func viewportHeight(rows, avail int) int {
	if rows > avail {
		return max(avail-1, 1)
	}
	return max(rows, 1)
}

// ruleHeader is a section label with a muted rule running out to width. It is
// structural, not categorical: hue across this app means category, so a group
// label carries weight and a rule instead of a colour.
func (m Model) ruleHeader(label string, width int) string {
	rule := strings.Repeat("─", max(width-lipgloss.Width(label)-1, 0))
	return m.theme.Title.Render(label) + " " + m.theme.Muted.Render(rule)
}

func (s scroller) move(delta, count int) scroller {
	s.cursor = clamp(s.cursor+delta, count)
	return s
}

func (s scroller) View() string { return s.vp.View() }

func clamp(cursor, count int) int {
	if count == 0 || cursor < 0 {
		return 0
	}
	if cursor >= count {
		return count - 1
	}
	return cursor
}

// rowAnchors is the anchor list for a flat list, where every row is its own
// stop. groupedRows builds the anchors for a list with headers in it.
func rowAnchors(count int) []int {
	anchors := make([]int, count)
	for i := range anchors {
		anchors[i] = i
	}
	return anchors
}
