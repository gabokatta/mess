package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestMoneyToARS(t *testing.T) {
	rate := decimal.NewFromInt(1500)

	pesos, ok := NewMoney(decimal.NewFromInt(1000), ARS).ToARS(rate, true)
	if !ok || !pesos.Equal(NewMoney(decimal.NewFromInt(1000), ARS)) {
		t.Errorf("ARS ToARS() = %s, %v; want 1000 ARS, true", pesos.Amount(), ok)
	}

	converted, ok := NewMoney(decimal.NewFromInt(400), USD).ToARS(rate, true)
	if !ok || !converted.Equal(NewMoney(decimal.NewFromInt(600000), ARS)) {
		t.Errorf("USD ToARS() = %s, %v; want 600000 ARS, true", converted.Amount(), ok)
	}
}

// A USD line with no rate is dropped, never converted at zero — counting it
// as nothing would silently understate the month.
func TestMoneyToARSWithoutRateIsDropped(t *testing.T) {
	if _, ok := NewMoney(decimal.NewFromInt(400), USD).ToARS(decimal.Decimal{}, false); ok {
		t.Error("USD ToARS() without a rate should report false")
	}
	if pesos, ok := NewMoney(decimal.NewFromInt(400), ARS).ToARS(decimal.Decimal{}, false); !ok || !pesos.Amount().Equal(decimal.NewFromInt(400)) {
		t.Error("ARS ToARS() should not need a rate")
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
