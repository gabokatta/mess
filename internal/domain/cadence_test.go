package domain

import (
	"testing"
	"time"
)

func TestMonthlyOccursEveryMonth(t *testing.T) {
	for month := time.January; month <= time.December; month++ {
		p := NewPeriod(2026, month)
		if !Monthly.Occurs(p) {
			t.Errorf("Monthly.Occurs(%v) = false, want true", p)
		}
	}
}

func TestAguinaldoOccursOnlyInJuneAndDecember(t *testing.T) {
	tests := []struct {
		month time.Month
		want  bool
	}{
		{time.June, true},
		{time.December, true},
		{time.January, false},
		{time.September, false},
	}

	for _, tt := range tests {
		got := Aguinaldo.Occurs(NewPeriod(2026, tt.month))
		if got != tt.want {
			t.Errorf("Aguinaldo.Occurs(%v) = %v, want %v", tt.month, got, tt.want)
		}
	}
}

func TestMonthsDecodesNewCadence(t *testing.T) {
	c := NewCadence(time.December, time.June)

	got := c.Months()
	want := []time.Month{time.June, time.December}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Months() = %v, want %v (January first, regardless of construction order)", got, want)
	}
}

func TestMonthsOfMonthlyIsAllTwelve(t *testing.T) {
	if got := Monthly.Months(); len(got) != 12 {
		t.Errorf("Monthly.Months() = %v, want all 12 months", got)
	}
}

func TestOneOffCadenceOccursOnlyInItsMonth(t *testing.T) {
	fridge := NewCadence(time.March)

	if !fridge.Occurs(NewPeriod(2026, time.March)) {
		t.Error("one-off cadence should occur in its own month")
	}
	if fridge.Occurs(NewPeriod(2026, time.April)) {
		t.Error("one-off cadence should not occur in other months")
	}
	if !fridge.Occurs(NewPeriod(2027, time.March)) {
		t.Error("Cadence carries no year — pinning a one-off to a single year is active_from/active_until's job, not Occurs'")
	}
}
