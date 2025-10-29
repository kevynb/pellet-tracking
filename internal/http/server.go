package http

import (
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"pellets-tracker/internal/core"
	"pellets-tracker/internal/store"
)

// Server exposes the HTTP API for the pellets tracker application.
type Server struct {
	store *store.JSONStore
	mux   *http.ServeMux
}

// NewServer constructs a Server backed by the provided datastore.
func NewServer(store *store.JSONStore) *Server {
	if store == nil {
		panic("nil JSONStore")
	}
	s := &Server{store: store, mux: http.NewServeMux()}
	s.registerRoutes()
	return s
}

// Handler returns the root HTTP handler with middleware attached.
func (s *Server) Handler() http.Handler {
	return s.loggingMiddleware(s.gzipMiddleware(s.mux))
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/healthz", s.handleHealthz)
	s.mux.HandleFunc("/api/marques", s.handleBrands)
	s.mux.HandleFunc("/api/achats", s.handlePurchases)
	s.mux.HandleFunc("/api/achats/", s.handlePurchaseByID)
	s.mux.HandleFunc("/api/consommations", s.handleConsumptions)
	s.mux.HandleFunc("/api/consommations/", s.handleConsumptionByID)
	s.mux.HandleFunc("/api/stats", s.handleStats)
	s.mux.HandleFunc("/api/export/", s.handleExport)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, http.MethodGet)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, `{"status":"ok"}`)
}

func (s *Server) handleBrands(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listBrands(w, r)
	case http.MethodPost:
		s.createBrand(w, r)
	default:
		s.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handlePurchases(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listPurchases(w, r)
	case http.MethodPost:
		s.createPurchase(w, r)
	default:
		s.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handlePurchaseByID(w http.ResponseWriter, r *http.Request) {
	id := core.ID(strings.TrimPrefix(r.URL.Path, "/api/achats/"))
	if id == "" || strings.ContainsRune(string(id), '/') {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodPut:
		s.updatePurchase(w, r, id)
	case http.MethodDelete:
		s.deletePurchase(w, r, id)
	default:
		s.methodNotAllowed(w, http.MethodPut, http.MethodDelete)
	}
}

func (s *Server) handleConsumptions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listConsumptions(w, r)
	case http.MethodPost:
		s.createConsumption(w, r)
	default:
		s.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleConsumptionByID(w http.ResponseWriter, r *http.Request) {
	id := core.ID(strings.TrimPrefix(r.URL.Path, "/api/consommations/"))
	if id == "" || strings.ContainsRune(string(id), '/') {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodPut:
		s.updateConsumption(w, r, id)
	case http.MethodDelete:
		s.deleteConsumption(w, r, id)
	default:
		s.methodNotAllowed(w, http.MethodPut, http.MethodDelete)
	}
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, http.MethodGet)
		return
	}
	ds := s.store.Data()
	from, to, err := parseRangeQuery(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	invested := core.ComputeInvesti(&ds, from, to)
	consumed, details, err := core.ComputeConsoValue(&ds, from, to)
	if err != nil {
		s.handleCoreError(w, err)
		return
	}
	inventory, err := core.ComputeInventaire(&ds)
	if err != nil {
		s.handleCoreError(w, err)
		return
	}
	monthly, err := core.ComputeSacsParMois(&ds, from, to)
	if err != nil {
		s.handleCoreError(w, err)
		return
	}
	avg, err := core.ComputeCoutMoyenParSac(&ds, from, to)
	if err != nil {
		s.handleCoreError(w, err)
		return
	}

	response := map[string]any{
		"investi_cents":            invested,
		"consomme_cents":           consumed,
		"consommations_detail":     details,
		"inventaire":               inventory,
		"sacs_par_mois":            monthly,
		"cout_moyen_par_sac_cents": avg,
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, http.MethodGet)
		return
	}
	format := strings.TrimPrefix(r.URL.Path, "/api/export/")
	switch format {
	case "json":
		s.exportJSON(w, r)
	case "csv":
		s.exportCSV(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) listBrands(w http.ResponseWriter, r *http.Request) {
	ds := s.store.Data()
	brands := ds.Brands
	sort.Slice(brands, func(i, j int) bool {
		return strings.ToLower(brands[i].Name) < strings.ToLower(brands[j].Name)
	})
	s.writeJSON(w, http.StatusOK, brands)
}

type brandPayload struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ImageBase64 string `json:"image_base64"`
}

func (s *Server) createBrand(w http.ResponseWriter, r *http.Request) {
	var payload brandPayload
	if err := decodeJSON(r.Body, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	ds := s.store.Data()
	brand, err := core.AddBrand(&ds, core.CreateBrandParams{
		Name:        payload.Name,
		Description: payload.Description,
		ImageBase64: payload.ImageBase64,
	})
	if err != nil {
		s.handleCoreError(w, err)
		return
	}
	if err := s.store.Replace(ds); err != nil {
		s.handleStoreError(w, err)
		return
	}
	log.Printf(`{"type":"save","entity":"brand","id":"%s"}`, brand.ID)
	s.writeJSON(w, http.StatusCreated, brand)
}

func (s *Server) listPurchases(w http.ResponseWriter, r *http.Request) {
	ds := s.store.Data()
	s.writeJSON(w, http.StatusOK, ds.Purchases)
}

type purchasePayload struct {
	BrandID     core.ID `json:"brand_id"`
	PurchasedAt string  `json:"purchased_at"`
	Bags        int     `json:"bags"`
	WeightKg    float64 `json:"weight_kg"`
	UnitPrice   int64   `json:"unit_price_cents"`
	Notes       string  `json:"notes"`
}

func parseTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, value)
}

func (s *Server) createPurchase(w http.ResponseWriter, r *http.Request) {
	var payload purchasePayload
	if err := decodeJSON(r.Body, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	purchasedAt, err := parseTime(payload.PurchasedAt)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	ds := s.store.Data()
	purchase, err := core.AddPurchase(&ds, core.CreatePurchaseParams{
		BrandID:     payload.BrandID,
		PurchasedAt: purchasedAt,
		Bags:        payload.Bags,
		WeightKg:    payload.WeightKg,
		UnitPrice:   core.Money(payload.UnitPrice),
		Notes:       payload.Notes,
	})
	if err != nil {
		s.handleCoreError(w, err)
		return
	}
	if err := s.store.Replace(ds); err != nil {
		s.handleStoreError(w, err)
		return
	}
	log.Printf(`{"type":"save","entity":"purchase","id":"%s"}`, purchase.ID)
	s.writeJSON(w, http.StatusCreated, purchase)
}

func (s *Server) updatePurchase(w http.ResponseWriter, r *http.Request, id core.ID) {
	var payload purchasePayload
	if err := decodeJSON(r.Body, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	purchasedAt, err := parseTime(payload.PurchasedAt)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	ds := s.store.Data()
	purchase, err := core.UpdatePurchase(&ds, id, core.UpdatePurchaseParams{
		PurchasedAt: purchasedAt,
		Bags:        payload.Bags,
		WeightKg:    payload.WeightKg,
		UnitPrice:   core.Money(payload.UnitPrice),
		Notes:       payload.Notes,
	})
	if err != nil {
		s.handleCoreError(w, err)
		return
	}
	if err := s.store.Replace(ds); err != nil {
		s.handleStoreError(w, err)
		return
	}
	log.Printf(`{"type":"save","entity":"purchase","id":"%s"}`, purchase.ID)
	s.writeJSON(w, http.StatusOK, purchase)
}

func (s *Server) deletePurchase(w http.ResponseWriter, r *http.Request, id core.ID) {
	ds := s.store.Data()
	if err := core.DeletePurchase(&ds, id); err != nil {
		s.handleCoreError(w, err)
		return
	}
	if err := s.store.Replace(ds); err != nil {
		s.handleStoreError(w, err)
		return
	}
	log.Printf(`{"type":"save","entity":"purchase","id":"%s","action":"delete"}`, id)
	w.WriteHeader(http.StatusNoContent)
}

type consumptionPayload struct {
	BrandID    core.ID `json:"brand_id"`
	ConsumedAt string  `json:"consumed_at"`
	Bags       int     `json:"bags"`
	Notes      string  `json:"notes"`
}

func (s *Server) listConsumptions(w http.ResponseWriter, r *http.Request) {
	ds := s.store.Data()
	s.writeJSON(w, http.StatusOK, ds.Consumptions)
}

func (s *Server) createConsumption(w http.ResponseWriter, r *http.Request) {
	var payload consumptionPayload
	if err := decodeJSON(r.Body, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	consumedAt, err := parseTime(payload.ConsumedAt)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	ds := s.store.Data()
	consumption, err := core.AddConsumption(&ds, core.CreateConsumptionParams{
		BrandID:    payload.BrandID,
		ConsumedAt: consumedAt,
		Bags:       payload.Bags,
		Notes:      payload.Notes,
	})
	if err != nil {
		s.handleCoreError(w, err)
		return
	}
	if err := s.store.Replace(ds); err != nil {
		s.handleStoreError(w, err)
		return
	}
	log.Printf(`{"type":"save","entity":"consumption","id":"%s"}`, consumption.ID)
	s.writeJSON(w, http.StatusCreated, consumption)
}

func (s *Server) updateConsumption(w http.ResponseWriter, r *http.Request, id core.ID) {
	var payload consumptionPayload
	if err := decodeJSON(r.Body, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	consumedAt, err := parseTime(payload.ConsumedAt)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	ds := s.store.Data()
	consumption, err := core.UpdateConsumption(&ds, id, core.UpdateConsumptionParams{
		ConsumedAt: consumedAt,
		Bags:       payload.Bags,
		Notes:      payload.Notes,
	})
	if err != nil {
		s.handleCoreError(w, err)
		return
	}
	if err := s.store.Replace(ds); err != nil {
		s.handleStoreError(w, err)
		return
	}
	log.Printf(`{"type":"save","entity":"consumption","id":"%s"}`, consumption.ID)
	s.writeJSON(w, http.StatusOK, consumption)
}

func (s *Server) deleteConsumption(w http.ResponseWriter, r *http.Request, id core.ID) {
	ds := s.store.Data()
	if err := core.DeleteConsumption(&ds, id); err != nil {
		s.handleCoreError(w, err)
		return
	}
	if err := s.store.Replace(ds); err != nil {
		s.handleStoreError(w, err)
		return
	}
	log.Printf(`{"type":"save","entity":"consumption","id":"%s","action":"delete"}`, id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) exportJSON(w http.ResponseWriter, r *http.Request) {
	ds := s.store.Data()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=pellets-datastore.json")
	if err := json.NewEncoder(w).Encode(ds); err != nil {
		log.Printf("export json: %v", err)
	}
}

func (s *Server) exportCSV(w http.ResponseWriter, r *http.Request) {
	ds := s.store.Data()
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=pellets-export.csv")

	writer := csv.NewWriter(w)
	header := []string{"type", "id", "brand_id", "brand_name", "timestamp", "bags", "weight_kg", "unit_price_cents", "total_price_cents", "notes"}
	if err := writer.Write(header); err != nil {
		log.Printf("export csv header: %v", err)
		return
	}

	brandNames := make(map[core.ID]string, len(ds.Brands))
	for _, brand := range ds.Brands {
		brandNames[brand.ID] = brand.Name
	}

	for _, purchase := range ds.Purchases {
		record := []string{
			"purchase",
			string(purchase.ID),
			string(purchase.BrandID),
			brandNames[purchase.BrandID],
			purchase.PurchasedAt.Format(time.RFC3339),
			itoaInt(purchase.Bags),
			formatFloat(purchase.WeightKg),
			itoaMoney(purchase.UnitPriceCents),
			itoaMoney(purchase.TotalPriceCents),
			purchase.Notes,
		}
		if err := writer.Write(record); err != nil {
			log.Printf("export csv purchase: %v", err)
			return
		}
	}

	for _, consumption := range ds.Consumptions {
		record := []string{
			"consumption",
			string(consumption.ID),
			string(consumption.BrandID),
			brandNames[consumption.BrandID],
			consumption.ConsumedAt.Format(time.RFC3339),
			itoaInt(consumption.Bags),
			"",
			"",
			"",
			consumption.Notes,
		}
		if err := writer.Write(record); err != nil {
			log.Printf("export csv consumption: %v", err)
			return
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		log.Printf("export csv flush: %v", err)
	}
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lrw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(lrw, r)
		duration := time.Since(start)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, lrw.status, duration)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (s *Server) gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		gz := gzip.NewWriter(w)
		defer gz.Close()
		w.Header().Set("Content-Encoding", "gzip")
		grw := &gzipResponseWriter{ResponseWriter: w, Writer: gz}
		next.ServeHTTP(grw, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	*gzip.Writer
}

func (w *gzipResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	w.ResponseWriter.WriteHeader(status)
}

func decodeJSON(r io.Reader, dst any) error {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(new(struct{})); err != io.EOF {
		if err == nil {
			return errors.New("unexpected data after JSON payload")
		}
		return err
	}
	return nil
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write json: %v", err)
	}
}

func (s *Server) writeError(w http.ResponseWriter, status int, err error) {
	s.writeJSON(w, status, map[string]any{"error": err.Error()})
}

func (s *Server) methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	if len(allowed) > 0 {
		w.Header().Set("Allow", strings.Join(allowed, ","))
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleStoreError(w http.ResponseWriter, err error) {
	log.Printf("store error: %v", err)
	s.writeError(w, http.StatusInternalServerError, errors.New("failed to persist datastore"))
}

func (s *Server) handleCoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, core.ErrBrandNotFound), errors.Is(err, core.ErrPurchaseNotFound), errors.Is(err, core.ErrConsumptionNotFound):
		s.writeError(w, http.StatusNotFound, err)
	case errors.Is(err, core.ErrBrandInUse), errors.Is(err, core.ErrInsufficientInventory):
		s.writeError(w, http.StatusConflict, err)
	case isValidationError(err):
		s.writeValidationError(w, err)
	default:
		log.Printf("core error: %v", err)
		s.writeError(w, http.StatusInternalServerError, errors.New("internal error"))
	}
}

func isValidationError(err error) bool {
	var ve core.ValidationErrors
	return errors.As(err, &ve)
}

func (s *Server) writeValidationError(w http.ResponseWriter, err error) {
	var ve core.ValidationErrors
	if !errors.As(err, &ve) {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	payload := map[string]any{
		"error":   "validation failed",
		"details": ve,
	}
	s.writeJSON(w, http.StatusBadRequest, payload)
}

func parseRangeQuery(r *http.Request) (time.Time, time.Time, error) {
	var from, to time.Time
	var err error
	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	return from, to, nil
}

func itoaInt(value int) string {
	return strconv.FormatInt(int64(value), 10)
}

func itoaMoney(value core.Money) string {
	return strconv.FormatInt(int64(value), 10)
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
