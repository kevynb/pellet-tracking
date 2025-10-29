package http_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	core "pellets-tracker/internal/core"
	httpserver "pellets-tracker/internal/http"
	"pellets-tracker/internal/store"
)

func TestServerAPIIntegration(t *testing.T) {
	t.Parallel()

	type params struct{}
	type want struct {
		brandName         string
		purchaseTotal     core.Money
		purchaseWeightKg  float64
		remainingBags     int
		consumedTotal     core.Money
		averageCostPerBag core.Money
	}

	tcs := []struct {
		name   string
		params params
		want   want
	}{
		{
			name:   "full CRUD and stats flow",
			params: params{},
			want: want{
				brandName:         "Granules Premium",
				purchaseTotal:     core.Money(4 * 450),
				purchaseWeightKg:  4 * 15,
				remainingBags:     2,
				consumedTotal:     core.Money(2 * 450),
				averageCostPerBag: core.Money(450),
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			dataFile := filepath.Join(tmpDir, "data.json")
			backupDir := filepath.Join(tmpDir, "backups")

			jsonStore, err := store.NewJSONStore(dataFile, backupDir)
			require.NoError(t, err, tc.name)

			server := httpserver.NewServer(jsonStore)
			ts := httptest.NewServer(server.Handler())
			t.Cleanup(ts.Close)

			client := ts.Client()
			if transport, ok := client.Transport.(*http.Transport); ok {
				transport.DisableCompression = true
			}

			brandReq := map[string]string{
				"name":         tc.want.brandName,
				"description":  "Pellets de chêne",
				"image_base64": "",
			}
			resp, body := doJSONRequest(t, client, http.MethodPost, ts.URL, "/api/marques", brandReq)
			require.Equal(t, http.StatusCreated, resp.StatusCode, tc.name)
			var brand core.Brand
			require.NoError(t, json.Unmarshal(body, &brand), tc.name)
			assert.Equal(t, tc.want.brandName, brand.Name, tc.name)

			purchaseReq := map[string]any{
				"brand_id":         string(brand.ID),
				"purchased_at":     time.Date(2024, time.January, 10, 15, 0, 0, 0, time.UTC).Format(time.RFC3339),
				"bags":             4,
				"bag_weight_kg":    15.0,
				"unit_price_cents": tc.want.averageCostPerBag,
				"notes":            "Stock initial",
			}
			resp, body = doJSONRequest(t, client, http.MethodPost, ts.URL, "/api/achats", purchaseReq)
			require.Equal(t, http.StatusCreated, resp.StatusCode, tc.name)
			var purchase core.Purchase
			require.NoError(t, json.Unmarshal(body, &purchase), tc.name)
			assert.Equal(t, tc.want.purchaseTotal, purchase.TotalPriceCents, tc.name)
			assert.InDelta(t, tc.want.purchaseWeightKg, purchase.TotalWeightKg, 1e-9, tc.name)
			assert.InDelta(t, 15.0, purchase.BagWeightKg, 1e-9, tc.name)

			consumptionReq := map[string]any{
				"brand_id":    string(brand.ID),
				"consumed_at": time.Date(2024, time.January, 20, 8, 0, 0, 0, time.UTC).Format(time.RFC3339),
				"bags":        2,
				"notes":       "Premier hiver",
			}
			resp, body = doJSONRequest(t, client, http.MethodPost, ts.URL, "/api/consommations", consumptionReq)
			require.Equal(t, http.StatusCreated, resp.StatusCode, tc.name)
			var consumption core.Consumption
			require.NoError(t, json.Unmarshal(body, &consumption), tc.name)
			assert.Equal(t, 2, consumption.Bags, tc.name)

			resp, body = doJSONRequest(t, client, http.MethodGet, ts.URL, "/api/stats", nil)
			require.Equal(t, http.StatusOK, resp.StatusCode, tc.name)
			var stats struct {
				InvestiCents         core.Money             `json:"investi_cents"`
				ConsommeCents        core.Money             `json:"consomme_cents"`
				ConsommationsDetail  []core.ConsumptionCost `json:"consommations_detail"`
				Inventaire           core.InventorySummary  `json:"inventaire"`
				CoutMoyenParSacCents core.Money             `json:"cout_moyen_par_sac_cents"`
			}
			require.NoError(t, json.Unmarshal(body, &stats), tc.name)
			assert.Equal(t, tc.want.purchaseTotal, stats.InvestiCents, tc.name)
			assert.Equal(t, tc.want.consumedTotal, stats.ConsommeCents, tc.name)
			assert.Equal(t, tc.want.remainingBags, stats.Inventaire.TotalBags, tc.name)
			assert.Equal(t, tc.want.averageCostPerBag, stats.CoutMoyenParSacCents, tc.name)
		})
	}
}

func doJSONRequest(t *testing.T, client *http.Client, method, baseURL, path string, payload any) (*http.Response, []byte) {
	t.Helper()

	var bodyReader *strings.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		require.NoError(t, err, "marshal payload")
		bodyReader = strings.NewReader(string(b))
	} else {
		bodyReader = strings.NewReader("")
	}

	req, err := http.NewRequest(method, baseURL+path, bodyReader)
	require.NoError(t, err, "create request")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	require.NoError(t, err, "do request")
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "read body")
	return resp, data
}
