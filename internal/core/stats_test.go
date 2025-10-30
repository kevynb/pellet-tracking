package core_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pellets-tracker/internal/core"
)

func TestComputeInvesti(t *testing.T) {
	t.Parallel()

	type params struct {
		datastore core.DataStore
		from      time.Time
		to        time.Time
	}
	type want struct {
		total core.Money
	}

	ds := sampleDataStore(t)

	tcs := []struct {
		name   string
		params params
		want   want
	}{
		{
			name: "sums all purchases within range",
			params: params{
				datastore: ds,
				from:      time.Time{},
				to:        time.Time{},
			},
			want: want{total: core.Money(5*550 + 3*600)},
		},
		{
			name: "filters by date",
			params: params{
				datastore: ds,
				from:      time.Date(2024, time.February, 1, 0, 0, 0, 0, time.UTC),
				to:        time.Date(2024, time.February, 28, 23, 59, 59, 0, time.UTC),
			},
			want: want{total: core.Money(3 * 600)},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			total := core.ComputeInvesti(&tc.params.datastore, tc.params.from, tc.params.to)
			assert.Equal(t, tc.want.total, total, tc.name)
		})
	}
}

func TestComputeConsoValue(t *testing.T) {
	t.Parallel()

	type params struct {
		datastore core.DataStore
		from      time.Time
		to        time.Time
	}
	type want struct {
		total core.Money
	}

	ds := sampleDataStore(t)

	tcs := []struct {
		name   string
		params params
		want   want
	}{
		{
			name: "calculates FIFO consumption cost",
			params: params{
				datastore: ds,
				from:      time.Time{},
				to:        time.Time{},
			},
			want: want{total: core.Money(2 * 550)},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			total, _, err := core.ComputeConsoValue(&tc.params.datastore, tc.params.from, tc.params.to)
			require.NoError(t, err, tc.name)
			assert.Equal(t, tc.want.total, total, tc.name)
		})
	}
}

func TestComputeInventaire(t *testing.T) {
	t.Parallel()

	ds := sampleDataStore(t)

	type params struct {
		datastore core.DataStore
	}
	type want struct {
		summary core.InventorySummary
	}

	tcs := []struct {
		name   string
		params params
		want   want
	}{
		{
			name:   "computes remaining bags and weight",
			params: params{datastore: ds},
			want: want{summary: core.InventorySummary{
				TotalBags:     6,
				TotalWeightKg: 6 * 15,
				TotalCost:     core.Money(3*600 + 3*550),
				Brands: []core.BrandInventory{
					{
						BrandID:   ds.Brands[0].ID,
						BrandName: ds.Brands[0].Name,
						Bags:      6,
						WeightKg:  6 * 15,
						TotalCost: core.Money(3*600 + 3*550),
					},
				},
			}},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			summary, err := core.ComputeInventaire(&tc.params.datastore)
			require.NoError(t, err, tc.name)
			assert.Equal(t, tc.want.summary, summary, tc.name)
		})
	}
}

func TestComputeSacsParMois(t *testing.T) {
	t.Parallel()

	ds := sampleDataStore(t)

	type params struct {
		datastore core.DataStore
		from      time.Time
		to        time.Time
	}
	type want struct {
		points []core.MonthlyBags
	}

	tcs := []struct {
		name   string
		params params
		want   want
	}{
		{
			name: "aggregates consumption per month",
			params: params{
				datastore: ds,
				from:      time.Time{},
				to:        time.Time{},
			},
			want: want{points: []core.MonthlyBags{{
				Month: time.Date(2024, time.February, 1, 0, 0, 0, 0, time.UTC),
				Bags:  2,
			}}}},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			points, err := core.ComputeSacsParMois(&tc.params.datastore, tc.params.from, tc.params.to)
			require.NoError(t, err, tc.name)
			assert.Equal(t, tc.want.points, points, tc.name)
		})
	}
}

func TestComputeCoutMoyenParSac(t *testing.T) {
	t.Parallel()

	ds := sampleDataStore(t)

	type params struct {
		datastore core.DataStore
	}
	type want struct {
		average core.Money
	}

	tcs := []struct {
		name   string
		params params
		want   want
	}{
		{
			name:   "averages inventory cost per bag",
			params: params{datastore: ds},
			want:   want{average: core.Money(550)},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			avg, err := core.ComputeCoutMoyenParSac(&tc.params.datastore, time.Time{}, time.Time{})
			require.NoError(t, err, tc.name)
			assert.Equal(t, tc.want.average, avg, tc.name)
		})
	}
}

func sampleDataStore(t *testing.T) core.DataStore {
	t.Helper()

	ds := core.DataStore{}
	brand, err := core.AddBrand(&ds, core.CreateBrandParams{Name: "Granules"})
	require.NoError(t, err, "seed brand")

	_, err = core.AddPurchase(&ds, core.CreatePurchaseParams{
		BrandID:     brand.ID,
		PurchasedAt: time.Date(2024, time.January, 10, 0, 0, 0, 0, time.UTC),
		Bags:        5,
		BagWeightKg: 15,
		UnitPrice:   core.Money(550),
	})
	require.NoError(t, err, "seed january purchase")

	_, err = core.AddPurchase(&ds, core.CreatePurchaseParams{
		BrandID:     brand.ID,
		PurchasedAt: time.Date(2024, time.February, 5, 0, 0, 0, 0, time.UTC),
		Bags:        3,
		BagWeightKg: 15,
		UnitPrice:   core.Money(600),
	})
	require.NoError(t, err, "seed february purchase")

	_, err = core.AddConsumption(&ds, core.CreateConsumptionParams{
		BrandID:    brand.ID,
		ConsumedAt: time.Date(2024, time.February, 20, 0, 0, 0, 0, time.UTC),
		Bags:       2,
	})
	require.NoError(t, err, "seed consumption")

	return ds
}
