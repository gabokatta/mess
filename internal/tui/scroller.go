package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
)

// scroller is a cursor over a list too long for the box. anchors gives the
// display line each selectable row starts on, so group headers and blank
// rows cost nothing to skip over.
type scroller struct {
	vp     viewport.Model
	cursor int
}

// show scrolls just far enough to keep the cursor's line visible.
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

	// One row of context above keeps the group header the cursor sits under
	// on screen; scrolling is minimal, so the list creeps rather than jumps.
	line := anchors[s.cursor]
	switch {
	case line-1 < s.vp.YOffset():
		s.vp.SetYOffset(max(line-1, 0))
	case line >= s.vp.YOffset()+height:
		s.vp.SetYOffset(line - height + 1)
	}
	return s
}

// group is one labelled run of selectable rows.
type group struct {
	label string
	rows  []string
}

// groupedRows separates groups with a blank line and reports the display
// line each row lands on, so the cursor skips the headers between them.
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
