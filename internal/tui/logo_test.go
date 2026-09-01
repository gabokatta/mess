package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

func TestOverlayLogoResumesTheLineBeforeItsTail(t *testing.T) {
	row := "│" + strings.Repeat(" ", 30) + "│"
	canvas := strings.Join([]string{row, row, row}, "\n")
	rowRunes := []rune(row)
	wantTail := string(rowRunes[len(rowRunes)-logoTail:])
	gap := strings.Repeat(" ", logoGap)

	got := overlayLogo(canvas, lipgloss.NewStyle())

	lines := strings.Split(got, "\n")
	if lines[0] != row {
		t.Errorf("row 0 = %q, want untouched (above the logo's rows)", lines[0])
	}
	for i, want := range logoLines {
		line := lines[len(lines)-len(logoLines)+i]
		wantSuffix := gap + want + gap + wantTail
		if !strings.HasSuffix(line, wantSuffix) {
			t.Errorf("row %d = %q, want it to end with %q (gap, glyph, gap, original tail)", i, line, wantSuffix)
		}
		if !strings.HasPrefix(line, "│") {
			t.Errorf("row %d = %q, want the left border char preserved", i, line)
		}
		if got, want := utf8.RuneCountInString(line), utf8.RuneCountInString(row); got != want {
			t.Errorf("row %d has %d runes, want %d (same width as before)", i, got, want)
		}
	}
}

func TestOverlayLogoNoopsWhenCanvasTooShort(t *testing.T) {
	canvas := strings.Repeat(" ", 20)

	got := overlayLogo(canvas, lipgloss.NewStyle())

	if got != canvas {
		t.Errorf("overlayLogo() = %q, want the canvas unchanged (fewer rows than the logo needs)", got)
	}
}

func TestOverlayLogoNoopsWhenRowTooNarrow(t *testing.T) {
	narrow := strings.Repeat(" ", utf8.RuneCountInString(logoLines[0]))
	canvas := strings.Join([]string{narrow, narrow}, "\n")

	got := overlayLogo(canvas, lipgloss.NewStyle())

	if got != canvas {
		t.Errorf("overlayLogo() = %q, want the canvas unchanged (no room)", got)
	}
}
