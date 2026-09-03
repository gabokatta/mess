package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
	"github.com/gabokatta/mess/internal/rates"
)

// loadedModel is a model with something in every view, the way the app sits
// once its start-up reads have landed.
func loadedModel(t *testing.T, width, height int) Model {
	t.Helper()
	db := testDB(t)

	salary := mustSeed(t, db, "Earnings", "Salary", catalog.Income)
	rent := mustSeed(t, db, "Home", "Rent", catalog.Expense)
	dollars := mustSeed(t, db, "Home", "Dollars", catalog.Saving)
	mustSeed(t, db, "Home", "Wash the house", catalog.Chore)
	mustNote(t, db, catalog.Note{Title: "Ideas", BodyMD: "- [ ] buy a lamp"})

	for id, amount := range map[int64]int64{salary.ID: 2400000, rent.ID: 785000, dollars.ID: 480000} {
		value := decimal.NewFromInt(amount)
		if err := catalog.SetMonthEntryAmount(db, id, september, &value); err != nil {
			t.Fatalf("SetMonthEntryAmount() unexpected error: %v", err)
		}
		if err := catalog.SetMonthEntryDone(db, id, september, true); err != nil {
			t.Fatalf("SetMonthEntryDone() unexpected error: %v", err)
		}
	}

	stored := []catalog.FxRate{
		{Period: domain.NewPeriod(2026, time.July), Value: decimal.NewFromInt(1400), Source: catalog.Close},
		{Period: domain.NewPeriod(2026, time.August), Value: decimal.NewFromInt(1500), Source: catalog.Close},
	}
	quotes := []rates.Quote{{House: domain.Blue, Buy: decimal.NewFromInt(1520), Sell: decimal.NewFromInt(1540)}}

	m := New(db)
	m.today = september
	m.period = september

	loaded, err := month.Load(db, september)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	concepts, _ := catalog.Concepts(db)
	categories, _ := catalog.Categories(db)
	notes, _ := catalog.Notes(db)

	m, _ = send(t, m,
		tea.WindowSizeMsg{Width: width, Height: height},
		ratesMsg{stored: stored, settings: catalog.Settings{FxHouse: domain.Blue}},
		quotesMsg{quotes: quotes},
		monthMsg{lines: loaded.Lines},
		catalogMsg{concepts: concepts, categories: categories},
		notesMsg{notes: notes},
	)

	year, err := month.LoadYear(db, 2026, m.fx())
	if err != nil {
		t.Fatalf("LoadYear() unexpected error: %v", err)
	}
	m, _ = send(t, m, yearMsg{year: year})
	return m
}

// Every view renders at a real terminal size without panicking, and none of
// them spills past the box.
func TestEveryViewRenders(t *testing.T) {
	const width, height = 100, 32
	m := loadedModel(t, width, height)

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

// The header states the month's arithmetic: what came in less what went
// out, what was put away, and the remainder.
func TestMonthHeaderShowsTheArithmetic(t *testing.T) {
	m := loadedModel(t, 100, 32)
	header := stripANSI(m.monthHeader())

	for _, want := range []string{"2026-09", "current", "3 of 4 done", "available 1.615.000", "saved 480.000", "pocket 1.135.000"} {
		if !strings.Contains(header, want) {
			t.Errorf("header is missing %q:\n%s", want, header)
		}
	}
}

// Over-saving is legal and reads as "over by X" rather than being refused.
func TestMonthHeaderReportsOverSaving(t *testing.T) {
	m := testModel(t,
		testLine(1, "Salary", catalog.Income, 100000, true),
		testLine(2, "Dollars", catalog.Saving, 130000, true),
	)
	header := stripANSI(m.monthHeader())

	if !strings.Contains(header, "over by 30.000") {
		t.Errorf("header does not report over-saving:\n%s", header)
	}
}

// A terminal too small for the layout says so instead of garbling.
func TestTooSmallTerminal(t *testing.T) {
	m := loadedModel(t, 30, 8)
	if !strings.Contains(stripANSI(m.View().Content), "make the terminal bigger") {
		t.Error("a tiny terminal should get the grow-your-terminal message")
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
	for _, size := range []struct{ width, height int }{{80, 24}, {100, 32}, {120, 40}} {
		m := loadedModel(t, size.width, size.height)
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
