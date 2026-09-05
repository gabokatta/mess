package domain

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/shopspring/decimal"
)

func TestMoneyToARS(t *testing.T) {
	rate := decimal.NewFromInt(1500)

	pesos, ok := NewMoney(decimal.NewFromInt(1000), ARS).ToARS(rate, true)
	if !ok {
		t.Fatal("ARS ToARS() reported false, want true")
	}
	if diff := cmp.Diff(NewMoney(decimal.NewFromInt(1000), ARS), pesos); diff != "" {
		t.Errorf("ARS ToARS() mismatch (-want +got):\n%s", diff)
	}

	converted, ok := NewMoney(decimal.NewFromInt(400), USD).ToARS(rate, true)
	if !ok {
		t.Fatal("USD ToARS() reported false, want true")
	}
	if diff := cmp.Diff(NewMoney(decimal.NewFromInt(600000), ARS), converted); diff != "" {
		t.Errorf("USD ToARS() mismatch (-want +got):\n%s", diff)
	}
}

func TestMoneyToARSWithoutRateIsDropped(t *testing.T) {
	if _, ok := NewMoney(decimal.NewFromInt(400), USD).ToARS(decimal.Decimal{}, false); ok {
		t.Error("USD ToARS() without a rate should report false")
	}

	pesos, ok := NewMoney(decimal.NewFromInt(400), ARS).ToARS(decimal.Decimal{}, false)
	if !ok {
		t.Fatal("ARS ToARS() should not need a rate")
	}
	if !pesos.Amount().Equal(decimal.NewFromInt(400)) {
		t.Errorf("ARS ToARS() = %s, want 400 unchanged", pesos.Amount())
	}
}

func TestParseCurrency(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Currency
		wantErr bool
	}{
		{"ars", "ARS", ARS, false},
		{"usd", "USD", USD, false},
		{"unknown", "EUR", 0, true},
		{"lowercase not accepted", "ars", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCurrency(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseCurrency(%q) = nil error, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseCurrency(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseCurrency(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
