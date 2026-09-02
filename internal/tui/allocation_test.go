package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/huh/v2"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
)

func TestAKeyOpensAllocationFormWithAvailableToSave(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40
	salary := catalog.Concept{Kind: catalog.Income, Money: &catalog.MoneyDetails{Currency: domain.ARS, Share: domain.NewPercent(100)}}
	updated, _ := m.Update(monthLoadedMsg{lines: []month.Line{
		{Concept: salary, Money: &month.LineMoney{Amount: amountFor(t, "1000000"), Confirmed: true}},
	}})
	m = updated.(Model)

	updated, cmd := m.Update(key("a"))
	m = updated.(Model)
	if m.allocationForm == nil {
		t.Fatal("allocationForm = nil, want a form opened")
	}
	m = settle(t, m, cmd)

	content := m.View().Content
	if !strings.Contains(content, "1000000.00") {
		t.Errorf("content = %q, want the available-to-save amount shown", content)
	}
}

// completeAllocationForm mutates the bound values as if the user had, then
// flips the form to StateCompleted directly — see completeSettingsForm.
func completeAllocationForm(m Model, mutate func(*allocationFormValues)) Model {
	mutate(m.allocationForm.values)
	m.allocationForm.form.State = huh.StateCompleted
	return m
}

func TestAllocationFormPercentSplitsAvailableIntoARS(t *testing.T) {
	db := openTestStore(t)
	period := domain.NewPeriod(2026, time.September)
	m := New(db)
	m.width, m.height = 100, 40
	m.period = period
	salary := catalog.Concept{Kind: catalog.Income, Money: &catalog.MoneyDetails{Currency: domain.ARS, Share: domain.NewPercent(100)}}
	updated, _ := m.Update(monthLoadedMsg{lines: []month.Line{
		{Concept: salary, Money: &month.LineMoney{Amount: amountFor(t, "100000"), Confirmed: true}},
	}})
	m = updated.(Model)

	updated, _ = m.Update(key("a"))
	m = updated.(Model)
	m = completeAllocationForm(m, func(v *allocationFormValues) {
		v.investedPercent = "70"
		v.cashPercent = "30"
	})

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	if m.allocationForm != nil {
		t.Fatal("a completed form should close")
	}
	m = settle(t, m, cmd)

	allocations, err := catalog.SavingAllocations(db, period)
	if err != nil {
		t.Fatalf("SavingAllocations() unexpected error: %v", err)
	}
	if len(allocations) != 2 {
		t.Fatalf("SavingAllocations() = %+v, want 2 rows", allocations)
	}
	for _, a := range allocations {
		if a.Currency != domain.ARS {
			t.Errorf("allocation %+v currency = %v, want ARS (percent of the ARS remainder)", a, a.Currency)
		}
		switch a.Destination {
		case catalog.Invested:
			if !a.Amount.Equal(amountFor(t, "70000")) {
				t.Errorf("Invested amount = %s, want 70000 (70%% of 100000)", a.Amount)
			}
		case catalog.Cash:
			if !a.Amount.Equal(amountFor(t, "30000")) {
				t.Errorf("Cash amount = %s, want 30000 (30%% of 100000)", a.Amount)
			}
		}
	}
}

func TestAllocationFormExactFigureInEitherCurrency(t *testing.T) {
	db := openTestStore(t)
	period := domain.NewPeriod(2026, time.September)
	m := New(db)
	m.width, m.height = 100, 40
	m.period = period
	updated, _ := m.Update(monthLoadedMsg{})
	m = updated.(Model)

	updated, _ = m.Update(key("a"))
	m = updated.(Model)
	m = completeAllocationForm(m, func(v *allocationFormValues) {
		v.investedAmount = "500"
		v.investedCurrency = domain.USD
	})

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	m = settle(t, m, cmd)

	allocations, err := catalog.SavingAllocations(db, period)
	if err != nil {
		t.Fatalf("SavingAllocations() unexpected error: %v", err)
	}
	if len(allocations) != 1 || allocations[0].Destination != catalog.Invested {
		t.Fatalf("SavingAllocations() = %+v, want a single Invested row", allocations)
	}
	if !allocations[0].Amount.Equal(amountFor(t, "500")) || allocations[0].Currency != domain.USD {
		t.Errorf("allocation = %+v, want 500 USD", allocations[0])
	}
}

func TestAllocationFormSkipsDestinationsLeftBlank(t *testing.T) {
	db := openTestStore(t)
	period := domain.NewPeriod(2026, time.September)
	m := New(db)
	m.width, m.height = 100, 40
	m.period = period
	updated, _ := m.Update(monthLoadedMsg{})
	m = updated.(Model)

	updated, _ = m.Update(key("a"))
	m = updated.(Model)
	m = completeAllocationForm(m, func(v *allocationFormValues) {
		v.cashAmount = "10000"
	})

	updated, cmd := m.Update(keyEnter())
	m = updated.(Model)
	m = settle(t, m, cmd)

	allocations, err := catalog.SavingAllocations(db, period)
	if err != nil {
		t.Fatalf("SavingAllocations() unexpected error: %v", err)
	}
	if len(allocations) != 1 || allocations[0].Destination != catalog.Cash {
		t.Fatalf("SavingAllocations() = %+v, want only the Cash row", allocations)
	}
}

func TestEscCancelsAllocationFormWithoutWriting(t *testing.T) {
	db := openTestStore(t)
	m := New(db)
	m.width, m.height = 100, 40
	updated, _ := m.Update(monthLoadedMsg{})
	m = updated.(Model)

	updated, _ = m.Update(key("a"))
	m = updated.(Model)
	m.allocationForm.values.cashAmount = "10000"

	updated, cmd := m.Update(keyEsc())
	m = updated.(Model)
	if m.allocationForm != nil {
		t.Error("esc should close the form")
	}
	if cmd != nil {
		t.Error("esc should not write anything")
	}

	allocations, err := catalog.SavingAllocations(db, m.period)
	if err != nil {
		t.Fatalf("SavingAllocations() unexpected error: %v", err)
	}
	if len(allocations) != 0 {
		t.Errorf("SavingAllocations() = %+v, want none after cancel", allocations)
	}
}

func TestMonthViewRendersPocketMoneyBetweenAvailableAndAllocated(t *testing.T) {
	period := domain.NewPeriod(2026, time.September)
	m := New(openTestStore(t))
	m.width, m.height = 100, 40
	m.period = period
	salary := catalog.Concept{Kind: catalog.Income, Money: &catalog.MoneyDetails{Currency: domain.ARS, Share: domain.NewPercent(100)}}
	updated, _ := m.Update(monthLoadedMsg{lines: []month.Line{
		{Concept: salary, Money: &month.LineMoney{Amount: amountFor(t, "100000"), Confirmed: true}},
	}})
	m = updated.(Model)
	updated, _ = m.Update(allocationsLoadedMsg{allocations: []catalog.SavingAllocation{
		{Period: period, Destination: catalog.Cash, Amount: amountFor(t, "40000"), Currency: domain.ARS},
	}})
	m = updated.(Model)

	content := m.View().Content
	for _, want := range []string{"Savings", "pocket money", "100000.00", "40000.00", "60000.00"} {
		if !strings.Contains(content, want) {
			t.Errorf("content missing %q (available/allocated/pocket money):\n%s", want, content)
		}
	}
}

func TestAllocationFormDefaultsCurrencyToUSD(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40

	updated, _ := m.Update(key("a"))
	m = updated.(Model)
	if m.allocationForm == nil {
		t.Fatal("allocationForm = nil, want a form opened")
	}
	if m.allocationForm.values.investedCurrency != domain.USD {
		t.Errorf("investedCurrency = %v, want USD default", m.allocationForm.values.investedCurrency)
	}
	if m.allocationForm.values.cashCurrency != domain.USD {
		t.Errorf("cashCurrency = %v, want USD default", m.allocationForm.values.cashCurrency)
	}
}

func TestSavingsSectionHintsWhenNothingAllocatedYet(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40
	salary := catalog.Concept{Kind: catalog.Income, Money: &catalog.MoneyDetails{Currency: domain.ARS, Share: domain.NewPercent(100)}}
	updated, _ := m.Update(monthLoadedMsg{lines: []month.Line{
		{Concept: salary, Money: &month.LineMoney{Amount: amountFor(t, "3834000"), Confirmed: true}},
	}})
	m = updated.(Model)

	content := m.View().Content
	if !strings.Contains(content, "3834000.00 unallocated") || !strings.Contains(content, "press a") {
		t.Errorf("content = %q, want an unallocated hint naming the amount and the a key", content)
	}
}

func TestSavingsSectionHintGoesAwayOnceAnyAllocationExists(t *testing.T) {
	period := domain.NewPeriod(2026, time.September)
	m := New(openTestStore(t))
	m.width, m.height = 100, 40
	m.period = period
	salary := catalog.Concept{Kind: catalog.Income, Money: &catalog.MoneyDetails{Currency: domain.ARS, Share: domain.NewPercent(100)}}
	updated, _ := m.Update(monthLoadedMsg{lines: []month.Line{
		{Concept: salary, Money: &month.LineMoney{Amount: amountFor(t, "3834000"), Confirmed: true}},
	}})
	m = updated.(Model)
	updated, _ = m.Update(allocationsLoadedMsg{allocations: []catalog.SavingAllocation{
		{Period: period, Destination: catalog.Cash, Amount: amountFor(t, "1000"), Currency: domain.ARS},
	}})
	m = updated.(Model)

	content := m.View().Content
	if strings.Contains(content, "unallocated") {
		t.Errorf("content = %q, want the hint gone once an allocation exists", content)
	}
}

func TestSavingsSectionNoHintWhenNothingAvailable(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40
	updated, _ := m.Update(monthLoadedMsg{})
	m = updated.(Model)

	content := m.View().Content
	if strings.Contains(content, "unallocated") {
		t.Errorf("content = %q, want no hint when there's nothing available to save", content)
	}
}

func TestAllocationFormShowsAvailableInUSDWhenRateKnown(t *testing.T) {
	period := domain.NewPeriod(2026, time.September)
	m := New(openTestStore(t))
	m.width, m.height = 100, 40
	m.period = period
	salary := catalog.Concept{Kind: catalog.Income, Money: &catalog.MoneyDetails{Currency: domain.ARS, Share: domain.NewPercent(100)}}
	updated, _ := m.Update(monthLoadedMsg{lines: []month.Line{
		{Concept: salary, Money: &month.LineMoney{Amount: amountFor(t, "1000"), Confirmed: true}},
	}})
	m = updated.(Model)
	updated, _ = m.Update(allocationsLoadedMsg{rates: []catalog.FxRate{
		{Period: period, Value: amountFor(t, "1000")},
	}})
	m = updated.(Model)

	updated, cmd := m.Update(key("a"))
	m = updated.(Model)
	m = settle(t, m, cmd)
	content := m.allocationForm.form.View()
	if !strings.Contains(content, "1.00 USD") {
		t.Errorf("form content = %q, want available-to-save shown in USD too", content)
	}
}
