package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestMoneyAdd(t *testing.T) {
	sum, err := NewMoney(decimal.NewFromInt(1000), ARS).Add(NewMoney(decimal.NewFromInt(500), ARS))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sum.Equal(NewMoney(decimal.NewFromInt(1500), ARS)) {
		t.Errorf("got %s %s, want 1500 ARS", sum.Amount(), sum.Currency())
	}

	if _, err := NewMoney(decimal.NewFromInt(1000), ARS).Add(NewMoney(decimal.NewFromInt(500), USD)); err == nil {
		t.Error("Add across currencies should return an error")
	}
}

func TestMoneySub(t *testing.T) {
	diff, err := NewMoney(decimal.NewFromInt(1000), ARS).Sub(NewMoney(decimal.NewFromInt(300), ARS))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !diff.Equal(NewMoney(decimal.NewFromInt(700), ARS)) {
		t.Errorf("got %s, want 700", diff.Amount())
	}

	if _, err := NewMoney(decimal.NewFromInt(1000), ARS).Sub(NewMoney(decimal.NewFromInt(300), USD)); err == nil {
		t.Error("Sub across currencies should return an error")
	}
}

func TestMoneyNegate(t *testing.T) {
	if got := NewMoney(decimal.NewFromInt(1000), ARS).Negate(); !got.Equal(NewMoney(decimal.NewFromInt(-1000), ARS)) {
		t.Errorf("Negate() = %s, want -1000", got.Amount())
	}
	if got := NewMoney(decimal.NewFromInt(-1000), ARS).Negate(); !got.Equal(NewMoney(decimal.NewFromInt(1000), ARS)) {
		t.Errorf("Negate() = %s, want 1000", got.Amount())
	}
}

func TestMoneyShare(t *testing.T) {
	tests := []struct {
		name    string
		amount  string
		percent int64
		want    string
	}{
		{"rent 50/50 split", "785000", 50, "392500"},
		{"full share", "442857", 100, "442857"},
		{"zero share", "100000", 0, "0"},
		{"half up on exact .5", "0.05", 50, "0.03"},
		{"rounds down under .5", "0.01", 49, "0"},
		{"rounds up over .5", "0.01", 51, "0.01"},
		{"negative amount half up", "-0.01", 50, "-0.01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amount, err := decimal.NewFromString(tt.amount)
			if err != nil {
				t.Fatalf("bad test amount %q: %v", tt.amount, err)
			}
			want, err := decimal.NewFromString(tt.want)
			if err != nil {
				t.Fatalf("bad test want %q: %v", tt.want, err)
			}

			got := NewMoney(amount, ARS).Share(NewPercent(tt.percent)).Amount()
			if !got.Equal(want) {
				t.Errorf("Money(%s).Share(%d%%) = %s, want %s", tt.amount, tt.percent, got, want)
			}
		})
	}
}

func TestMoneySharePreservesCurrency(t *testing.T) {
	if got := NewMoney(decimal.NewFromInt(1000), USD).Share(NewPercent(50)).Currency(); got != USD {
		t.Errorf("Share() currency = %s, want USD", got)
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
