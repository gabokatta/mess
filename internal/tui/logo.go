package tui

import (
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

// logoLines is the mess wordmark, stamped into the bottom-right corner once
// the terminal is roomy enough not to crowd the content above it.
var logoLines = []string{
	`┏┳┓┏┓┏┏`,
	`┛┗┗┗ ┛┛`,
}

const (
	logoMinWidth  = 80
	logoMinHeight = 24
	logoGap       = 3
	// logoTail keeps the border's corner intact past the logo.
	logoTail = 8
)

// overlayLogo splices logoLines into canvas's trailing rows: logoGap blank
// columns, the glyph, logoGap more, then the row's original last logoTail
// runes untouched. Rows are sliced by rune count, so canvas and style must
// stay ANSI-free there — a color code would throw the column count off.
func overlayLogo(canvas string, style lipgloss.Style) string {
	lines := strings.Split(canvas, "\n")
	start := len(lines) - len(logoLines)
	if start < 0 {
		return canvas
	}
	for i, l := range logoLines {
		segment := 2*logoGap + utf8.RuneCountInString(l)
		row := []rune(lines[start+i])
		if len(row) < segment+logoTail {
			return canvas
		}
		cut := len(row) - logoTail - segment
		head, tail := row[:cut], row[len(row)-logoTail:]
		gap := strings.Repeat(" ", logoGap)
		lines[start+i] = string(head) + gap + style.Render(l) + gap + string(tail)
	}
	return strings.Join(lines, "\n")
}
