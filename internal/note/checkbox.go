// Package note holds the pure logic over a note's markdown body, whose only
// imposed structure is its task-list checkboxes.
package note

import (
	"regexp"
	"strings"
)

var checkboxPattern = regexp.MustCompile(`^(\s*[-*]\s\[)([ xX])(\]\s+)(.*)$`)

// Checkbox is one task-list item found in a note's markdown body.
type Checkbox struct {
	Text string
	Done bool
}

// Checkboxes returns every task-list item in body, in the document order
// Toggle addresses them by index.
func Checkboxes(body string) []Checkbox {
	var boxes []Checkbox
	for _, line := range strings.Split(body, "\n") {
		if m := checkboxPattern.FindStringSubmatch(line); m != nil {
			boxes = append(boxes, Checkbox{Text: m[4], Done: m[2] != " "})
		}
	}
	return boxes
}

// Progress counts done checkboxes against the total, for a note's "3/7".
func Progress(body string) (done, total int) {
	for _, c := range Checkboxes(body) {
		total++
		if c.Done {
			done++
		}
	}
	return done, total
}

// Toggle flips the nth checkbox in document order, rewriting only that
// line's marker. An unreported index is a no-op, so a stale cursor is safe.
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
