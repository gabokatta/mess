package tui

import (
	"strings"

	glamour "charm.land/glamour/v2"
	"github.com/charmbracelet/x/ansi"
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

// Only a leading glyph counts; glyphs inside prose must not shift checkbox indices.
func startsWithCheckbox(line string) bool {
	return hasCheckboxGlyph(strings.TrimLeft(ansi.Strip(line), " \t"))
}

func hasCheckboxGlyph(s string) bool {
	for _, g := range checkboxGlyphs {
		if strings.HasPrefix(s, g) {
			return true
		}
	}
	return false
}
