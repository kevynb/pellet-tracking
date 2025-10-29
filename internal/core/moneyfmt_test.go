package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"pellets-tracker/internal/core"
)

func TestFormatMoney(t *testing.T) {
	t.Parallel()

	type params struct {
		amount core.Money
	}
	type want struct {
		formatted string
	}

	tcs := []struct {
		name   string
		params params
		want   want
	}{
		{
			name:   "zero",
			params: params{amount: 0},
			want:   want{formatted: "0,00 €"},
		},
		{
			name:   "positive",
			params: params{amount: core.Money(12345)},
			want:   want{formatted: "123,45 €"},
		},
		{
			name:   "negative",
			params: params{amount: core.Money(-999)},
			want:   want{formatted: "-9,99 €"},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			formatted := core.FormatMoney(tc.params.amount)
			assert.Equal(t, tc.want.formatted, formatted, tc.name)
		})
	}
}

func TestRoundBancaire(t *testing.T) {
	t.Parallel()

	type params struct {
		value float64
	}
	type want struct {
		rounded float64
	}

	tcs := []struct {
		name   string
		params params
		want   want
	}{
		{
			name:   "rounds to even up",
			params: params{value: 1.235},
			want:   want{rounded: 1.24},
		},
		{
			name:   "rounds to even down",
			params: params{value: 1.225},
			want:   want{rounded: 1.22},
		},
		{
			name:   "handles negative",
			params: params{value: -2.555},
			want:   want{rounded: -2.56},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rounded := core.RoundBancaire(tc.params.value)
			assert.Equal(t, tc.want.rounded, rounded, tc.name)
		})
	}
}

func TestFormatFR(t *testing.T) {
	t.Parallel()

	type params struct {
		amount float64
	}
	type want struct {
		formatted string
	}

	tcs := []struct {
		name   string
		params params
		want   want
	}{
		{
			name:   "formats thousands",
			params: params{amount: 1234.56},
			want:   want{formatted: "1 234,56 €"},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			formatted := core.FormatFR(tc.params.amount)
			assert.Equal(t, tc.want.formatted, formatted, tc.name)
		})
	}
}
