package tui

import (
	"strings"

	glamour "charm.land/glamour/v2"
)

// checkboxGlyphs are what Glamour's dark and light styles both emit for a
// task-list item, which is how a rendered line is found without re-parsing
// the markdown.
var checkboxGlyphs = []string{"[ ] ", "[✓] "}

func renderMarkdown(body string, width int, dark bool) (string, error) {
	width = max(width, 20)
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

// startsWithCheckbox reports whether a task-list glyph opens the line's
// visible content. Glamour always puts the marker first, so a "[ ] " sitting
// inside prose is not a checkbox and must not take a cursor position — the
// cursor indexes the source, and a phantom here would shift every toggle
// after it onto the wrong line.
func startsWithCheckbox(line string) bool {
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == ' ' || line[i] == '\t':
		case line[i] == escape:
			for i < len(line) && line[i] != 'm' {
				i++
			}
		default:
			return hasCheckboxGlyph(line[i:])
		}
	}
	return false
}

const escape = 0x1b

func hasCheckboxGlyph(s string) bool {
	for _, g := range checkboxGlyphs {
		if strings.HasPrefix(s, g) {
			return true
		}
	}
	return false
}
