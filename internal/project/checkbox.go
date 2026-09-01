// Package project holds the pure logic over a project's markdown body: the
// only structure mess imposes on that text is its task-list checkboxes.
package project

import (
	"regexp"
	"strings"
)

// checkboxPattern matches a GFM task-list marker at the start of a line,
// after optional indentation: "- [ ] " or "- [x] " (case-insensitive), the
// only syntax mess treats as a checkbox rather than plain prose or a bullet.
var checkboxPattern = regexp.MustCompile(`^(\s*[-*]\s\[)([ xX])(\]\s+)(.*)$`)

// Checkbox is one task-list item found in a project's markdown body, in
// document order.
type Checkbox struct {
	Text string
	Done bool
}

// Checkboxes returns every task-list item in body, in document order —
// the same order Toggle addresses them by index.
func Checkboxes(body string) []Checkbox {
	var boxes []Checkbox
	for _, line := range strings.Split(body, "\n") {
		if m := checkboxPattern.FindStringSubmatch(line); m != nil {
			boxes = append(boxes, Checkbox{Text: m[4], Done: m[2] != " "})
		}
	}
	return boxes
}

// Progress counts done checkboxes against the total, for a project's "3/7"
// summary.
func Progress(body string) (done, total int) {
	for _, c := range Checkboxes(body) {
		total++
		if c.Done {
			done++
		}
	}
	return done, total
}

// Toggle flips the nth checkbox in document order (0-based, Checkboxes'
// order), rewriting only that line's marker. An index Checkboxes didn't
// report is a no-op, so a stale cursor can never corrupt the body.
func Toggle(body string, n int) string {
	lines := strings.Split(body, "\n")
	i := -1
	for lineNo, line := range lines {
		m := checkboxPattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		i++
		if i != n {
			continue
		}
		mark := "x"
		if m[2] != " " {
			mark = " "
		}
		lines[lineNo] = m[1] + mark + m[3] + m[4]
		return strings.Join(lines, "\n")
	}
	return body
}
