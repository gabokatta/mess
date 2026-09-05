package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

const (
	gutterWidth = 2
	colGap      = 2
)

type scroller struct {
	vp     viewport.Model
	cursor int
}

func (s scroller) show(rows []string, anchors []int, width, height int) scroller {
	height = max(height, 1)
	s.vp.SetWidth(max(width, 1))
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

func (s scroller) hidden() (above, below int) {
	above = s.vp.YOffset()
	return above, max(s.vp.TotalLineCount()-above-s.vp.Height(), 0)
}

func (m Model) scrollHint(s scroller, indent int) string {
	return m.theme.scrollHint(s, indent)
}

func (t Theme) scrollHint(s scroller, indent int) string {
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
	return "\n" + t.Muted.Render(strings.Repeat(" ", indent)+strings.Join(marks, " · ")+" more")
}

// Reserve one row for the scroll hint only when the content overflows.
func viewportHeight(rows, avail int) int {
	if rows > avail {
		return max(avail-1, 1)
	}
	return max(rows, 1)
}

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

// Flat lists stop on every row; groupedRows skips group headers.
func rowAnchors(count int) []int {
	anchors := make([]int, count)
	for i := range anchors {
		anchors[i] = i
	}
	return anchors
}
