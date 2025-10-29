package core

import (
	"fmt"
	"strings"
)

// RoundBancaire performs bankers rounding (round half to even).
func RoundBancaire(value float64) float64 {
	return roundHalfEven(value*100) / 100
}

// FormatFR formats a euro amount using French thousands and decimal separators.
func FormatFR(amount float64) string {
	cents := int64(roundHalfEven(amount * 100))
	return formatCents(cents)
}

// FormatMoney renders the provided Money amount in French locale with the euro symbol.
func FormatMoney(m Money) string {
	return formatCents(m.Int64())
}

func formatCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	euros := cents / 100
	remainder := cents % 100
	eurosStr := formatThousands(euros)
	return fmt.Sprintf("%s%s,%02d €", sign, eurosStr, remainder)
}

func formatThousands(value int64) string {
	s := fmt.Sprintf("%d", value)
	if len(s) <= 3 {
		return s
	}
	var builder strings.Builder
	prefix := len(s) % 3
	if prefix == 0 {
		prefix = 3
	}
	builder.WriteString(s[:prefix])
	for i := prefix; i < len(s); i += 3 {
		builder.WriteString(" ")
		builder.WriteString(s[i : i+3])
	}
	return builder.String()
}
