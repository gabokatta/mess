package tui

import (
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

// logoLines is the mess wordmark, stamped into the bottom-right corner:
// the upper line beside the help, the lower one in the border below it.
var logoLines = []string{
	`┏┳┓┏┓┏┏`,
	`┛┗┗┗ ┛┛`,
}

const (
	logoGap = 3
	// logoTail keeps the border's corner intact past the logo.
	logoTail = 8
)

var (
	logoWidth   = utf8.RuneCountInString(logoLines[0])
	logoSegment = 2*logoGap + logoWidth
)

// overlayLogo splices the logo's lower line into canvas's bottom border,
// keeping the row's last logoTail runes so the corner survives. The row is
// sliced by rune count, so the border must stay ANSI-free.
func overlayLogo(canvas string, style lipgloss.Style) string {
	lines := strings.Split(canvas, "\n")
	last := len(lines) - 1
	if last < 0 {
		return canvas
	}
	row := []rune(lines[last])
	cut := len(row) - logoTail - logoSegment
	if cut < 0 {
		return canvas
	}
	head, tail := row[:cut], row[len(row)-logoTail:]
	gap := strings.Repeat(" ", logoGap)
	lines[last] = string(head) + gap + style.Render(logoLines[1]) + gap + string(tail)
	return strings.Join(lines, "\n")
}
