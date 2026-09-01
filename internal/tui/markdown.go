package tui

import (
	"strings"

	glamour "charm.land/glamour/v2"
)

// checkboxGlyphs are the literal prefixes Glamour's dark and light styles
// both use for a task-list item — the fixed point that lets markCheckboxCursor
// find a checkbox's rendered line without re-parsing the markdown.
var checkboxGlyphs = []string{"[ ] ", "[✓] "}

func renderMarkdown(body string, width int, dark bool) (string, error) {
	style := glamour.WithStandardStyle("light")
	if dark {
		style = glamour.WithStandardStyle("dark")
	}
	r, err := glamour.NewTermRenderer(style, glamour.WithWordWrap(width))
	if err != nil {
		return "", err
	}
	return r.Render(body)
}

// markCheckboxCursor prefixes every rendered line with a two-column gutter,
// putting ">" on the target-th checkbox glyph in document order (Toggle's
// same indexing) and nothing everywhere else. target < 0 marks no line.
func markCheckboxCursor(rendered string, target int) string {
	lines := strings.Split(rendered, "\n")
	occurrence := -1
	for i, line := range lines {
		if containsCheckboxGlyph(line) {
			occurrence++
		}
		gutter := "  "
		if occurrence == target {
			gutter = "> "
		}
		lines[i] = gutter + line
	}
	return strings.Join(lines, "\n")
}

func containsCheckboxGlyph(line string) bool {
	for _, g := range checkboxGlyphs {
		if strings.Contains(line, g) {
			return true
		}
	}
	return false
}
