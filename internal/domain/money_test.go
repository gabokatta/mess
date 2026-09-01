package domain

import "testing"

func TestMoneyAdd(t *testing.T) {
	sum, err := NewMoney(1000, ARS).Add(NewMoney(500, ARS))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sum.Amount() != 1500 || sum.Currency() != ARS {
		t.Errorf("got %d %s, want 1500 ARS", sum.Amount(), sum.Currency())
	}

	if _, err := NewMoney(1000, ARS).Add(NewMoney(500, USD)); err == nil {
		t.Error("Add across currencies should return an error")
	}
}

func TestMoneySub(t *testing.T) {
	diff, err := NewMoney(1000, ARS).Sub(NewMoney(300, ARS))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.Amount() != 700 {
		t.Errorf("got %d, want 700", diff.Amount())
	}

	if _, err := NewMoney(1000, ARS).Sub(NewMoney(300, USD)); err == nil {
		t.Error("Sub across currencies should return an error")
	}
}

func TestMoneyNegate(t *testing.T) {
	if got := NewMoney(1000, ARS).Negate().Amount(); got != -1000 {
		t.Errorf("Negate() = %d, want -1000", got)
	}
	if got := NewMoney(-1000, ARS).Negate().Amount(); got != 1000 {
		t.Errorf("Negate() = %d, want 1000", got)
	}
}

func TestMoneyShare(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		bp     int64
		want   int64
	}{
		{"rent 50/50 split", 785000, 5000, 392500},
		{"full share", 442857, 10000, 442857},
		{"zero share", 100000, 0, 0},
		{"half up on exact .5", 1, 5000, 1},
		{"rounds down under .5", 1, 4999, 0},
		{"rounds up over .5", 1, 5001, 1},
		{"negative amount half up", -1, 5000, -1},
		{"repeating fraction rounds to nearest", 3, 3333, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewMoney(tt.amount, ARS).Share(tt.bp).Amount()
			if got != tt.want {
				t.Errorf("Money(%d).Share(%d) = %d, want %d", tt.amount, tt.bp, got, tt.want)
			}
		})
	}
}

func TestMoneySharePreservesCurrency(t *testing.T) {
	if got := NewMoney(1000, USD).Share(5000).Currency(); got != USD {
		t.Errorf("Share() currency = %s, want USD", got)
	}
}
