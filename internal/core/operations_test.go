package core_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pellets-tracker/internal/core"
)

func TestAddBrand(t *testing.T) {
	t.Parallel()

	type params struct {
		datastore core.DataStore
		input     core.CreateBrandParams
	}
	type want struct {
		err        error
		brandName  string
		brandCount int
	}
	tcs := []struct {
		name   string
		params params
		want   want
	}{
		{
			name: "creates brand with normalized name",
			params: params{
				datastore: core.DataStore{},
				input: core.CreateBrandParams{
					Name: "  Premium   Pellets  ",
				},
			},
			want: want{
				err:        nil,
				brandName:  "Premium Pellets",
				brandCount: 1,
			},
		},
		{
			name: "rejects duplicate brand names",
			params: params{
				datastore: func() core.DataStore {
					ds := core.DataStore{}
					brand, err := core.AddBrand(&ds, core.CreateBrandParams{Name: "Existing"})
					if err != nil {
						panic(err)
					}
					_ = brand
					return ds
				}(),
				input: core.CreateBrandParams{Name: "existing"},
			},
			want: want{
				err:        core.ValidationErrors{{Field: "name", Message: "brand name already exists"}},
				brandName:  "Existing",
				brandCount: 1,
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ds := tc.params.datastore
			brand, err := core.AddBrand(&ds, tc.params.input)

			if tc.want.err == nil {
				require.NoError(t, err, tc.name)
				assert.Equal(t, tc.want.brandName, brand.Name, tc.name)
				assert.Equal(t, tc.want.brandCount, len(ds.Brands), tc.name)
				assert.False(t, ds.Meta.UpdatedAt.IsZero(), tc.name)
			} else {
				assert.Error(t, err, tc.name)
				var vErr core.ValidationErrors
				assert.True(t, errors.As(err, &vErr), tc.name)
				assert.True(t, vErr.Has("name"), tc.name)
				assert.Equal(t, len(tc.params.datastore.Brands), len(ds.Brands), tc.name)
			}
		})
	}
}

func TestAddPurchase(t *testing.T) {
	t.Parallel()

	type params struct {
		existing core.DataStore
		input    core.CreatePurchaseParams
	}
	type want struct {
		err             error
		totalPriceCents core.Money
		totalWeightKg   float64
		bagWeightKg     float64
	}
	ds := core.DataStore{}
	brand, err := core.AddBrand(&ds, core.CreateBrandParams{Name: "Test"})
	require.NoError(t, err, "seed brand")

	tcs := []struct {
		name   string
		params params
		want   want
	}{
		{
			name: "computes total price and weight",
			params: params{
				existing: ds,
				input: core.CreatePurchaseParams{
					BrandID:     brand.ID,
					PurchasedAt: time.Date(2024, time.January, 2, 0, 0, 0, 0, time.UTC),
					Bags:        4,
					BagWeightKg: 15,
					UnitPrice:   core.Money(499),
					Notes:       "Stock initial",
				},
			},
			want: want{
				err:             nil,
				totalPriceCents: core.Money(1996),
				totalWeightKg:   60,
				bagWeightKg:     15,
			},
		},
		{
			name: "fails when bag weight missing",
			params: params{
				existing: ds,
				input: core.CreatePurchaseParams{
					BrandID:     brand.ID,
					PurchasedAt: time.Now(),
					Bags:        2,
					BagWeightKg: 0,
					UnitPrice:   core.Money(1000),
				},
			},
			want: want{err: core.ValidationErrors{{Field: "bag_weight_kg", Message: "bag weight must be greater than zero"}}},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dsCopy := tc.params.existing
			purchase, err := core.AddPurchase(&dsCopy, tc.params.input)

			if tc.want.err == nil {
				require.NoError(t, err, tc.name)
				assert.Equal(t, tc.want.totalPriceCents, purchase.TotalPriceCents, tc.name)
				assert.InDelta(t, tc.want.totalWeightKg, purchase.TotalWeightKg, 1e-9, tc.name)
				assert.InDelta(t, tc.want.bagWeightKg, purchase.BagWeightKg, 1e-9, tc.name)
			} else {
				assert.Error(t, err, tc.name)
				var vErr core.ValidationErrors
				assert.True(t, errors.As(err, &vErr), tc.name)
				assert.True(t, vErr.Has("bag_weight_kg"), tc.name)
				assert.Equal(t, len(tc.params.existing.Purchases), len(dsCopy.Purchases), tc.name)
			}
		})
	}
}

func TestUpdatePurchase(t *testing.T) {
	t.Parallel()

	type params struct {
		existing core.DataStore
		update   core.UpdatePurchaseParams
	}
	type want struct {
		err             error
		totalPriceCents core.Money
		totalWeightKg   float64
	}

	base := core.DataStore{}
	brand, err := core.AddBrand(&base, core.CreateBrandParams{Name: "Brand"})
	require.NoError(t, err, "seed brand")
	purchase, err := core.AddPurchase(&base, core.CreatePurchaseParams{
		BrandID:     brand.ID,
		PurchasedAt: time.Date(2024, time.January, 5, 0, 0, 0, 0, time.UTC),
		Bags:        3,
		BagWeightKg: 12.5,
		UnitPrice:   core.Money(700),
	})
	require.NoError(t, err, "seed purchase")

	tcs := []struct {
		name   string
		params params
		want   want
	}{
		{
			name: "updates totals when bags change",
			params: params{
				existing: base,
				update: core.UpdatePurchaseParams{
					PurchasedAt: purchase.PurchasedAt,
					Bags:        5,
					BagWeightKg: 13,
					UnitPrice:   core.Money(650),
					Notes:       "ajusté",
				},
			},
			want: want{
				err:             nil,
				totalPriceCents: core.Money(3250),
				totalWeightKg:   65,
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dsCopy := tc.params.existing
			updated, err := core.UpdatePurchase(&dsCopy, purchase.ID, tc.params.update)
			require.NoError(t, err, tc.name)
			assert.Equal(t, tc.want.totalPriceCents, updated.TotalPriceCents, tc.name)
			assert.InDelta(t, tc.want.totalWeightKg, updated.TotalWeightKg, 1e-9, tc.name)
		})
	}
}

func TestDeleteBrand(t *testing.T) {
	t.Parallel()

	type params struct {
		datastore core.DataStore
		brandID   core.ID
	}
	type want struct {
		err error
	}

	seed := core.DataStore{}
	brand, err := core.AddBrand(&seed, core.CreateBrandParams{Name: "Brand"})
	require.NoError(t, err, "seed brand")
	_, err = core.AddPurchase(&seed, core.CreatePurchaseParams{
		BrandID:     brand.ID,
		PurchasedAt: time.Now().Add(-time.Hour),
		Bags:        1,
		BagWeightKg: 15,
		UnitPrice:   core.Money(500),
	})
	require.NoError(t, err, "seed purchase")

	tcs := []struct {
		name   string
		params params
		want   want
	}{
		{
			name: "prevents deletion when brand in use",
			params: params{
				datastore: seed,
				brandID:   brand.ID,
			},
			want: want{err: core.ErrBrandInUse},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ds := tc.params.datastore
			err := core.DeleteBrand(&ds, tc.params.brandID)
			assert.ErrorIs(t, err, tc.want.err, tc.name)
		})
	}
}

func TestAddConsumption(t *testing.T) {
	t.Parallel()

	type params struct {
		datastore core.DataStore
		input     core.CreateConsumptionParams
	}
	type want struct {
		bagCount int
	}

	seed := core.DataStore{}
	brand, err := core.AddBrand(&seed, core.CreateBrandParams{Name: "Brand"})
	require.NoError(t, err, "seed brand")

	tcs := []struct {
		name   string
		params params
		want   want
	}{
		{
			name: "creates consumption entry",
			params: params{
				datastore: seed,
				input: core.CreateConsumptionParams{
					BrandID:    brand.ID,
					ConsumedAt: time.Date(2024, time.February, 1, 0, 0, 0, 0, time.UTC),
					Bags:       2,
					Notes:      "Hiver",
				},
			},
			want: want{
				bagCount: 2,
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ds := tc.params.datastore
			consumption, err := core.AddConsumption(&ds, tc.params.input)
			require.NoError(t, err, tc.name)
			assert.Equal(t, tc.want.bagCount, consumption.Bags, tc.name)
			assert.Equal(t, 1, len(ds.Consumptions), tc.name)
		})
	}
}
