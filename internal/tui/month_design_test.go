package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
	"github.com/gabokatta/mess/internal/month"
)

func TestMonthColumnHeaderRendersOnceAboveTheList(t *testing.T) {
	m := modelFor(t, richWorld(), minUsableWidth, 32)
	content := stripANSI(m.renderMonth())

	if got := strings.Count(content, "CATEGORY"); got != 1 {
		t.Errorf("CATEGORY appears %d times, want exactly 1", got)
	}
	for _, col := range []string{"CUR", "AMOUNT"} {
		if !strings.Contains(content, col) {
			t.Errorf("column header is missing %q:\n%s", col, content)
		}
	}
	if strings.Contains(content, "CONCEPT") {
		t.Errorf("the name column speaks for itself and needs no label:\n%s", content)
	}
}

func TestKindHeadersCarryNoPaletteHue(t *testing.T) {
	m := modelFor(t, richWorld(), minUsableWidth, 32)

	// A block nobody has ticked carries no subtotal, so the rule runs the
	// full width.
	want := m.theme.Title.Render("INCOME") + " " +
		m.theme.Muted.Render(strings.Repeat("─", tableWidth-len("INCOME")-1))
	if got := m.kindHeader(catalog.Income, decimal.Decimal{}); got != want {
		t.Errorf("kindHeader(Income, 0) = %q, want %q", got, want)
	}

	// With one, the figure takes the label's weight so it reads as part of
	// the header rather than as another row amount. No palette hue anywhere.
	subtotal := decimal.NewFromInt(5480160)
	got := m.kindHeader(catalog.Income, subtotal)
	if lineWidth(got) != tableWidth {
		t.Errorf("kindHeader width = %d, want %d", lineWidth(got), tableWidth)
	}
	if !strings.Contains(got, m.theme.Title.Render(formatAmount(subtotal))) {
		t.Errorf("the subtotal should carry the label's weight:\n%q", got)
	}
	if strings.Contains(got, m.theme.Accent.Render(formatAmount(subtotal))) {
		t.Errorf("the subtotal took Accent, which belongs to the cursor:\n%q", got)
	}
}

func TestCategoryColorCollisionStillReadsByName(t *testing.T) {
	// Nine distinct categories so the ninth wraps onto the first's palette slot.
	concepts := make([]fixture.Concept, 9)
	for i := range concepts {
		concepts[i] = fixture.Concept{
			Name: fmt.Sprintf("C%d", i), Category: fmt.Sprintf("Category%d", i), Kind: catalog.Expense, Base: "100",
		}
	}

	m := modelFor(t, fixture.World{Concepts: concepts}, minUsableWidth, 32)
	first := categoryColor(m.categories, m.categories[0].ID)
	ninth := categoryColor(m.categories, m.categories[8].ID)
	if first != ninth {
		t.Fatalf("expected the 9th category to wrap onto the 1st's palette slot")
	}

	content := stripANSI(m.renderMonth())
	if !strings.Contains(content, "Category0") || !strings.Contains(content, "Category8") {
		t.Errorf("both colliding categories should still be spelled out by name:\n%s", content)
	}
}

func TestLongConceptNameTruncatesInsteadOfWrapping(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{
			{Name: "Refacción Completa del Techo de la Casa", Category: "Home", Kind: catalog.Expense, Base: "159600"},
		},
	}, minUsableWidth, 32)

	row, _ := m.cursorLine()
	styled := m.renderLine(row, false)
	rendered := stripANSI(styled)
	if strings.Contains(rendered, "\n") {
		t.Fatalf("renderLine produced more than one line:\n%q", rendered)
	}
	if !strings.Contains(rendered, "…") {
		t.Errorf("name cell = %q, want it to end in an ellipsis", rendered)
	}
	if !strings.Contains(rendered, "159.600") {
		t.Errorf("amount fell out of its column:\n%q", rendered)
	}
	if got := lineWidth(styled); got != tableWidth {
		t.Errorf("row width = %d, want %d (a truncated name should not desync the columns)", got, tableWidth)
	}
}

func TestRowsSortByCategoryWithinKind(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{
			{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "1000"},
			{Name: "Coffee", Category: "Food", Kind: catalog.Expense, Base: "1000"},
			{Name: "ABL", Category: "Debt", Kind: catalog.Expense, Base: "1000"},
		},
	}, minUsableWidth, 32)

	got := linesOfKind(m.lines, m.categories, catalog.Expense)
	want := []string{"ABL", "Coffee", "Rent"} // Debt, Food, Home
	for i, name := range want {
		if got[i].Concept.Name != name {
			t.Errorf("position %d = %q, want %q (category order)", i, got[i].Concept.Name, name)
		}
	}
}

func TestChoreRowLeavesCurrencyAndAmountBlank(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{{Name: "Wash the house", Category: "Home", Kind: catalog.Chore}},
	}, minUsableWidth, 32)

	row, _ := m.cursorLine()
	rendered := stripANSI(m.renderLine(row, false))
	if strings.Contains(rendered, "ARS") || strings.Contains(rendered, "USD") {
		t.Errorf("a chore row should carry no currency: %q", rendered)
	}
	if got := lineWidth(m.renderLine(row, false)); got != tableWidth {
		t.Errorf("chore row width = %d, want %d (blank cells still hold their column)", got, tableWidth)
	}
}

func TestAmountEditStaysInsideTheAmountColumn(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "785000"}},
	}, minUsableWidth, 32)

	before, _ := m.cursorLine()
	staticRow := stripANSI(m.renderLine(before, false))

	m, _ = send(t, m, key("e"))
	editRow := stripANSI(m.renderLine(before, false))

	prefixWidth := gutterWidth + checkWidth + nameWidth + colGap + categoryWidth + colGap + currencyWidth + colGap
	if got, want := editRow[:prefixWidth], staticRow[:prefixWidth]; got != want {
		t.Errorf("editing moved the category/currency cells: got %q, want %q", got, want)
	}
	if got := lineWidth(m.renderLine(before, false)); got != tableWidth {
		t.Errorf("edit row width = %d, want %d", got, tableWidth)
	}
}

func TestRailShowsARSAndItsUSDConversion(t *testing.T) {
	m := modelFor(t, richWorld(), minUsableWidth, 32)
	rate := m.fx().At(m.period)
	totals := month.ResolveTotals(m.lines, rate)
	content := stripANSI(m.renderRail(totals, rate))

	for _, want := range []string{
		"available", formatAmount(totals.Available.Amount()), formatAmount(totals.AvailableUSD(rate)),
		"saved", formatAmount(totals.Saved.Amount()), formatAmount(totals.SavedUSD(rate)),
		"pocket", formatAmount(totals.Pocket.Amount()), formatAmount(totals.PocketUSD(rate)),
	} {
		if !strings.Contains(content, want) {
			t.Errorf("rail is missing %q:\n%s", want, content)
		}
	}
}

func TestRailWithNoRateRendersAnEmDash(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{{Name: "Salary", Category: "Earnings", Kind: catalog.Income, Base: "100000"}},
	}, minUsableWidth, 32)
	m.today = domain.NewPeriod(2030, 1) // far from the world's period, and unrated

	rate := m.fx().At(m.period)
	if rate.OK() {
		t.Fatalf("expected no rate for this period")
	}
	totals := month.ResolveTotals(m.lines, rate)
	spec := m.totalsBox("available", totals.Available.Amount(), totals.AvailableUSD(rate), rate.OK(), shortfall{})
	box := m.renderBox(spec, railMinInterior)

	if !strings.Contains(stripANSI(box), "—") {
		t.Errorf("box without a rate should render an em dash:\n%s", box)
	}
	if got := lipgloss.Height(box); got != 4 {
		t.Errorf("box has %d lines, want 4 regardless of rate availability", got)
	}
}

func TestOverspentPocketReadsAsAShortfall(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{
			{Name: "Salary", Category: "Earnings", Kind: catalog.Income, Base: "100000"},
			{Name: "Rent", Category: "Home", Kind: catalog.Expense, Base: "180000"},
		},
		Entries: []fixture.Entry{
			{Concept: "Salary", Period: fixture.Period, Amount: "100000", Done: true},
			{Concept: "Rent", Period: fixture.Period, Amount: "180000", Done: true},
		},
	}, minUsableWidth, 32)

	totals := month.ResolveTotals(m.lines, m.fx().At(m.period))
	if !totals.Overspent() {
		t.Fatalf("expected this fixture to overspend; available = %s", totals.Available.Amount())
	}

	got := negativeShortfall(totals.Pocket.Amount(), totals.Overspent())
	if got.label != "short by" || !got.alert {
		t.Errorf("negativeShortfall = %+v, want a short-by that alerts", got)
	}
}

func TestPocketOverSavedIsNamedWithoutAlert(t *testing.T) {
	m := modelFor(t, fixture.World{
		Concepts: []fixture.Concept{
			{Name: "Salary", Category: "Earnings", Kind: catalog.Income, Base: "100000"},
			{Name: "Dollars", Category: "Home", Kind: catalog.Saving, Base: "130000"},
		},
		Entries: []fixture.Entry{
			{Concept: "Salary", Period: fixture.Period, Amount: "100000", Done: true},
			{Concept: "Dollars", Period: fixture.Period, Amount: "130000", Done: true},
		},
	}, minUsableWidth, 32)

	rate := m.fx().At(m.period)
	totals := month.ResolveTotals(m.lines, rate)
	if !totals.Pocket.Amount().IsNegative() {
		t.Fatalf("expected pocket to be negative in this fixture")
	}

	if totals.Overspent() {
		t.Fatal("this fixture over-saves; it must not read as overspending")
	}
	short := negativeShortfall(totals.Pocket.Amount(), totals.Overspent())
	if short.alert {
		t.Errorf("over-saving alerted; only overspending should")
	}
	spec := m.totalsBox("pocket", totals.Pocket.Amount(), totals.PocketUSD(rate), rate.OK(), short)
	box := m.renderBox(spec, railMinInterior)
	plain := stripANSI(box)
	if !strings.Contains(plain, "over by") || !strings.Contains(plain, formatAmount(totals.Pocket.Amount().Abs())) {
		t.Errorf("over-saved pocket should say so:\n%s", plain)
	}

	over := railLine{"over by", formatAmount(totals.Pocket.Amount().Abs()), m.theme.Bright}
	if !strings.Contains(box, railField(over, railMinInterior)) {
		t.Errorf("the over-by line should stay bright:\n%s", box)
	}
	if strings.Contains(box, m.theme.Alert.Render("over by")) {
		t.Errorf("over-saving took Alert, which belongs to overspending:\n%s", box)
	}
}

func TestMonthMetaShowsTheDoneCount(t *testing.T) {
	m := modelFor(t, richWorld(), minUsableWidth, 32)
	totals := month.ResolveTotals(m.lines, m.fx().At(m.period))
	content := stripANSI(m.monthMeta(totals))

	done, total := month.DoneCount(m.lines)
	if !strings.Contains(content, fmt.Sprintf("done  %d / %d", done, total)) {
		t.Errorf("meta is missing the done count:\n%s", content)
	}
}

func TestExcludedCountOnlyShowsWhenNonZero(t *testing.T) {
	clean := month.Totals{}
	m := modelFor(t, richWorld(), minUsableWidth, 32)
	if strings.Contains(m.monthMeta(clean), "no rate") {
		t.Error("a month with nothing excluded should carry no warning")
	}

	dirty := month.Totals{Excluded: 2}
	if !strings.Contains(stripANSI(m.monthMeta(dirty)), "2 left out, no rate") {
		t.Error("a month with excluded lines should say how many")
	}
}

func TestMonthTitleNamesTheMonthInFull(t *testing.T) {
	m := modelFor(t, richWorld(), minUsableWidth, 32)
	header := stripANSI(m.periodHeading())
	if !strings.Contains(header, "SEPTEMBER 2026") {
		t.Errorf("header = %q, want the month spelled out", header)
	}
	if !strings.Contains(header, "current") {
		t.Errorf("header = %q, want the period status beside the title", header)
	}
}

func TestMonthTitleIsUnderlined(t *testing.T) {
	m := modelFor(t, richWorld(), minUsableWidth, 32)
	want := m.theme.Title.Underline(true).Render("SEPTEMBER 2026")
	if !strings.Contains(m.periodHeading(), want) {
		t.Errorf("periodHeading() = %q, want the title underlined", m.periodHeading())
	}
}

func TestSidebarDoesNotPushShortRowsAwayFromTheHeader(t *testing.T) {
	rows := "A\nB"
	sidebar := "W\nX\nY\nZ\nV"
	composed := stripANSI(joinRowsAndSidebar(rows, sidebar))

	lines := strings.Split(composed, "\n")
	if !strings.HasPrefix(lines[0], "A") {
		t.Fatalf("composed[0] = %q, want the rows' first line, unpadded", lines[0])
	}
	if !strings.HasPrefix(lines[1], "B") {
		t.Fatalf("composed[1] = %q, want the rows' second line, unpadded", lines[1])
	}
}

func TestSidebarCentersAgainstTallerRows(t *testing.T) {
	rows := "1\n2\n3\n4\n5"
	sidebar := "X\nY\nZ"
	composed := stripANSI(joinRowsAndSidebar(rows, sidebar))

	lines := strings.Split(composed, "\n")
	if len(lines) != 5 {
		t.Fatalf("composed has %d lines, want 5 (the taller side, rows)", len(lines))
	}
	if strings.Contains(lines[0], "X") {
		t.Errorf("sidebar's first line landed on row 0; want it centered, not flush top:\n%s", composed)
	}
	if !strings.Contains(lines[1], "X") {
		t.Errorf("sidebar should center within the rows' height:\n%s", composed)
	}
}

// A world whose figures run to eight digits, plus enough filler concepts to
// overrun any terminal the tests use.
func bigFigureWorld(filler int) fixture.World {
	w := fixture.World{
		Concepts: []fixture.Concept{
			{Name: "Salary", Category: "Earnings", Kind: catalog.Income, Base: "3136000"},
			{Name: "Credit Card", Category: "Debt", Kind: catalog.Expense, Base: "45678901.50"},
		},
		Entries: []fixture.Entry{
			{Concept: "Salary", Period: fixture.Period, Amount: "3136000", Done: true},
			{Concept: "Credit Card", Period: fixture.Period, Amount: "45678901.50", Done: true},
		},
	}
	for i := range filler {
		w.Concepts = append(w.Concepts, fixture.Concept{
			Name: fmt.Sprintf("Filler %02d", i), Category: "Utilities", Kind: catalog.Expense, Base: "1000",
		})
	}
	return w
}

func TestRailBoxesGrowToHoldBigFigures(t *testing.T) {
	m := modelFor(t, bigFigureWorld(0), 120, 40)
	rate := m.fx().At(m.period)
	totals := month.ResolveTotals(m.lines, rate)
	rail := stripANSI(m.renderRail(totals, rate))

	width := 0
	for _, line := range strings.Split(rail, "\n") {
		if line == "" {
			continue
		}
		if width == 0 {
			width = lipgloss.Width(line)
		}
		if got := lipgloss.Width(line); got != width {
			t.Fatalf("box line is %d columns, want %d — the boxes must share one width:\n%s", got, width, rail)
		}
	}
	if width <= railMinInterior+2 {
		t.Errorf("rail is %d columns, want it grown past the %d floor", width, railMinInterior+2)
	}
	if !strings.Contains(rail, formatAmount(totals.Available.Amount())) {
		t.Errorf("the available figure should sit intact inside its box:\n%s", rail)
	}
}

func TestRailBoxesKeepTheirFloorForSmallFigures(t *testing.T) {
	m := modelFor(t, richWorld(), minUsableWidth, 32)
	rate := m.fx().At(m.period)
	rail := stripANSI(m.renderRail(month.ResolveTotals(m.lines, rate), rate))

	if got := lipgloss.Width(strings.Split(rail, "\n")[0]); got != railMinInterior+2 {
		t.Errorf("rail is %d columns, want the %d floor", got, railMinInterior+2)
	}
}

func TestScrollHintCountsWhatIsOffScreen(t *testing.T) {
	m := modelFor(t, bigFigureWorld(60), minUsableWidth, minUsableHeight)
	hint := stripANSI(m.scrollHint(m.monthList, gutterWidth))

	if !strings.Contains(hint, "↓") || !strings.Contains(hint, "more") {
		t.Errorf("hint = %q, want it to say how many rows are below", hint)
	}
}

func TestNoScrollHintWhenTheMonthFits(t *testing.T) {
	m := modelFor(t, richWorld(), minUsableWidth, 44)
	if got := m.scrollHint(m.monthList, gutterWidth); got != "" {
		t.Errorf("hint = %q, want none when the whole month is on screen", got)
	}
}

func TestShortMonthCentersItsCard(t *testing.T) {
	roomy := modelFor(t, richWorld(), minUsableWidth, 44)
	if blankLinesAbove(roomy.renderMonth()) == 0 {
		t.Error("a card with room to spare should not be pinned to the top")
	}

	full := modelFor(t, bigFigureWorld(60), minUsableWidth, minUsableHeight)
	if got := blankLinesAbove(full.renderMonth()); got != 0 {
		t.Errorf("a card that fills the screen has %d blank lines above it, want 0", got)
	}
}

func blankLinesAbove(rendered string) int {
	for i, line := range strings.Split(rendered, "\n") {
		if strings.TrimSpace(stripANSI(line)) != "" {
			return i
		}
	}
	return 0
}
