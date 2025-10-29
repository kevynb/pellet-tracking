package core

import "testing"

func TestFormatMoney(t *testing.T) {
	tests := []struct {
		name  string
		input Money
		want  string
	}{
		{"zero", 0, "0,00 €"},
		{"euros", Money(12345), "123,45 €"},
		{"thousands", Money(1234567), "12 345,67 €"},
		{"negative", Money(-999), "-9,99 €"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatMoney(tt.input); got != tt.want {
				t.Fatalf("FormatMoney(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatFR(t *testing.T) {
	if got := FormatFR(1234.56); got != "1 234,56 €" {
		t.Fatalf("FormatFR() = %q", got)
	}
}

func TestRoundBancaire(t *testing.T) {
	cases := []struct {
		value float64
		want  float64
	}{
		{1.235, 1.24},
		{1.225, 1.22},
		{-2.555, -2.56},
	}
	for _, c := range cases {
		if got := RoundBancaire(c.value); got != c.want {
			t.Fatalf("RoundBancaire(%v) = %v, want %v", c.value, got, c.want)
		}
	}
}
