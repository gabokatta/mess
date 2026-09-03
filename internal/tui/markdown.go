package tui

import (
	"strings"

	glamour "charm.land/glamour/v2"
)

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

// Only a leading glyph counts: a "[ ] " inside prose would take a cursor
// position and shift every toggle after it onto the wrong line.
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
