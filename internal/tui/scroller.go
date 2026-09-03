package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
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
