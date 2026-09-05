package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/gabokatta/mess/internal/fixture"
)

// openForms is every modal the app can put on screen, by the key that opens it.
func openForms(t *testing.T) map[string]Model {
	t.Helper()
	base := modelFor(t, fixture.Demo(fixture.Period), 150, 40)

	opened := map[string]Model{}
	open := func(name string, view view, keys ...string) {
		t.Helper()
		m := base
		m.view = view
		m = m.sync()
		var cmd tea.Cmd
		for _, k := range keys {
			m, cmd = send(t, m, key(k))
			m, _ = pump(t, m, cmd)
		}
		if m.topModal() == nil {
			t.Fatalf("%s: pressing %v opened no modal", name, keys)
		}
		opened[name] = m
	}

	open("concept edit", viewConcepts, "e")
	open("concept new", viewConcepts, "n")
	open("concept delete", viewConcepts, "d")
	open("categories", viewConcepts, "c")
	open("category new", viewConcepts, "c", "n")
	open("category rename", viewConcepts, "c", "r")
	open("category delete", viewConcepts, "c", "d")
	open("note new", viewNotes, "n")
	open("manual rate", viewRates, "e")
	return opened
}

// Every modal is a card: drawn at its own size and placed, not stretched over
// the screen with its fields in one corner.
func TestEveryModalIsACentredCard(t *testing.T) {
	for name, m := range openForms(t) {
		t.Run(name, func(t *testing.T) {
			view := stripANSI(m.topModal().View())
			if !strings.HasPrefix(view, "╭") {
				t.Errorf("modal is not drawn in a card:\n%s", view)
			}
			if got := lipgloss.Width(view); got > m.contentWidth() {
				t.Errorf("card is %d columns, wider than the %d it has", got, m.contentWidth())
			}

			var boxed []string
			for _, line := range strings.Split(stripANSI(m.renderBody()), "\n") {
				if strings.ContainsAny(line, "│╭╰") {
					boxed = append(boxed, line)
				}
			}
			if len(boxed) == 0 {
				t.Fatalf("no card in the body:\n%s", stripANSI(m.renderBody()))
			}
			for _, line := range boxed {
				left := len(line) - len(strings.TrimLeft(line, " "))
				right := len(line) - len(strings.TrimRight(line, " "))
				if diff := left - right; diff > 1 || diff < -1 {
					t.Errorf("card sits %d from the left and %d from the right:\n%q", left, right, line)
				}
			}
		})
	}
}

// huh positions a confirm's buttons against the width of its title, so a title
// that wraps inside the card pushes Delete and Keep off its right edge.
func TestConfirmKeepsItsButtons(t *testing.T) {
	forms := openForms(t)
	for _, name := range []string{"concept delete", "category delete"} {
		t.Run(name, func(t *testing.T) {
			view := stripANSI(forms[name].topModal().View())
			var buttons string
			for _, line := range strings.Split(view, "\n") {
				if strings.Contains(line, "Keep") {
					buttons = line
				}
			}
			if buttons == "" {
				t.Fatalf("the confirm has no buttons:\n%s", view)
			}
			if !strings.Contains(buttons, "Delete") {
				t.Errorf("button row = %q, want both choices", buttons)
			}
		})
	}
}

// The keys live in the app's help row, the same as on every screen, so the
// card is fields and nothing else.
func TestModalCardsDoNotDrawTheirOwnHelp(t *testing.T) {
	for name, m := range openForms(t) {
		t.Run(name, func(t *testing.T) {
			view := stripANSI(m.topModal().View())
			for _, hint := range []string{"enter next", "enter submit", "→ toggle"} {
				if strings.Contains(view, hint) {
					t.Errorf("card draws its own help %q, which the help row already carries:\n%s", hint, view)
				}
			}
			if m.help() == "" {
				t.Error("the help row is empty while a modal is open")
			}
		})
	}
}

// A long concept name cannot make the card outgrow the terminal it is on.
func TestFormCardFitsTheNarrowestTerminal(t *testing.T) {
	m := modelFor(t, fixture.Demo(fixture.Period), minUsableWidth, minUsableHeight)
	m.view = viewConcepts
	m = m.sync()

	m, cmd := send(t, m, key("e"))
	m, _ = pump(t, m, cmd)

	for i, line := range strings.Split(m.View().Content, "\n") {
		if got := lineWidth(line); got > minUsableWidth {
			t.Fatalf("line %d is %d columns on a %d-column terminal:\n%q",
				i, got, minUsableWidth, stripANSI(line))
		}
	}
}

// huh's Charm theme paints an unselected option in ANSI 235, which is
// near-black and vanishes against anything but a near-white background. Every
// choice on a card has to be readable, not just the one under the cursor.
func TestUnselectedOptionsCarryTheAppsForeground(t *testing.T) {
	m := modelFor(t, fixture.Demo(fixture.Period), 150, 40)
	m.view = viewConcepts
	m = m.sync()
	m, cmd := send(t, m, key("e"))
	m, _ = pump(t, m, cmd)

	want := m.theme.Bright.Render("Utilities")
	view := m.topModal().View()
	if !strings.Contains(view, want) {
		t.Errorf("an unselected option is not in the app's foreground:\n%s", view)
	}
	for _, dim := range []string{"\x1b[38;5;235m", "\x1b[38;5;238m", "\x1b[38;5;243m"} {
		if strings.Contains(view, dim) {
			t.Errorf("the card still carries a near-black ANSI colour %q:\n%s", dim, view)
		}
	}
}

// huh keeps a style's glyph and padding inside the style, so replacing one
// wholesale loses the "> " on a selector and the space around a button.
func TestFormStylesKeepTheirGlyphsAndPadding(t *testing.T) {
	forms := openForms(t)

	edit := stripANSI(forms["concept edit"].topModal().View())
	if !strings.Contains(edit, "> Freelance Income") {
		t.Errorf("the focused field lost its selector:\n%s", edit)
	}

	confirm := stripANSI(forms["concept delete"].topModal().View())
	if strings.Contains(confirm, "DeleteKeep") {
		t.Errorf("the buttons lost the padding between them:\n%s", confirm)
	}
	if !strings.Contains(confirm, "Delete") || !strings.Contains(confirm, "Keep") {
		t.Errorf("the confirm is missing a button:\n%s", confirm)
	}
}
