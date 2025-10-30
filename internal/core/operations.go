package core

import (
	"errors"
	"sort"
	"strings"
	"time"
)

// CreateBrandParams captures the fields required to create a brand.
type CreateBrandParams struct {
	Name        string
	Description string
	ImageBase64 string
}

// UpdateBrandParams captures the mutable brand fields.
type UpdateBrandParams struct {
	Name        string
	Description string
	ImageBase64 string
}

// CreatePurchaseParams contains the data necessary to create a purchase entry.
type CreatePurchaseParams struct {
	BrandID     ID
	PurchasedAt time.Time
	Bags        int
	BagWeightKg float64
	UnitPrice   Money
	Notes       string
}

// UpdatePurchaseParams captures the mutable purchase fields.
type UpdatePurchaseParams struct {
	PurchasedAt time.Time
	Bags        int
	BagWeightKg float64
	UnitPrice   Money
	Notes       string
}

// CreateConsumptionParams contains the fields to create a consumption entry.
type CreateConsumptionParams struct {
	BrandID    ID
	ConsumedAt time.Time
	Bags       int
	Notes      string
}

// UpdateConsumptionParams captures mutable consumption fields.
type UpdateConsumptionParams struct {
	ConsumedAt time.Time
	Bags       int
	Notes      string
}

// AddBrand inserts a new brand into the datastore.
func AddBrand(ds *DataStore, params CreateBrandParams) (Brand, error) {
	if ds == nil {
		return Brand{}, errors.New("nil datastore")
	}

	name := NormalizeName(params.Name)
	errs := ValidationErrors{}
	errs = errs.AppendIf(name == "", "name", "name is required")
	if name != "" && hasBrandWithName(ds.Brands, name, "") {
		errs = errs.AppendIf(true, "name", "brand name already exists")
	}
	if len(errs) > 0 {
		return Brand{}, errs
	}

	now := time.Now().UTC()
	brand := Brand{
		Meta: Meta{
			ID:        NewID(),
			CreatedAt: now,
			UpdatedAt: now,
		},
		Name:        name,
		Description: strings.TrimSpace(params.Description),
		ImageBase64: strings.TrimSpace(params.ImageBase64),
	}

	ds.Brands = append(ds.Brands, brand)
	touchDatastore(ds, now)

	return brand, nil
}

// UpdateBrand mutates an existing brand.
func UpdateBrand(ds *DataStore, id ID, params UpdateBrandParams) (Brand, error) {
	if ds == nil {
		return Brand{}, errors.New("nil datastore")
	}

	idx := findBrandIndex(ds.Brands, id)
	if idx == -1 {
		return Brand{}, ErrBrandNotFound
	}

	name := NormalizeName(params.Name)
	errs := ValidationErrors{}
	errs = errs.AppendIf(name == "", "name", "name is required")
	if name != "" && hasBrandWithName(ds.Brands, name, id) {
		errs = errs.AppendIf(true, "name", "brand name already exists")
	}
	if len(errs) > 0 {
		return Brand{}, errs
	}

	now := time.Now().UTC()
	brand := ds.Brands[idx]
	brand.Name = name
	brand.Description = strings.TrimSpace(params.Description)
	brand.ImageBase64 = strings.TrimSpace(params.ImageBase64)
	brand.UpdatedAt = now
	ds.Brands[idx] = brand

	touchDatastore(ds, now)

	return brand, nil
}

// DeleteBrand removes a brand when no purchase or consumption references it.
func DeleteBrand(ds *DataStore, id ID) error {
	if ds == nil {
		return errors.New("nil datastore")
	}
	idx := findBrandIndex(ds.Brands, id)
	if idx == -1 {
		return ErrBrandNotFound
	}
	if brandReferenced(ds, id) {
		return ErrBrandInUse
	}

	ds.Brands = append(ds.Brands[:idx], ds.Brands[idx+1:]...)
	touchDatastore(ds, time.Now().UTC())
	return nil
}

// AddPurchase appends a new purchase entry with derived totals.
func AddPurchase(ds *DataStore, params CreatePurchaseParams) (Purchase, error) {
	if ds == nil {
		return Purchase{}, errors.New("nil datastore")
	}

	errs := validatePurchaseInput(ds, params.BrandID, params.Bags, params.BagWeightKg, params.UnitPrice, params.PurchasedAt)
	if len(errs) > 0 {
		return Purchase{}, errs
	}

	now := time.Now().UTC()
	purchasedAt := params.PurchasedAt
	if purchasedAt.IsZero() {
		purchasedAt = now
	}

	purchase := Purchase{
		Meta: Meta{
			ID:        NewID(),
			CreatedAt: now,
			UpdatedAt: now,
		},
		BrandID:         params.BrandID,
		PurchasedAt:     purchasedAt,
		Bags:            params.Bags,
		BagWeightKg:     params.BagWeightKg,
		TotalWeightKg:   params.BagWeightKg * float64(params.Bags),
		UnitPriceCents:  params.UnitPrice,
		TotalPriceCents: params.UnitPrice.MulInt(params.Bags),
		Notes:           strings.TrimSpace(params.Notes),
	}

	ds.Purchases = append(ds.Purchases, purchase)
	sort.Slice(ds.Purchases, func(i, j int) bool {
		if ds.Purchases[i].PurchasedAt.Equal(ds.Purchases[j].PurchasedAt) {
			return string(ds.Purchases[i].ID) > string(ds.Purchases[j].ID)
		}
		return ds.Purchases[i].PurchasedAt.After(ds.Purchases[j].PurchasedAt)
	})
	touchDatastore(ds, now)

	return purchase, nil
}

// UpdatePurchase mutates an existing purchase and recomputes totals.
func UpdatePurchase(ds *DataStore, id ID, params UpdatePurchaseParams) (Purchase, error) {
	if ds == nil {
		return Purchase{}, errors.New("nil datastore")
	}

	idx := findPurchaseIndex(ds.Purchases, id)
	if idx == -1 {
		return Purchase{}, ErrPurchaseNotFound
	}

	errs := validatePurchaseInput(ds, ds.Purchases[idx].BrandID, params.Bags, params.BagWeightKg, params.UnitPrice, params.PurchasedAt)
	if len(errs) > 0 {
		return Purchase{}, errs
	}

	now := time.Now().UTC()
	purchasedAt := params.PurchasedAt
	if purchasedAt.IsZero() {
		purchasedAt = ds.Purchases[idx].PurchasedAt
	}

	purchase := ds.Purchases[idx]
	purchase.PurchasedAt = purchasedAt
	purchase.Bags = params.Bags
	purchase.BagWeightKg = params.BagWeightKg
	purchase.TotalWeightKg = params.BagWeightKg * float64(params.Bags)
	purchase.UnitPriceCents = params.UnitPrice
	purchase.TotalPriceCents = params.UnitPrice.MulInt(params.Bags)
	purchase.Notes = strings.TrimSpace(params.Notes)
	purchase.UpdatedAt = now
	ds.Purchases[idx] = purchase

	sort.Slice(ds.Purchases, func(i, j int) bool {
		if ds.Purchases[i].PurchasedAt.Equal(ds.Purchases[j].PurchasedAt) {
			return string(ds.Purchases[i].ID) > string(ds.Purchases[j].ID)
		}
		return ds.Purchases[i].PurchasedAt.After(ds.Purchases[j].PurchasedAt)
	})
	touchDatastore(ds, now)

	return purchase, nil
}

// DeletePurchase removes a purchase.
func DeletePurchase(ds *DataStore, id ID) error {
	if ds == nil {
		return errors.New("nil datastore")
	}
	idx := findPurchaseIndex(ds.Purchases, id)
	if idx == -1 {
		return ErrPurchaseNotFound
	}
	ds.Purchases = append(ds.Purchases[:idx], ds.Purchases[idx+1:]...)
	touchDatastore(ds, time.Now().UTC())
	return nil
}

// AddConsumption creates a new consumption entry.
func AddConsumption(ds *DataStore, params CreateConsumptionParams) (Consumption, error) {
	if ds == nil {
		return Consumption{}, errors.New("nil datastore")
	}

	errs := validateConsumptionInput(ds, params.BrandID, params.Bags, params.ConsumedAt)
	if len(errs) > 0 {
		return Consumption{}, errs
	}

	now := time.Now().UTC()
	consumedAt := params.ConsumedAt
	if consumedAt.IsZero() {
		consumedAt = now
	}

	consumption := Consumption{
		Meta: Meta{
			ID:        NewID(),
			CreatedAt: now,
			UpdatedAt: now,
		},
		BrandID:    params.BrandID,
		ConsumedAt: consumedAt,
		Bags:       params.Bags,
		Notes:      strings.TrimSpace(params.Notes),
	}

	ds.Consumptions = append(ds.Consumptions, consumption)
	sort.Slice(ds.Consumptions, func(i, j int) bool {
		if ds.Consumptions[i].ConsumedAt.Equal(ds.Consumptions[j].ConsumedAt) {
			return string(ds.Consumptions[i].ID) > string(ds.Consumptions[j].ID)
		}
		return ds.Consumptions[i].ConsumedAt.After(ds.Consumptions[j].ConsumedAt)
	})
	touchDatastore(ds, now)

	return consumption, nil
}

// UpdateConsumption mutates an existing consumption entry.
func UpdateConsumption(ds *DataStore, id ID, params UpdateConsumptionParams) (Consumption, error) {
	if ds == nil {
		return Consumption{}, errors.New("nil datastore")
	}

	idx := findConsumptionIndex(ds.Consumptions, id)
	if idx == -1 {
		return Consumption{}, ErrConsumptionNotFound
	}

	errs := validateConsumptionInput(ds, ds.Consumptions[idx].BrandID, params.Bags, params.ConsumedAt)
	if len(errs) > 0 {
		return Consumption{}, errs
	}

	now := time.Now().UTC()
	consumedAt := params.ConsumedAt
	if consumedAt.IsZero() {
		consumedAt = ds.Consumptions[idx].ConsumedAt
	}

	consumption := ds.Consumptions[idx]
	consumption.ConsumedAt = consumedAt
	consumption.Bags = params.Bags
	consumption.Notes = strings.TrimSpace(params.Notes)
	consumption.UpdatedAt = now
	ds.Consumptions[idx] = consumption

	sort.Slice(ds.Consumptions, func(i, j int) bool {
		if ds.Consumptions[i].ConsumedAt.Equal(ds.Consumptions[j].ConsumedAt) {
			return string(ds.Consumptions[i].ID) > string(ds.Consumptions[j].ID)
		}
		return ds.Consumptions[i].ConsumedAt.After(ds.Consumptions[j].ConsumedAt)
	})
	touchDatastore(ds, now)

	return consumption, nil
}

// DeleteConsumption removes a consumption entry.
func DeleteConsumption(ds *DataStore, id ID) error {
	if ds == nil {
		return errors.New("nil datastore")
	}
	idx := findConsumptionIndex(ds.Consumptions, id)
	if idx == -1 {
		return ErrConsumptionNotFound
	}
	ds.Consumptions = append(ds.Consumptions[:idx], ds.Consumptions[idx+1:]...)
	touchDatastore(ds, time.Now().UTC())
	return nil
}

func validatePurchaseInput(ds *DataStore, brandID ID, bags int, bagWeightKg float64, unitPrice Money, purchasedAt time.Time) ValidationErrors {
	errs := ValidationErrors{}
	errs = errs.AppendIf(!brandExists(ds.Brands, brandID), "brand_id", "unknown brand")
	errs = errs.AppendIf(bags <= 0, "bags", "bags must be greater than zero")
	errs = errs.AppendIf(bagWeightKg <= 0, "bag_weight_kg", "bag weight must be greater than zero")
	errs = errs.AppendIf(unitPrice.Int64() < 0, "unit_price", "unit price cannot be negative")
	errs = errs.AppendIf(!purchasedAt.IsZero() && purchasedAt.After(time.Now().Add(24*time.Hour)), "purchased_at", "purchase date cannot be in the far future")
	return errs
}

func validateConsumptionInput(ds *DataStore, brandID ID, bags int, consumedAt time.Time) ValidationErrors {
	errs := ValidationErrors{}
	errs = errs.AppendIf(!brandExists(ds.Brands, brandID), "brand_id", "unknown brand")
	errs = errs.AppendIf(bags <= 0, "bags", "bags must be greater than zero")
	if !consumedAt.IsZero() {
		errs = errs.AppendIf(consumedAt.After(time.Now().Add(24*time.Hour)), "consumed_at", "consumption date cannot be in the far future")
	}
	return errs
}

func brandExists(brands []Brand, id ID) bool {
	for _, b := range brands {
		if b.ID == id {
			return true
		}
	}
	return false
}

func hasBrandWithName(brands []Brand, name string, exclude ID) bool {
	lowered := strings.ToLower(name)
	for _, b := range brands {
		if exclude != "" && b.ID == exclude {
			continue
		}
		if strings.ToLower(b.Name) == lowered {
			return true
		}
	}
	return false
}

func brandReferenced(ds *DataStore, id ID) bool {
	for _, p := range ds.Purchases {
		if p.BrandID == id {
			return true
		}
	}
	for _, c := range ds.Consumptions {
		if c.BrandID == id {
			return true
		}
	}
	return false
}

func findBrandIndex(brands []Brand, id ID) int {
	for i, b := range brands {
		if b.ID == id {
			return i
		}
	}
	return -1
}

func findPurchaseIndex(purchases []Purchase, id ID) int {
	for i, p := range purchases {
		if p.ID == id {
			return i
		}
	}
	return -1
}

func findConsumptionIndex(consumptions []Consumption, id ID) int {
	for i, c := range consumptions {
		if c.ID == id {
			return i
		}
	}
	return -1
}

func touchDatastore(ds *DataStore, ts time.Time) {
	if ds.ID == "" {
		ds.ID = NewID()
	}
	if ds.CreatedAt.IsZero() {
		ds.CreatedAt = ts
	}
	ds.UpdatedAt = ts
}
