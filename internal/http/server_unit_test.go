package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.uber.org/mock/gomock"
	"pellets-tracker/internal/core"
	"pellets-tracker/internal/http/mock"
)

func TestServer_createPurchase(t *testing.T) {
	t.Parallel()

	brandID := core.NewID()
	baseData := core.DataStore{
		Brands: []core.Brand{{
			Meta: core.Meta{ID: brandID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			Name: "Granules",
		}},
	}

	type params struct {
		payload    map[string]any
		dataReturn core.DataStore
		replaceErr error
	}
	type want struct {
		statusCode      int
		expectTotalKg   float64
		expectBagWeight float64
		expectErrorBody bool
	}

	tcs := []struct {
		name   string
		params params
		want   want
	}{
		{
			name: "creates purchase with computed totals",
			params: params{
				payload: map[string]any{
					"brand_id":         string(brandID),
					"purchased_at":     time.Now().Format(time.RFC3339),
					"bags":             3,
					"bag_weight_kg":    14.5,
					"unit_price_cents": 499,
				},
				dataReturn: baseData,
				replaceErr: nil,
			},
			want: want{
				statusCode:      http.StatusCreated,
				expectTotalKg:   43.5,
				expectBagWeight: 14.5,
			},
		},
		{
			name: "handles persistence failures",
			params: params{
				payload: map[string]any{
					"brand_id":         string(brandID),
					"purchased_at":     time.Now().Format(time.RFC3339),
					"bags":             2,
					"bag_weight_kg":    15.0,
					"unit_price_cents": 450,
				},
				dataReturn: baseData,
				replaceErr: errors.New("disk full"),
			},
			want: want{
				statusCode:      http.StatusInternalServerError,
				expectErrorBody: true,
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			storeMock := mock.NewMockDataStore(ctrl)
			storeMock.EXPECT().Data().Return(tc.params.dataReturn).Times(1)
			storeMock.EXPECT().Replace(gomock.Any()).DoAndReturn(func(ds core.DataStore) error {
				if tc.want.expectErrorBody {
					return tc.params.replaceErr
				}
				if !assert.Equal(t, 1, len(ds.Purchases), tc.name) {
					return tc.params.replaceErr
				}
				assert.InDelta(t, tc.want.expectTotalKg, ds.Purchases[0].TotalWeightKg, 1e-9, tc.name)
				assert.InDelta(t, tc.want.expectBagWeight, ds.Purchases[0].BagWeightKg, 1e-9, tc.name)
				return tc.params.replaceErr
			}).Times(1)

			server := NewServer(storeMock)

			body, err := json.Marshal(tc.params.payload)
			require.NoError(t, err, tc.name)

			req := httptest.NewRequest(http.MethodPost, "/api/achats", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			server.createPurchase(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			assert.Equal(t, tc.want.statusCode, res.StatusCode, tc.name)
			responseBody, err := io.ReadAll(res.Body)
			require.NoError(t, err, tc.name)

			if tc.want.expectErrorBody {
				assert.Contains(t, string(responseBody), "error", tc.name)
				return
			}

			var purchase core.Purchase
			require.NoError(t, json.Unmarshal(responseBody, &purchase), tc.name)
			assert.InDelta(t, tc.want.expectTotalKg, purchase.TotalWeightKg, 1e-9, tc.name)
			assert.InDelta(t, tc.want.expectBagWeight, purchase.BagWeightKg, 1e-9, tc.name)
		})
	}
}
