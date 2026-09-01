package domain

import "testing"

func TestFxHouseString(t *testing.T) {
	tests := []struct {
		house FxHouse
		want  string
	}{
		{Blue, "Blue"},
		{Official, "Official"},
		{MEP, "MEP"},
	}
	for _, tt := range tests {
		if got := tt.house.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}

func TestParseFxHouse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    FxHouse
		wantErr bool
	}{
		{"blue", "Blue", Blue, false},
		{"official", "Official", Official, false},
		{"mep", "MEP", MEP, false},
		{"unknown", "Cripto", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFxHouse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseFxHouse(%q) = nil error, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFxHouse(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseFxHouse(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
