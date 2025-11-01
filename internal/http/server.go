// Package http exposes the REST API handlers and HTML views for the pellets tracker server.
package http

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"image"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "image/gif"
	_ "image/png"

	"github.com/disintegration/imaging"

	"pellets-tracker/internal/core"
)

// DataStore defines the persistence contract required by the HTTP server.
type DataStore interface {
	Data() core.DataStore
	Replace(core.DataStore) error
}

// Server exposes the HTTP API for the pellets tracker application.
type Server struct {
	store              DataStore
	mux                *http.ServeMux
	templates          map[string]*template.Template
	maxBrandImageBytes int64
}

// Config holds customization knobs for the HTTP server.
type Config struct {
	MaxBrandImageBytes int64
}

const (
	defaultMaxBrandImageBytes = 5 * 1024 * 1024
	brandImageTargetWidth     = 800
	// brandImageRequestOverhead compensates for multipart boundaries and additional form fields.
	// Without it a request containing an image close to the byte limit would be rejected before
	// we have a chance to validate or resize it.
	brandImageRequestOverhead = 512 * 1024
)

var (
	errBrandImageTooLarge = errors.New("brand image too large")
	errBrandImageInvalid  = errors.New("invalid brand image")
)

// NewServer constructs a Server backed by the provided datastore.
func NewServer(store DataStore, cfg Config) *Server {
	if store == nil {
		panic("nil datastore")
	}
	if cfg.MaxBrandImageBytes <= 0 {
		cfg.MaxBrandImageBytes = defaultMaxBrandImageBytes
	}
	s := &Server{
		store:              store,
		mux:                http.NewServeMux(),
		templates:          newTemplateSet(),
		maxBrandImageBytes: cfg.MaxBrandImageBytes,
	}
	s.registerRoutes()
	return s
}

// Handler returns the root HTTP handler with middleware attached.
func (s *Server) Handler() http.Handler {
	return s.loggingMiddleware(s.gzipMiddleware(s.mux))
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/healthz", s.handleHealthz)
	s.mux.Handle("/static/", http.StripPrefix("/static/", staticFileServer()))
	s.mux.HandleFunc("/", s.handleHome)
	s.mux.HandleFunc("/marques", s.handleBrandsPage)
	s.mux.HandleFunc("/consommations", s.handleConsumptionsPage)
	s.mux.HandleFunc("/stats", s.handleStatsPage)

	s.mux.HandleFunc("/api/marques", s.handleBrandsAPI)
	s.mux.HandleFunc("/api/achats", s.handlePurchasesAPI)
	s.mux.HandleFunc("/api/achats/", s.handlePurchaseByIDAPI)
	s.mux.HandleFunc("/api/consommations", s.handleConsumptionsAPI)
	s.mux.HandleFunc("/api/consommations/", s.handleConsumptionByIDAPI)
	s.mux.HandleFunc("/api/stats", s.handleStatsAPI)
	s.mux.HandleFunc("/api/export/", s.handleExport)
}

func (s *Server) renderPage(w http.ResponseWriter, templateName, title, active string, data any, flash *flashMessage) {
	if s.templates == nil {
		http.Error(w, "templates not initialized", http.StatusInternalServerError)
		return
	}
	tmpl, ok := s.templates[templateName]
	if !ok {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	payload := pageData{
		Title:     title,
		ActiveNav: active,
		Flash:     flash,
		Data:      data,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, templateName, payload); err != nil {
		log.Printf("render template %s: %v", templateName, err)
		http.Error(w, "template rendering error", http.StatusInternalServerError)
	}
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

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.renderHomePage(w, s.successFlash(r, "purchase", "Achat enregistré avec succès"))
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			s.renderHomePage(w, &flashMessage{Kind: "error", Message: "Formulaire invalide"})
			return
		}
		ds := s.store.Data()
		purchasedAt, err := parseDateOnly(r.FormValue("purchased_at"))
		if err != nil {
			s.renderHomePage(w, &flashMessage{Kind: "error", Message: "Date d'achat invalide"})
			return
		}
		bags, err := parseIntField(r.FormValue("bags"))
		if err != nil {
			s.renderHomePage(w, &flashMessage{Kind: "error", Message: "Nombre de sacs invalide"})
			return
		}
		bagWeightKg, err := parseFloatField(r.FormValue("bag_weight_kg"))
		if err != nil {
			s.renderHomePage(w, &flashMessage{Kind: "error", Message: "Poids par sac invalide"})
			return
		}
		unitPrice, err := parseMoneyField(r.FormValue("unit_price_eur"))
		if err != nil {
			s.renderHomePage(w, &flashMessage{Kind: "error", Message: err.Error()})
			return
		}

		purchase, err := core.AddPurchase(&ds, core.CreatePurchaseParams{
			BrandID:     core.ID(strings.TrimSpace(r.FormValue("brand_id"))),
			PurchasedAt: purchasedAt,
			Bags:        bags,
			BagWeightKg: bagWeightKg,
			UnitPrice:   unitPrice,
			Notes:       strings.TrimSpace(r.FormValue("notes")),
		})
		if err != nil {
			s.renderHomePage(w, &flashMessage{Kind: "error", Message: s.friendlyError(err)})
			return
		}
		if err := s.store.Replace(ds); err != nil {
			log.Printf("persist purchase form: %v", err)
			s.renderHomePage(w, &flashMessage{Kind: "error", Message: "Impossible d'enregistrer l'achat"})
			return
		}
		log.Printf(`{"type":"save","entity":"purchase","id":"%s"}`, purchase.ID)
		http.Redirect(w, r, "/?added=purchase", http.StatusSeeOther)
	default:
		s.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleBrandsPage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.renderBrandsPage(w, s.successFlash(r, "brand", "Marque enregistrée"))
	case http.MethodPost:
		maxBytes := s.effectiveMaxBrandImageBytes()
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes+brandImageRequestOverhead)
		if err := r.ParseMultipartForm(maxBytes); err != nil {
			if errors.Is(err, http.ErrNotMultipart) {
				if err := r.ParseForm(); err != nil {
					s.renderBrandsPage(w, &flashMessage{Kind: "error", Message: "Formulaire invalide"})
					return
				}
			} else {
				w.WriteHeader(http.StatusBadRequest)
				s.renderBrandsPage(w, &flashMessage{Kind: "error", Message: "Fichier trop volumineux ou invalide"})
				return
			}
		}

		imageBase64, err := s.brandImageFromRequest(r)
		if err != nil {
			message := "Impossible de traiter l'image"
			switch {
			case errors.Is(err, errBrandImageTooLarge):
				message = fmt.Sprintf("Image trop volumineuse (max %d Mo)", maxBytes/(1024*1024))
			case errors.Is(err, errBrandImageInvalid):
				message = "Format d'image non reconnu"
			case errors.Is(err, http.ErrMissingFile):
				message = "Téléversement de fichier invalide"
			}
			w.WriteHeader(http.StatusBadRequest)
			s.renderBrandsPage(w, &flashMessage{Kind: "error", Message: message})
			return
		}

		ds := s.store.Data()
		brand, err := core.AddBrand(&ds, core.CreateBrandParams{
			Name:        r.FormValue("name"),
			Description: r.FormValue("description"),
			ImageBase64: imageBase64,
		})
		if err != nil {
			s.renderBrandsPage(w, &flashMessage{Kind: "error", Message: s.friendlyError(err)})
			return
		}
		if err := s.store.Replace(ds); err != nil {
			log.Printf("persist brand form: %v", err)
			s.renderBrandsPage(w, &flashMessage{Kind: "error", Message: "Impossible d'enregistrer la marque"})
			return
		}
		log.Printf(`{"type":"save","entity":"brand","id":"%s"}`, brand.ID)
		http.Redirect(w, r, "/marques?added=brand", http.StatusSeeOther)
	default:
		s.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) encodeBrandImage(r io.Reader) (string, error) {
	maxBytes := s.effectiveMaxBrandImageBytes()

	limit := maxBytes + 1
	data, err := io.ReadAll(io.LimitReader(r, limit))
	if err != nil {
		return "", fmt.Errorf("read brand image: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return "", errBrandImageTooLarge
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("%w: %v", errBrandImageInvalid, err)
	}

	if img.Bounds().Dx() > brandImageTargetWidth {
		img = imaging.Resize(img, brandImageTargetWidth, 0, imaging.Lanczos)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return "", fmt.Errorf("encode brand image: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func (s *Server) brandImageFromRequest(r *http.Request) (string, error) {
	file, _, err := r.FormFile("image_file")
	switch {
	case err == nil:
		defer file.Close()
		return s.encodeBrandImage(file)
	case errors.Is(err, http.ErrMissingFile):
		return "", nil
	default:
		return "", err
	}
}

func (s *Server) effectiveMaxBrandImageBytes() int64 {
	if s.maxBrandImageBytes <= 0 {
		return defaultMaxBrandImageBytes
	}
	return s.maxBrandImageBytes
}

func (s *Server) handleConsumptionsPage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.renderConsumptionsPage(w, s.successFlash(r, "consumption", "Consommation enregistrée"))
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			s.renderConsumptionsPage(w, &flashMessage{Kind: "error", Message: "Formulaire invalide"})
			return
		}
		ds := s.store.Data()
		consumedAt, err := parseDateOnly(r.FormValue("consumed_at"))
		if err != nil {
			s.renderConsumptionsPage(w, &flashMessage{Kind: "error", Message: "Date invalide"})
			return
		}
		bags, err := parseIntField(r.FormValue("bags"))
		if err != nil {
			s.renderConsumptionsPage(w, &flashMessage{Kind: "error", Message: "Nombre de sacs invalide"})
			return
		}
		consumption, err := core.AddConsumption(&ds, core.CreateConsumptionParams{
			BrandID:    core.ID(strings.TrimSpace(r.FormValue("brand_id"))),
			ConsumedAt: consumedAt,
			Bags:       bags,
			Notes:      strings.TrimSpace(r.FormValue("notes")),
		})
		if err != nil {
			s.renderConsumptionsPage(w, &flashMessage{Kind: "error", Message: s.friendlyError(err)})
			return
		}
		if err := s.store.Replace(ds); err != nil {
			log.Printf("persist consumption form: %v", err)
			s.renderConsumptionsPage(w, &flashMessage{Kind: "error", Message: "Impossible d'enregistrer la consommation"})
			return
		}
		log.Printf(`{"type":"save","entity":"consumption","id":"%s"}`, consumption.ID)
		http.Redirect(w, r, "/consommations?added=consumption", http.StatusSeeOther)
	default:
		s.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleStatsPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, http.MethodGet)
		return
	}
	ds := s.store.Data()
	from, to, err := parseRangeQuery(r)
	if err != nil {
		s.renderPage(w, "stats", "Statistiques", "stats", statsView{}, &flashMessage{Kind: "error", Message: err.Error()})
		return
	}
	invested := core.ComputeInvesti(&ds, from, to)
	consumed, details, err := core.ComputeConsoValue(&ds, from, to)
	if err != nil {
		s.renderPage(w, "stats", "Statistiques", "stats", statsView{}, &flashMessage{Kind: "error", Message: s.friendlyError(err)})
		return
	}
	inventory, err := core.ComputeInventaire(&ds)
	if err != nil {
		s.renderPage(w, "stats", "Statistiques", "stats", statsView{}, &flashMessage{Kind: "error", Message: s.friendlyError(err)})
		return
	}
	monthly, err := core.ComputeSacsParMois(&ds, from, to)
	if err != nil {
		s.renderPage(w, "stats", "Statistiques", "stats", statsView{}, &flashMessage{Kind: "error", Message: s.friendlyError(err)})
		return
	}
	avg, err := core.ComputeCoutMoyenParSac(&ds, from, to)
	if err != nil {
		s.renderPage(w, "stats", "Statistiques", "stats", statsView{}, &flashMessage{Kind: "error", Message: s.friendlyError(err)})
		return
	}
	view := newStatsView(&ds, invested, consumed, avg, monthly, inventory, details)
	s.renderPage(w, "stats", "Statistiques", "stats", view, nil)
}

func (s *Server) renderHomePage(w http.ResponseWriter, flash *flashMessage) {
	ds := s.store.Data()
	view := newHomeView(&ds)
	s.renderPage(w, "home", "Achats", "purchases", view, flash)
}

func (s *Server) renderBrandsPage(w http.ResponseWriter, flash *flashMessage) {
	ds := s.store.Data()
	view := newBrandsView(&ds)
	s.renderPage(w, "brands", "Marques", "brands", view, flash)
}

func (s *Server) renderConsumptionsPage(w http.ResponseWriter, flash *flashMessage) {
	ds := s.store.Data()
	view := newConsumptionsView(&ds)
	s.renderPage(w, "consumptions", "Consommations", "consumptions", view, flash)
}

func (s *Server) successFlash(r *http.Request, expected, message string) *flashMessage {
	if r.URL.Query().Get("added") == expected {
		return &flashMessage{Kind: "success", Message: message}
	}
	return nil
}

func (s *Server) friendlyError(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, core.ErrBrandNotFound):
		return "Marque introuvable"
	case errors.Is(err, core.ErrBrandInUse):
		return "La marque est référencée, impossible de la supprimer"
	case errors.Is(err, core.ErrInsufficientInventory):
		return "Inventaire insuffisant pour cette opération"
	default:
		var vErr core.ValidationErrors
		if errors.As(err, &vErr) {
			return vErr.Error()
		}
		return err.Error()
	}
}

func (s *Server) handleBrandsAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listBrands(w, r)
	case http.MethodPost:
		s.createBrand(w, r)
	default:
		s.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handlePurchasesAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listPurchases(w, r)
	case http.MethodPost:
		s.createPurchase(w, r)
	default:
		s.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handlePurchaseByIDAPI(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) handleConsumptionsAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listConsumptions(w, r)
	case http.MethodPost:
		s.createConsumption(w, r)
	default:
		s.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleConsumptionByIDAPI(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) handleStatsAPI(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) listBrands(w http.ResponseWriter, _ *http.Request) {
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

func (s *Server) listPurchases(w http.ResponseWriter, _ *http.Request) {
	ds := s.store.Data()
	s.writeJSON(w, http.StatusOK, ds.Purchases)
}

type purchasePayload struct {
	BrandID     core.ID `json:"brand_id"`
	PurchasedAt string  `json:"purchased_at"`
	Bags        int     `json:"bags"`
	BagWeightKg float64 `json:"bag_weight_kg"`
	WeightKg    float64 `json:"weight_kg"`
	UnitPrice   int64   `json:"unit_price_cents"`
	Notes       string  `json:"notes"`
}

func (p purchasePayload) effectiveBagWeight() float64 {
	if p.BagWeightKg > 0 {
		return p.BagWeightKg
	}
	if p.WeightKg > 0 && p.Bags > 0 {
		return p.WeightKg / float64(p.Bags)
	}
	return 0
}

func parseIntField(value string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(value))
}

func parseFloatField(value string) (float64, error) {
	cleaned := strings.ReplaceAll(strings.TrimSpace(value), ",", ".")
	return strconv.ParseFloat(cleaned, 64)
}

func parseMoneyField(value string) (core.Money, error) {
	cleaned := strings.ReplaceAll(strings.TrimSpace(value), ",", ".")
	if cleaned == "" {
		return 0, fmt.Errorf("prix unitaire requis")
	}
	amount, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, fmt.Errorf("prix invalide: %w", err)
	}
	if amount < 0 {
		return 0, errors.New("le prix ne peut pas être négatif")
	}
	return core.ParseMoney(amount), nil
}

func parseDateOnly(value string) (time.Time, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return time.Time{}, fmt.Errorf("date requise")
	}
	t, err := time.ParseInLocation("2006-01-02", v, time.UTC)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
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
		BagWeightKg: payload.effectiveBagWeight(),
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
		BagWeightKg: payload.effectiveBagWeight(),
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

func (s *Server) deletePurchase(w http.ResponseWriter, _ *http.Request, id core.ID) {
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

func (s *Server) listConsumptions(w http.ResponseWriter, _ *http.Request) {
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

func (s *Server) deleteConsumption(w http.ResponseWriter, _ *http.Request, id core.ID) {
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

func (s *Server) exportJSON(w http.ResponseWriter, _ *http.Request) {
	ds := s.store.Data()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=pellets-datastore.json")
	if err := json.NewEncoder(w).Encode(ds); err != nil {
		log.Printf("export json: %v", err)
	}
}

func (s *Server) exportCSV(w http.ResponseWriter, _ *http.Request) {
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
			formatFloat(purchase.TotalWeightKg),
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
