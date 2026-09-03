package fixture

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

const demoMonths = 18

var demoDrift = decimal.NewFromFloat(0.04)

type demoLine struct {
	concept Concept
	entries bool
	done    bool
}

// Demo is the world `make seed` writes, carrying every layout stress case on
// purpose. Every period is anchor plus an offset, so it is reproducible.
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

func demoLastExport(anchor domain.Period) time.Time {
	return time.Date(anchor.Year(), anchor.Month(), 1, 9, 0, 0, 0, time.UTC).AddDate(0, 0, -3)
}

func demoLines(anchor, oldest domain.Period) []demoLine {
	endedEarly := anchor.AddMonths(-3)

	return []demoLine{
		{entries: true, concept: Concept{
			Name: "Salary", Category: "Earnings", Kind: catalog.Income,
			Currency: domain.ARS, Base: "2800000", From: oldest,
		}},
		{entries: true, concept: Concept{
			Name: "Freelance Income", Category: "Earnings", Kind: catalog.Income,
			Currency: domain.USD, Base: "300", From: oldest,
		}},

		{entries: true, concept: Concept{
			Name: "Rent", Category: "Home", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "850000", From: oldest,
		}},
		{entries: true, concept: Concept{
			Name: "Building Fees", Category: "Home", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "210000", From: oldest,
		}},
		{entries: true, concept: Concept{
			Name: "Refacción Completa del Techo", Category: "Home", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "95000", From: oldest,
		}},
		{entries: true, concept: Concept{
			Name: "Home Insurance", Category: "Home", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "38000", From: oldest,
		}},

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
		{entries: false, concept: Concept{
			Name: "Water", Category: "Utilities", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "15000", From: oldest,
		}},

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
		{entries: true, concept: Concept{
			Name: "School Semester Fee", Category: "Subscriptions", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "150000", Months: domain.Aguinaldo, From: oldest,
		}},

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

		{entries: true, concept: Concept{
			Name: "Health Insurance", Category: "Health", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "60000", From: oldest,
		}},
		{entries: true, done: true, concept: Concept{
			Name: "Pharmacy", Category: "Health", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "25000", From: oldest,
		}},

		{entries: true, concept: Concept{
			Name: "Credit Card", Category: "Debt", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "180000", From: oldest,
		}},
		{entries: true, concept: Concept{
			Name: "Personal Loan", Category: "Debt", Kind: catalog.Expense,
			Currency: domain.ARS, Base: "120000", From: oldest, Until: endedEarly,
		}},

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

func driftedAmount(base string, i int) string {
	b := decimal.RequireFromString(base)
	factor := decimal.NewFromInt(1).Add(demoDrift.Mul(decimal.NewFromInt(int64(i))))
	return b.Mul(factor).StringFixed(2)
}

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
