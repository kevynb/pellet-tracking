package http_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	core "pellets-tracker/internal/core"
	httpserver "pellets-tracker/internal/http"
	"pellets-tracker/internal/store"
)

func TestServerAPIIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	dataFile := filepath.Join(tmpDir, "data.json")
	backupDir := filepath.Join(tmpDir, "backups")

	jsonStore, err := store.NewJSONStore(dataFile, backupDir)
	if err != nil {
		t.Fatalf("create JSON store: %v", err)
	}

	server := httpserver.NewServer(jsonStore)
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	client := ts.Client()
	if transport, ok := client.Transport.(*http.Transport); ok {
		transport.DisableCompression = true
	}

	brandReq := map[string]string{
		"name":         "Granules Premium",
		"description":  "Pellets de chêne",
		"image_base64": "",
	}
	resp, body := doJSONRequest(t, client, http.MethodPost, ts.URL, "/api/marques", brandReq)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status for create brand: %d", resp.StatusCode)
	}
	var brand core.Brand
	if err := json.Unmarshal(body, &brand); err != nil {
		t.Fatalf("decode brand: %v", err)
	}
	if brand.Name != "Granules Premium" {
		t.Fatalf("unexpected brand name: %s", brand.Name)
	}

	resp, body = doJSONRequest(t, client, http.MethodGet, ts.URL, "/api/marques", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status for list brands: %d", resp.StatusCode)
	}
	var brands []core.Brand
	if err := json.Unmarshal(body, &brands); err != nil {
		t.Fatalf("decode brands: %v", err)
	}
	if len(brands) != 1 {
		t.Fatalf("expected 1 brand, got %d", len(brands))
	}

	purchasedAt := time.Date(2024, time.January, 10, 15, 0, 0, 0, time.UTC)
	price := core.Money(450)
	purchaseReq := map[string]any{
		"brand_id":         string(brand.ID),
		"purchased_at":     purchasedAt.Format(time.RFC3339),
		"bags":             4,
		"weight_kg":        60.0,
		"unit_price_cents": price,
		"notes":            "Stock initial",
	}
	resp, body = doJSONRequest(t, client, http.MethodPost, ts.URL, "/api/achats", purchaseReq)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status for create purchase: %d", resp.StatusCode)
	}
	var purchase core.Purchase
	if err := json.Unmarshal(body, &purchase); err != nil {
		t.Fatalf("decode purchase: %v", err)
	}
	if purchase.TotalPriceCents != price.MulInt(4) {
		t.Fatalf("unexpected total price: %d", purchase.TotalPriceCents)
	}

	resp, body = doJSONRequest(t, client, http.MethodGet, ts.URL, "/api/achats", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status for list purchases: %d", resp.StatusCode)
	}
	var purchases []core.Purchase
	if err := json.Unmarshal(body, &purchases); err != nil {
		t.Fatalf("decode purchases: %v", err)
	}
	if len(purchases) != 1 {
		t.Fatalf("expected 1 purchase, got %d", len(purchases))
	}

	consumptionReq := map[string]any{
		"brand_id":    string(brand.ID),
		"consumed_at": purchasedAt.Add(48 * time.Hour).Format(time.RFC3339),
		"bags":        2,
		"notes":       "Premier hiver",
	}
	resp, body = doJSONRequest(t, client, http.MethodPost, ts.URL, "/api/consommations", consumptionReq)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status for create consumption: %d", resp.StatusCode)
	}
	var consumption core.Consumption
	if err := json.Unmarshal(body, &consumption); err != nil {
		t.Fatalf("decode consumption: %v", err)
	}
	if consumption.Bags != 2 {
		t.Fatalf("unexpected consumption bags: %d", consumption.Bags)
	}

	resp, body = doJSONRequest(t, client, http.MethodGet, ts.URL, "/api/consommations", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status for list consumptions: %d", resp.StatusCode)
	}
	var consumptions []core.Consumption
	if err := json.Unmarshal(body, &consumptions); err != nil {
		t.Fatalf("decode consumptions: %v", err)
	}
	if len(consumptions) != 1 {
		t.Fatalf("expected 1 consumption, got %d", len(consumptions))
	}

	resp, body = doJSONRequest(t, client, http.MethodGet, ts.URL, "/api/stats", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status for stats: %d", resp.StatusCode)
	}
	var stats struct {
		InvestiCents         core.Money             `json:"investi_cents"`
		ConsommeCents        core.Money             `json:"consomme_cents"`
		ConsommationsDetail  []core.ConsumptionCost `json:"consommations_detail"`
		Inventaire           core.InventorySummary  `json:"inventaire"`
		SacsParMois          []core.MonthlyBags     `json:"sacs_par_mois"`
		CoutMoyenParSacCents core.Money             `json:"cout_moyen_par_sac_cents"`
	}
	if err := json.Unmarshal(body, &stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	expectedInvest := price.MulInt(4)
	if stats.InvestiCents != expectedInvest {
		t.Fatalf("unexpected invested total: %d", stats.InvestiCents)
	}
	if stats.ConsommeCents != price.MulInt(2) {
		t.Fatalf("unexpected consumed total: %d", stats.ConsommeCents)
	}
	if stats.Inventaire.TotalBags != 2 {
		t.Fatalf("unexpected remaining bags: %d", stats.Inventaire.TotalBags)
	}
	if stats.CoutMoyenParSacCents != price {
		t.Fatalf("unexpected average cost: %d", stats.CoutMoyenParSacCents)
	}
	if len(stats.ConsommationsDetail) != 1 {
		t.Fatalf("expected detailed consumption entry")
	}
}

func TestServerStatsInsufficientInventory(t *testing.T) {
	tmpDir := t.TempDir()
	dataFile := filepath.Join(tmpDir, "data.json")
	backupDir := filepath.Join(tmpDir, "backups")

	jsonStore, err := store.NewJSONStore(dataFile, backupDir)
	if err != nil {
		t.Fatalf("create JSON store: %v", err)
	}

	server := httpserver.NewServer(jsonStore)
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	client := ts.Client()
	if transport, ok := client.Transport.(*http.Transport); ok {
		transport.DisableCompression = true
	}

	brandReq := map[string]string{
		"name": "Test Brand",
	}
	resp, body := doJSONRequest(t, client, http.MethodPost, ts.URL, "/api/marques", brandReq)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status for create brand: %d", resp.StatusCode)
	}
	var brand core.Brand
	if err := json.Unmarshal(body, &brand); err != nil {
		t.Fatalf("decode brand: %v", err)
	}

	purchaseReq := map[string]any{
		"brand_id":         string(brand.ID),
		"purchased_at":     time.Now().UTC().Format(time.RFC3339),
		"bags":             1,
		"weight_kg":        15.0,
		"unit_price_cents": core.Money(600),
	}
	resp, _ = doJSONRequest(t, client, http.MethodPost, ts.URL, "/api/achats", purchaseReq)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status for create purchase: %d", resp.StatusCode)
	}

	overConsumeReq := map[string]any{
		"brand_id":    string(brand.ID),
		"consumed_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		"bags":        2,
	}
	resp, _ = doJSONRequest(t, client, http.MethodPost, ts.URL, "/api/consommations", overConsumeReq)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status for create consumption: %d", resp.StatusCode)
	}

	resp, body = doJSONRequest(t, client, http.MethodGet, ts.URL, "/api/stats", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected conflict for insufficient inventory, got %d", resp.StatusCode)
	}
	var errPayload map[string]any
	if err := json.Unmarshal(body, &errPayload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if errMsg, _ := errPayload["error"].(string); errMsg == "" {
		t.Fatalf("expected error message in payload")
	}
}

func doJSONRequest(t *testing.T, client *http.Client, method, baseURL, path string, payload any) (*http.Response, []byte) {
	t.Helper()

	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, baseURL+path, body)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("perform request: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	return resp, data
}
