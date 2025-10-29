package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pellets-tracker/internal/core"
)

var binaryPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "pellets-e2e-bin-*")
	if err != nil {
		panic(fmt.Sprintf("create temp dir: %v", err))
	}

	binaryPath = filepath.Join(dir, "pellets")
	wd, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("determine working directory: %v", err))
	}
	repoRoot := filepath.Dir(filepath.Dir(wd))
	build := exec.Command("go", "build", "-trimpath", "-o", binaryPath, "./cmd/app")
	build.Dir = repoRoot
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic(fmt.Sprintf("build binary: %v", err))
	}

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

type brandSeed struct {
	Name        string
	Description string
}

type purchaseSeed struct {
	BrandName      string
	PurchasedAt    time.Time
	Bags           int
	BagWeightKg    float64
	UnitPriceCents int64
	Notes          string
}

type consumptionSeed struct {
	BrandName  string
	ConsumedAt time.Time
	Bags       int
	Notes      string
}

type params struct {
	brands       []brandSeed
	purchases    []purchaseSeed
	consumptions []consumptionSeed
}

type want struct {
	invested            core.Money
	consumed            core.Money
	inventoryBags       int
	inventoryWeight     float64
	inventoryCost       core.Money
	averageCost         core.Money
	monthlyBags         int
	homeSnippets        []string
	statsSnippets       []string
	consumptionSnippets []string
}

type testCase struct {
	name   string
	params params
	want   want
}

func TestPelletsEndToEnd(t *testing.T) {
	t.Parallel()

	tcs := []testCase{
		{
			name: "purchases populate stats and home table",
			params: params{
				brands: []brandSeed{
					{Name: "MontBlanc"},
					{Name: "EcoChaleur"},
				},
				purchases: []purchaseSeed{
					{
						BrandName:      "MontBlanc",
						PurchasedAt:    time.Date(2024, time.October, 1, 8, 30, 0, 0, time.UTC),
						Bags:           4,
						BagWeightKg:    15,
						UnitPriceCents: 499,
						Notes:          "Stock initial",
					},
					{
						BrandName:      "EcoChaleur",
						PurchasedAt:    time.Date(2024, time.October, 5, 10, 0, 0, 0, time.UTC),
						Bags:           3,
						BagWeightKg:    10,
						UnitPriceCents: 550,
						Notes:          "Palette promo",
					},
					{
						BrandName:      "MontBlanc",
						PurchasedAt:    time.Date(2024, time.November, 1, 9, 0, 0, 0, time.UTC),
						Bags:           2,
						BagWeightKg:    15,
						UnitPriceCents: 520,
						Notes:          "Complément",
					},
				},
			},
			want: want{
				invested:        core.Money(4686),
				consumed:        0,
				inventoryBags:   9,
				inventoryWeight: 120,
				inventoryCost:   core.Money(4686),
				averageCost:     0,
				monthlyBags:     0,
				homeSnippets: []string{
					"MontBlanc",
					"EcoChaleur",
					"Stock initial",
					"60,00",
				},
				statsSnippets: []string{
					"9 sacs",
					"120,00 kg",
					"46,86 €",
				},
			},
		},
		{
			name: "consumptions update fifo stats and listings",
			params: params{
				brands: []brandSeed{
					{Name: "MontBlanc"},
					{Name: "EcoChaleur"},
				},
				purchases: []purchaseSeed{
					{
						BrandName:      "MontBlanc",
						PurchasedAt:    time.Date(2024, time.October, 1, 8, 30, 0, 0, time.UTC),
						Bags:           4,
						BagWeightKg:    15,
						UnitPriceCents: 499,
					},
					{
						BrandName:      "MontBlanc",
						PurchasedAt:    time.Date(2024, time.November, 1, 9, 0, 0, 0, time.UTC),
						Bags:           2,
						BagWeightKg:    15,
						UnitPriceCents: 520,
					},
					{
						BrandName:      "EcoChaleur",
						PurchasedAt:    time.Date(2024, time.October, 5, 10, 0, 0, 0, time.UTC),
						Bags:           3,
						BagWeightKg:    10,
						UnitPriceCents: 550,
					},
				},
				consumptions: []consumptionSeed{
					{
						BrandName:  "MontBlanc",
						ConsumedAt: time.Date(2024, time.November, 10, 18, 0, 0, 0, time.UTC),
						Bags:       5,
						Notes:      "Pics de froid",
					},
					{
						BrandName:  "EcoChaleur",
						ConsumedAt: time.Date(2024, time.November, 20, 7, 0, 0, 0, time.UTC),
						Bags:       1,
					},
				},
			},
			want: want{
				invested:        core.Money(4686),
				consumed:        core.Money(3066),
				inventoryBags:   3,
				inventoryWeight: 35,
				inventoryCost:   core.Money(1620),
				averageCost:     core.Money(511),
				monthlyBags:     6,
				homeSnippets: []string{
					"MontBlanc",
					"EcoChaleur",
				},
				statsSnippets: []string{
					"3 sacs",
					"35,00 kg",
					"16,20 €",
				},
				consumptionSnippets: []string{
					"Pics de froid",
					">5<",
					"MontBlanc",
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dataDir := t.TempDir()
			backupDir := filepath.Join(dataDir, "backups")
			require.NoError(t, os.MkdirAll(backupDir, 0o755), tc.name)

			listenAddr := freePort(t)
			env := map[string]string{
				"PELLETS_DATA_FILE":   filepath.Join(dataDir, "pellets.json"),
				"PELLETS_BACKUP_DIR":  backupDir,
				"PELLETS_LISTEN_ADDR": listenAddr,
			}

			client, stop := launchServer(t, env, tc.name)
			defer stop()

			brandIDs := registerBrands(t, client, listenAddr, tc.params.brands, tc.name)
			registerPurchases(t, client, listenAddr, brandIDs, tc.params.purchases, tc.name)
			registerConsumptions(t, client, listenAddr, brandIDs, tc.params.consumptions, tc.name)

			verifyAPIState(t, client, listenAddr, tc)
			verifyPages(t, client, listenAddr, tc)
		})
	}
}

func registerBrands(t *testing.T, client *http.Client, addr string, seeds []brandSeed, caseName string) map[string]string {
	t.Helper()
	ids := make(map[string]string, len(seeds))
	for _, seed := range seeds {
		payload := map[string]any{
			"name":        seed.Name,
			"description": seed.Description,
		}
		resp := mustDo(t, client, http.MethodPost, fmt.Sprintf("http://%s/api/marques", addr), payload, fmt.Sprintf("%s/%s", caseName, seed.Name))
		assert.Equal(t, http.StatusCreated, resp.StatusCode, fmt.Sprintf("%s/%s", caseName, seed.Name))
		var body struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		decodeJSONBody(t, resp, &body, fmt.Sprintf("%s/%s", caseName, seed.Name))
		ids[seed.Name] = body.ID
	}
	return ids
}

func registerPurchases(t *testing.T, client *http.Client, addr string, brandIDs map[string]string, seeds []purchaseSeed, name string) {
	t.Helper()
	for _, seed := range seeds {
		brandID := brandIDs[seed.BrandName]
		require.NotEmpty(t, brandID, fmt.Sprintf("%s/%s", name, seed.BrandName))
		payload := map[string]any{
			"brand_id":         brandID,
			"purchased_at":     seed.PurchasedAt.Format(time.RFC3339),
			"bags":             seed.Bags,
			"bag_weight_kg":    seed.BagWeightKg,
			"unit_price_cents": seed.UnitPriceCents,
			"notes":            seed.Notes,
		}
		resp := mustDo(t, client, http.MethodPost, fmt.Sprintf("http://%s/api/achats", addr), payload, name)
		assert.Equal(t, http.StatusCreated, resp.StatusCode, name)
		resp.Body.Close()
	}
}

func registerConsumptions(t *testing.T, client *http.Client, addr string, brandIDs map[string]string, seeds []consumptionSeed, name string) {
	t.Helper()
	for _, seed := range seeds {
		brandID := brandIDs[seed.BrandName]
		require.NotEmpty(t, brandID, fmt.Sprintf("%s/%s", name, seed.BrandName))
		payload := map[string]any{
			"brand_id":    brandID,
			"consumed_at": seed.ConsumedAt.Format(time.RFC3339),
			"bags":        seed.Bags,
			"notes":       seed.Notes,
		}
		resp := mustDo(t, client, http.MethodPost, fmt.Sprintf("http://%s/api/consommations", addr), payload, name)
		assert.Equal(t, http.StatusCreated, resp.StatusCode, name)
		resp.Body.Close()
	}
}

func verifyAPIState(t *testing.T, client *http.Client, addr string, tc testCase) {
	t.Helper()

	resp := mustDo(t, client, http.MethodGet, fmt.Sprintf("http://%s/api/stats", addr), nil, tc.name)
	assert.Equal(t, http.StatusOK, resp.StatusCode, tc.name)
	var stats struct {
		Invested  core.Money `json:"investi_cents"`
		Consumed  core.Money `json:"consomme_cents"`
		Average   core.Money `json:"cout_moyen_par_sac_cents"`
		Inventory struct {
			TotalBags     int        `json:"total_bags"`
			TotalWeightKg float64    `json:"total_weight_kg"`
			TotalCost     core.Money `json:"total_cost_cents"`
		} `json:"inventaire"`
		Monthly []struct {
			Month time.Time `json:"month"`
			Bags  int       `json:"bags"`
		} `json:"sacs_par_mois"`
	}
	decodeJSONBody(t, resp, &stats, tc.name)

	assert.Equal(t, tc.want.invested, stats.Invested, tc.name)
	assert.Equal(t, tc.want.consumed, stats.Consumed, tc.name)
	assert.Equal(t, tc.want.inventoryBags, stats.Inventory.TotalBags, tc.name)
	assert.InDelta(t, tc.want.inventoryWeight, stats.Inventory.TotalWeightKg, 1e-6, tc.name)
	assert.Equal(t, tc.want.inventoryCost, stats.Inventory.TotalCost, tc.name)
	assert.Equal(t, tc.want.averageCost, stats.Average, tc.name)

	if tc.want.monthlyBags == 0 {
		assert.Equal(t, 0, len(stats.Monthly), tc.name)
	} else {
		require.Equal(t, 1, len(stats.Monthly), tc.name)
		assert.Equal(t, tc.want.monthlyBags, stats.Monthly[0].Bags, tc.name)
	}

	purchasesResp := mustDo(t, client, http.MethodGet, fmt.Sprintf("http://%s/api/achats", addr), nil, tc.name)
	assert.Equal(t, http.StatusOK, purchasesResp.StatusCode, tc.name)
	var purchases []map[string]any
	decodeJSONBody(t, purchasesResp, &purchases, tc.name)
	assert.Equal(t, len(tc.params.purchases), len(purchases), tc.name)

	consumptionsResp := mustDo(t, client, http.MethodGet, fmt.Sprintf("http://%s/api/consommations", addr), nil, tc.name)
	assert.Equal(t, http.StatusOK, consumptionsResp.StatusCode, tc.name)
	var consumptions []map[string]any
	decodeJSONBody(t, consumptionsResp, &consumptions, tc.name)
	assert.Equal(t, len(tc.params.consumptions), len(consumptions), tc.name)
}

func verifyPages(t *testing.T, client *http.Client, addr string, tc testCase) {
	t.Helper()
	home := fetchHTML(t, client, fmt.Sprintf("http://%s/", addr), tc.name)
	for _, snippet := range tc.want.homeSnippets {
		assert.Contains(t, home, snippet, tc.name)
	}

	statsPage := fetchHTML(t, client, fmt.Sprintf("http://%s/stats", addr), tc.name)
	for _, snippet := range tc.want.statsSnippets {
		assert.Contains(t, statsPage, snippet, tc.name)
	}

	if len(tc.want.consumptionSnippets) > 0 {
		consumptionsPage := fetchHTML(t, client, fmt.Sprintf("http://%s/consommations", addr), tc.name)
		for _, snippet := range tc.want.consumptionSnippets {
			assert.Contains(t, consumptionsPage, snippet, tc.name)
		}
	}
}

func launchServer(t *testing.T, env map[string]string, caseName string) (*http.Client, func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, binaryPath)
	cmdEnv := os.Environ()
	for k, v := range env {
		cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = cmdEnv

	var buffer bytes.Buffer
	cmd.Stdout = &buffer
	cmd.Stderr = &buffer

	require.NoError(t, cmd.Start(), "%s: start server", caseName)

	baseURL := fmt.Sprintf("http://%s", env["PELLETS_LISTEN_ADDR"])
	waitForReady(t, baseURL, caseName)

	client := &http.Client{Timeout: 5 * time.Second}

	cleanup := func() {
		cancel()
		done := make(chan struct{})
		go func() {
			_ = cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
		}
		if t.Failed() {
			t.Logf("%s: server output:\n%s", caseName, buffer.String())
		}
	}

	return client, cleanup
}

func waitForReady(t *testing.T, baseURL, caseName string) {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("%s: server did not become ready at %s", caseName, baseURL)
}

func mustDo(t *testing.T, client *http.Client, method, url string, payload any, msg string) *http.Response {
	t.Helper()
	var body io.Reader
	if payload != nil {
		buf := &bytes.Buffer{}
		require.NoError(t, json.NewEncoder(buf).Encode(payload), "%s: encode payload", msg)
		body = buf
	}
	req, err := http.NewRequest(method, url, body)
	require.NoError(t, err, "%s: build request", msg)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	require.NoError(t, err, "%s: execute request", msg)
	return resp
}

func decodeJSONBody[T any](t *testing.T, resp *http.Response, target *T, msg string) {
	t.Helper()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "%s: read response body", msg)
	require.NoError(t, json.Unmarshal(data, target), "%s: decode json body", msg)
}

func fetchHTML(t *testing.T, client *http.Client, url, caseName string) string {
	t.Helper()
	resp := mustDo(t, client, http.MethodGet, url, nil, caseName)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, caseName)
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "%s: read html body", caseName)
	return string(data)
}

func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "reserve port")
	addr := ln.Addr().(*net.TCPAddr)
	require.NoError(t, ln.Close(), "close probe listener")
	return fmt.Sprintf("127.0.0.1:%d", addr.Port)
}
