package domain

import (
	"testing"
	"time"
)

func TestParsePeriod(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Period
		wantErr bool
	}{
		{"valid", "2026-09", NewPeriod(2026, time.September), false},
		{"single digit month zero padded", "2026-01", NewPeriod(2026, time.January), false},
		{"malformed", "2026/09", Period{}, true},
		{"missing day-less month out of range", "2026-13", Period{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePeriod(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParsePeriod(%q) = nil error, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePeriod(%q) unexpected error: %v", tt.input, err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("ParsePeriod(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestPeriodString(t *testing.T) {
	got := NewPeriod(2026, time.September).String()
	if got != "2026-09" {
		t.Errorf("String() = %q, want %q", got, "2026-09")
	}
}

func TestPeriodOrdering(t *testing.T) {
	jan := NewPeriod(2026, time.January)
	sep := NewPeriod(2026, time.September)
	nextJan := NewPeriod(2027, time.January)

	if !jan.Before(sep) {
		t.Error("2026-01 should be before 2026-09")
	}
	if !sep.After(jan) {
		t.Error("2026-09 should be after 2026-01")
	}
	if !sep.Before(nextJan) {
		t.Error("2026-09 should be before 2027-01")
	}
	if !jan.Equal(NewPeriod(2026, time.January)) {
		t.Error("2026-01 should equal 2026-01")
	}
}

func TestPeriodAddMonths(t *testing.T) {
	tests := []struct {
		name  string
		start Period
		n     int
		want  Period
	}{
		{"same year", NewPeriod(2026, time.September), 1, NewPeriod(2026, time.October)},
		{"year rollover forward", NewPeriod(2026, time.December), 1, NewPeriod(2027, time.January)},
		{"year rollover backward", NewPeriod(2026, time.January), -1, NewPeriod(2025, time.December)},
		{"zero is identity", NewPeriod(2026, time.September), 0, NewPeriod(2026, time.September)},
		{"multi-year jump", NewPeriod(2026, time.September), 16, NewPeriod(2028, time.January)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.start.AddMonths(tt.n)
			if !got.Equal(tt.want) {
				t.Errorf("%v.AddMonths(%d) = %v, want %v", tt.start, tt.n, got, tt.want)
			}
		})
	}
}
