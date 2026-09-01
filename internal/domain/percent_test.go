package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestNewPercent(t *testing.T) {
	tests := []struct {
		name  string
		whole int64
		want  string
	}{
		{"fifty", 50, "0.5"},
		{"hundred", 100, "1"},
		{"zero", 0, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, err := decimal.NewFromString(tt.want)
			if err != nil {
				t.Fatalf("bad test want %q: %v", tt.want, err)
			}
			if got := NewPercent(tt.whole).Fraction(); !got.Equal(want) {
				t.Errorf("NewPercent(%d).Fraction() = %s, want %s", tt.whole, got, want)
			}
		})
	}
}

func TestNewPercentFromFraction(t *testing.T) {
	got := NewPercentFromFraction(decimal.RequireFromString("0.5")).Fraction()
	if !got.Equal(NewPercent(50).Fraction()) {
		t.Errorf("NewPercentFromFraction(0.5).Fraction() = %s, want %s", got, NewPercent(50).Fraction())
	}
}
