package fixture

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

// demoMonths is how many months of history Demo builds: past settled, the
// anchor month in progress, future untouched by any override. That is short
// of two full calendar years, so paging the Year view back one year renders
// full and the anchor's own year renders partial in the same run.
const demoMonths = 18

// demoDrift is how much a confirmed amount grows per month walking from the
// oldest month in the window toward the anchor, so the Year charts show a
// slope instead of a wall of identical bars.
var demoDrift = decimal.NewFromFloat(0.04)

// demoLine is one concept plus how Demo confirms it. entries writes a
// drifted amount for every period the concept occurs in; done additionally
// ticks it off, but only in the anchor month, so the anchor mixes
// confirmed, unconfirmed and done rows the way a real month does.
type demoLine struct {
	concept Concept
	entries bool
	done    bool
}

// Demo is the world make seed writes: every case that breaks a layout by
// accident, carried on purpose, so it shows up every session instead of by
// chance. Every period in it is anchor plus an offset, so pinning anchor
// with --period reproduces the same database byte for byte.
func Demo(anchor domain.Period) World {
	oldest := anchor.AddMonths(-(demoMonths - 1))
	lines := demoLines(anchor, oldest)

	concepts := make([]Concept, len(lines))
	for i, l := range lines {
		concepts[i] = l.concept
	}

	lastExport := demoLastExport(anchor)
	return World{
		Concepts:   concepts,
		Entries:    demoEntries(anchor, oldest, lines),
		Notes:      demoNotes(anchor),
		Rates:      demoRates(anchor, oldest),
		FxHouse:    domain.Blue,
		LastExport: &lastExport,
	}
}

// demoLastExport is a few days before the anchor month starts, so the Month
// status line's exported segment renders as a backup taken just before the
// data on screen.
func demoLastExport(anchor domain.Period) time.Time {
	return time.Date(anchor.Year(), anchor.Month(), 1, 9, 0, 0, 0, time.UTC).AddDate(0, 0, -3)
}

// demoLines is the roster: English names but for two deliberate exceptions —
// one concept name past the 26 character column that also carries the one
// accented rune among concept names, and the emoji in a note title alongside
// it. Category order here is the order Load creates them in, which is the
// order the Month view groups by and the Year chart colours by — a
// decision, not an accident.
func demoLines(anchor, oldest domain.Period) []demoLine {
	endedEarly := anchor.AddMonths(-3)

	return []demoLine{
		// Earnings
		{entries: true, concept: Concept{
			Name: "Salary", Category: "Earnings", Kind: catalog.Income,
			Currency: domain.ARS, Base: "2800000", From: oldest,
		}},
		// Confirmed every month, including the oldest one with no stored fx
		// rate: the case that makes the header's Excluded non-zero.
		{entries: true, concept: Concept{
			Name: "Freelance Income", Category: "Earnings", Kind: catalog.Income,
			Currency: domain.USD, Base: "300", From: oldest,
		}},

		// Home
		{entries: true, concept: Concept{
			Name: "Rent", Category: "Home", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "850000", From: oldest,
		}},
		{entries: true, concept: Concept{
			Name: "Building Fees", Category: "Home", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "210000", From: oldest,
		}},
		// Past the 26 character name column, with the one accented rune
		// Demo carries in a concept name.
		{entries: true, concept: Concept{
			Name: "Refacción Completa del Techo", Category: "Home", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "95000", From: oldest,
		}},
		{entries: true, concept: Concept{
			Name: "Home Insurance", Category: "Home", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "38000", From: oldest,
		}},

		// Utilities
		{entries: true, concept: Concept{
			Name: "Electricity", Category: "Utilities", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "55000", From: oldest,
		}},
		{entries: true, concept: Concept{
			Name: "Gas", Category: "Utilities", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "32000", From: oldest,
		}},
		{entries: true, concept: Concept{
			Name: "Internet", Category: "Utilities", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "28000", From: oldest,
		}},
		{entries: true, concept: Concept{
			Name: "Mobile Phone", Category: "Utilities", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "22000", From: oldest,
		}},
		// Never confirmed, so the anchor month always carries at least one
		// unconfirmed row beside the confirmed and done ones.
		{entries: false, concept: Concept{
			Name: "Water", Category: "Utilities", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "15000", From: oldest,
		}},

		// Subscriptions
		{entries: true, concept: Concept{
			Name: "Streaming Bundle", Category: "Subscriptions", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "18000", From: oldest,
		}},
		{entries: true, concept: Concept{
			Name: "Gym", Category: "Subscriptions", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "42000", From: oldest,
		}},
		{entries: true, concept: Concept{
			Name: "Spotify", Category: "Subscriptions", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "9000", From: oldest,
		}},
		{entries: true, concept: Concept{
			Name: "Cloud Storage", Category: "Subscriptions", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "6000", From: oldest,
		}},
		// Only June and December: the cadence gap that shows up arrowing
		// across months.
		{entries: true, concept: Concept{
			Name: "School Semester Fee", Category: "Subscriptions", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "150000", Months: domain.Aguinaldo, From: oldest,
		}},

		// Transport
		{entries: true, done: true, concept: Concept{
			Name: "Fuel", Category: "Transport", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "65000", From: oldest,
		}},
		{entries: true, concept: Concept{
			Name: "Car Insurance", Category: "Transport", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "48000", From: oldest,
		}},
		{entries: true, concept: Concept{
			Name: "Car Maintenance", Category: "Transport", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "30000", From: oldest,
		}},

		// Food
		{entries: true, concept: Concept{
			Name: "Groceries", Category: "Food", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "220000", From: oldest,
		}},
		{entries: true, concept: Concept{
			Name: "Delivery", Category: "Food", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "45000", From: oldest,
		}},
		{entries: true, concept: Concept{
			Name: "Produce", Category: "Food", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "35000", From: oldest,
		}},

		// Health
		{entries: true, concept: Concept{
			Name: "Health Insurance", Category: "Health", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "60000", From: oldest,
		}},
		{entries: true, done: true, concept: Concept{
			Name: "Pharmacy", Category: "Health", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "25000", From: oldest,
		}},

		// Debt
		{entries: true, concept: Concept{
			Name: "Credit Card", Category: "Debt", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "180000", From: oldest,
		}},
		// Active range ends three months before the anchor: the concept
		// the active-range filter is there to hide.
		{entries: true, concept: Concept{
			Name: "Personal Loan", Category: "Debt", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "120000", From: oldest, Until: endedEarly,
		}},

		// Savings — three, so the saved-per-month chart stacks.
		{entries: true, concept: Concept{
			Name: "Dollar Savings", Category: "Savings", Kind: catalog.Saving,
			Currency: domain.USD, Base: "200", From: oldest,
		}},
		{entries: true, concept: Concept{
			Name: "Fixed-Term Deposit", Category: "Savings", Kind: catalog.Saving,
			Currency: domain.ARS, Base: "300000", From: oldest,
		}},
		{entries: true, concept: Concept{
			Name: "Emergency Fund", Category: "Savings", Kind: catalog.Saving,
			Currency: domain.ARS, Base: "150000", From: oldest,
		}},
	}
}

// demoEntries writes one drifted, confirmed amount per month for every line
// that wants entries, skipping a period a concept does not occur in, plus
// two deliberate one-off overrides: bigPurchaseAt replaces one month's
// Credit Card line with an eight digit figure with cents, the amount
// column's stress case; overSavedAt replaces one month's Fixed-Term Deposit
// with a spike big enough on its own to outweigh that month's Available, so
// Pocket renders negative for the reason the story asks for — over-saving,
// not an unrelated expense.
func demoEntries(anchor, oldest domain.Period, lines []demoLine) []Entry {
	bigPurchaseAt := anchor.AddMonths(-14)
	overSavedAt := anchor.AddMonths(-10)

	var entries []Entry
	for i := 0; i < demoMonths; i++ {
		period := oldest.AddMonths(i)
		for _, l := range lines {
			if !l.entries || !occurs(l.concept, period) {
				continue
			}
			amount := driftedAmount(l.concept.Base, i)
			switch {
			case l.concept.Name == "Credit Card" && period.Equal(bigPurchaseAt):
				amount = "45678901.50"
			case l.concept.Name == "Fixed-Term Deposit" && period.Equal(overSavedAt):
				amount = "5000000"
			}
			entries = append(entries, Entry{
				Concept: l.concept.Name,
				Period:  period,
				Amount:  amount,
				Done:    l.done && period.Equal(anchor),
			})
		}
	}
	return entries
}

// occurs mirrors month.Resolve's own cadence-and-range check, ahead of Load
// applying it for real, so Demo only ever writes an entry for a period a
// line will actually surface in.
func occurs(c Concept, p domain.Period) bool {
	months := c.Months
	if months == 0 {
		months = domain.Monthly
	}
	if !months.Occurs(p) {
		return false
	}
	if p.Before(c.From) {
		return false
	}
	return c.Until.IsZero() || !p.After(c.Until)
}

// driftedAmount is base grown by demoDrift for every month between the
// window's oldest and i, so the amount at the anchor is the amount at the
// oldest month plus eighteen months of drift.
func driftedAmount(base string, i int) string {
	b := decimal.RequireFromString(base)
	factor := decimal.NewFromInt(1).Add(demoDrift.Mul(decimal.NewFromInt(int64(i))))
	return b.Mul(factor).StringFixed(2)
}

// demoRates leaves the oldest month in the window without a stored close —
// the gap the backfill path exists for, and, paired with Freelance
// Income's confirmed USD line that month, the case that makes Excluded
// non-zero. The anchor is left unstored too, same as a real month in
// progress: its rate comes from a live quote, not a close. One rate in the
// middle is Manual, so the Rates view shows both origins.
func demoRates(anchor, oldest domain.Period) []Rate {
	manualAt := anchor.AddMonths(-8)

	var rates []Rate
	for i := 1; i < demoMonths-1; i++ {
		period := oldest.AddMonths(i)
		source := catalog.Close
		if period.Equal(manualAt) {
			source = catalog.Manual
		}
		rates = append(rates, Rate{
			Period: period,
			Value:  decimal.NewFromInt(1200 + 20*int64(i)).String(),
			Source: source,
		})
	}
	return rates
}

const longDemoNote = `# Goals for the year

- Keep the emergency fund above 6 months of expenses.
- Cut delivery spending by 20%.
- Review subscriptions nobody uses.

## Savings

1. Dollars: contribute every month, no exceptions.
2. Fixed-term deposit: renew automatically.

> Consistency matters more than the amount.
`

// demoNotes covers every note state the UI renders: pinned, scoped to one
// period, long markdown, and done. The pinned note's title carries Demo's
// other deliberate non-ASCII rune, an emoji.
func demoNotes(anchor domain.Period) []catalog.Note {
	return []catalog.Note{
		{
			Title:  "📌 General Reminder",
			BodyMD: "Review fixed costs before the month closes.",
		},
		{
			Title:  "Close out " + anchor.String(),
			BodyMD: "Confirm the pending lines before rolling over.",
			Period: anchor,
		},
		{
			Title:  "Annual Financial Plan",
			BodyMD: longDemoNote,
			Period: anchor,
		},
		{
			Title:  "Renew car insurance",
			BodyMD: "Due at year end, get quotes ahead of time.",
			Period: anchor,
			Done:   true,
		},
	}
}
