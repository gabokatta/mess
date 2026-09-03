package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
	"github.com/gabokatta/mess/internal/month"
)

// richWorld has something in every view, the way the app sits once its
// start-up reads have landed: three confirmed, done lines across kinds, one
// untouched chore, one note, and a couple of stored closes.
func richWorld() fixture.World {
	return fixture.World{
		Concepts: []fixture.Concept{
			{Name: "Salary", Category: "Earnings", Kind: catalog.Income},
			{Name: "Rent", Category: "Home", Kind: catalog.Expense},
			{Name: "Dollars", Category: "Home", Kind: catalog.Saving},
			{Name: "Wash the house", Category: "Home", Kind: catalog.Chore},
		},
		Entries: []fixture.Entry{
			{Concept: "Salary", Period: fixture.Period, Amount: "2400000", Done: true},
			{Concept: "Rent", Period: fixture.Period, Amount: "785000", Done: true},
			{Concept: "Dollars", Period: fixture.Period, Amount: "480000", Done: true},
		},
		Notes: []catalog.Note{{Title: "Ideas", BodyMD: "- [ ] buy a lamp"}},
		Rates: []fixture.Rate{
			{Period: domain.NewPeriod(fixture.Year, time.July), Value: "1400"},
			{Period: domain.NewPeriod(fixture.Year, time.August), Value: "1500"},
		},
	}
}

// Every view renders at a real terminal size without panicking, and none of
// them spills past the box.
func TestEveryViewRenders(t *testing.T) {
	const width, height = 100, 32
	m := modelFor(t, richWorld(), width, height)

	for range viewNames {
		t.Run(m.view.String(), func(t *testing.T) {
			content := m.View().Content
			for i, line := range strings.Split(content, "\n") {
				if got := lineWidth(line); got > width {
					t.Fatalf("line %d is %d columns wide, want at most %d:\n%s", i, got, width, line)
				}
			}
		})
		m, _ = send(t, m, key("tab"))
	}
}

// The header states wiring, not arithmetic: whatever ResolveTotals says for
// this world is what the header must show. totals_test.go is the only
// place the arithmetic itself is asserted.
func TestMonthHeaderShowsTheArithmetic(t *testing.T) {
	m := modelFor(t, richWorld(), 100, 32)
	header := stripANSI(m.monthHeader())

	totals := month.ResolveTotals(m.lines, m.fx().At(m.period))
	done, total := month.DoneCount(m.lines)

	want := []string{
		fixture.Period.String(),
		"current",
		fmt.Sprintf("%d of %d done", done, total),
		"available " + formatAmount(totals.Available.Amount()),
		"saved " + formatAmount(totals.Saved.Amount()),
		"pocket " + formatAmount(totals.Pocket.Amount()),
	}
	for _, w := range want {
		if !strings.Contains(header, w) {
			t.Errorf("header is missing %q:\n%s", w, header)
		}
	}
}

// A terminal too small for the layout says so, and says by how much.
func TestTooSmallTerminal(t *testing.T) {
	m := modelFor(t, richWorld(), 30, 8)
	content := stripANSI(m.View().Content)

	want := []string{
		"make the terminal bigger",
		"have  30 ×   8",
		fmt.Sprintf("need %3d × %3d", minUsableWidth, minUsableHeight),
	}
	for _, w := range want {
		if !strings.Contains(content, w) {
			t.Errorf("the too-small screen is missing %q:\n%s", w, content)
		}
	}
}

// The message exists to say which way to drag, so only the short side is
// coloured.
func TestTooSmallFlagsOnlyTheShortSide(t *testing.T) {
	m := modelFor(t, richWorld(), minUsableWidth, minUsableHeight-1)
	content := m.View().Content

	if strings.Contains(content, m.theme.Alert.Render(fmt.Sprintf("%3d", minUsableWidth))) {
		t.Error("a width that clears the floor should not be flagged")
	}
	if !strings.Contains(content, m.theme.Alert.Render(fmt.Sprintf("%3d", minUsableHeight-1))) {
		t.Error("the height that falls short should be flagged")
	}
}

func lineWidth(line string) int {
	return len([]rune(stripANSI(line)))
}

// stripANSI drops escape sequences so a test can measure and match the text
// a reader actually sees.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != 0x1b {
			b.WriteByte(s[i])
			continue
		}
		for i < len(s) && s[i] != 'm' {
			i++
		}
	}
	return b.String()
}

// The box has to be exactly the terminal minus the tab strip on every view
// and at any width: a help line that overflows would push the layout off
// the bottom of the screen.
func TestBoxFitsTheTerminal(t *testing.T) {
	for _, size := range []struct{ width, height int }{{minUsableWidth, minUsableHeight}, {120, 40}, {177, 51}} {
		m := modelFor(t, richWorld(), size.width, size.height)
		m, _ = send(t, m, key("right"))

		for range viewNames {
			content := m.View().Content
			lines := strings.Split(content, "\n")
			if len(lines) != size.height {
				t.Errorf("%s at %dx%d rendered %d rows, want %d",
					m.view, size.width, size.height, len(lines), size.height)
			}
			for i, line := range lines {
				if got := lineWidth(line); got > size.width {
					t.Errorf("%s at %dx%d: row %d is %d columns, want at most %d",
						m.view, size.width, size.height, i, got, size.width)
				}
			}
			m, _ = send(t, m, key("tab"))
		}
	}
}
