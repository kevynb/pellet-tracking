package core

import (
	"errors"
	"testing"
	"time"
)

func TestAddBrand(t *testing.T) {
	ds := &DataStore{}
	brand, err := AddBrand(ds, CreateBrandParams{Name: "  Premium   Pellets  "})
	if err != nil {
		t.Fatalf("AddBrand returned error: %v", err)
	}
	if brand.Name != "Premium Pellets" {
		t.Fatalf("expected normalized name, got %q", brand.Name)
	}
	if len(ds.Brands) != 1 {
		t.Fatalf("expected 1 brand, got %d", len(ds.Brands))
	}
	if ds.Brands[0].ID != brand.ID {
		t.Fatal("brand not stored in datastore")
	}
	if ds.Meta.UpdatedAt.IsZero() {
		t.Fatal("datastore update timestamp not set")
	}
}

func TestAddBrandDuplicateName(t *testing.T) {
	ds := &DataStore{}
	if _, err := AddBrand(ds, CreateBrandParams{Name: "Pellets"}); err != nil {
		t.Fatalf("unexpected error adding brand: %v", err)
	}
	_, err := AddBrand(ds, CreateBrandParams{Name: "pellets"})
	if err == nil {
		t.Fatal("expected duplicate name error")
	}
	var vErr ValidationErrors
	if errorsAs(err, &vErr) {
		if !vErr.Has("name") {
			t.Fatalf("expected name error, got %v", vErr)
		}
	} else {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}
}

func TestAddPurchaseComputesTotal(t *testing.T) {
	ds := &DataStore{}
	brand, err := AddBrand(ds, CreateBrandParams{Name: "Test"})
	if err != nil {
		t.Fatalf("add brand: %v", err)
	}

	purchase, err := AddPurchase(ds, CreatePurchaseParams{
		BrandID:     brand.ID,
		PurchasedAt: time.Now().Add(-time.Hour),
		Bags:        5,
		UnitPrice:   Money(790),
	})
	if err != nil {
		t.Fatalf("AddPurchase error: %v", err)
	}
	if purchase.TotalPriceCents != Money(3950) {
		t.Fatalf("expected total 3950, got %d", purchase.TotalPriceCents)
	}
	if len(ds.Purchases) != 1 {
		t.Fatalf("expected 1 purchase, got %d", len(ds.Purchases))
	}
}

func TestUpdatePurchaseRecalculatesTotal(t *testing.T) {
	ds := &DataStore{}
	brand, _ := AddBrand(ds, CreateBrandParams{Name: "Brand"})
	purchase, _ := AddPurchase(ds, CreatePurchaseParams{
		BrandID:     brand.ID,
		PurchasedAt: time.Now(),
		Bags:        4,
		UnitPrice:   Money(600),
	})

	updated, err := UpdatePurchase(ds, purchase.ID, UpdatePurchaseParams{
		PurchasedAt: purchase.PurchasedAt,
		Bags:        6,
		UnitPrice:   Money(550),
	})
	if err != nil {
		t.Fatalf("UpdatePurchase error: %v", err)
	}
	if updated.TotalPriceCents != Money(3300) {
		t.Fatalf("expected total 3300, got %d", updated.TotalPriceCents)
	}
}

func TestDeleteBrandInUse(t *testing.T) {
	ds := &DataStore{}
	brand, _ := AddBrand(ds, CreateBrandParams{Name: "Brand"})
	_, _ = AddPurchase(ds, CreatePurchaseParams{
		BrandID:     brand.ID,
		PurchasedAt: time.Now(),
		Bags:        1,
		UnitPrice:   Money(500),
	})

	if err := DeleteBrand(ds, brand.ID); err != ErrBrandInUse {
		t.Fatalf("expected ErrBrandInUse, got %v", err)
	}
}

func TestAddConsumption(t *testing.T) {
	ds := &DataStore{}
	brand, _ := AddBrand(ds, CreateBrandParams{Name: "Brand"})
	consumption, err := AddConsumption(ds, CreateConsumptionParams{
		BrandID:    brand.ID,
		ConsumedAt: time.Now(),
		Bags:       2,
	})
	if err != nil {
		t.Fatalf("AddConsumption error: %v", err)
	}
	if len(ds.Consumptions) != 1 {
		t.Fatalf("expected 1 consumption, got %d", len(ds.Consumptions))
	}
	if consumption.Bags != 2 {
		t.Fatalf("expected bags 2, got %d", consumption.Bags)
	}
}

func TestAddPurchaseUnknownBrand(t *testing.T) {
	ds := &DataStore{}
	_, err := AddPurchase(ds, CreatePurchaseParams{
		BrandID:     ID("missing"),
		Bags:        1,
		UnitPrice:   Money(100),
		PurchasedAt: time.Now(),
	})
	if err == nil {
		t.Fatal("expected error for missing brand")
	}
	var vErr ValidationErrors
	if !errorsAs(err, &vErr) {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}
	if !vErr.Has("brand_id") {
		t.Fatalf("expected brand_id error, got %v", vErr)
	}
}

func TestAddPurchaseNegativeWeight(t *testing.T) {
	ds := &DataStore{}
	brand, _ := AddBrand(ds, CreateBrandParams{Name: "Brand"})
	_, err := AddPurchase(ds, CreatePurchaseParams{
		BrandID:     brand.ID,
		Bags:        1,
		WeightKg:    -1,
		UnitPrice:   Money(100),
		PurchasedAt: time.Now(),
	})
	if err == nil {
		t.Fatal("expected error for negative weight")
	}
	var vErr ValidationErrors
	if !errorsAs(err, &vErr) {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}
	if !vErr.Has("weight_kg") {
		t.Fatalf("expected weight_kg error, got %v", vErr)
	}
}

func errorsAs(err error, target *ValidationErrors) bool {
	if err == nil {
		return false
	}
	var ve ValidationErrors
	if errors.As(err, &ve) {
		*target = ve
		return true
	}
	return false
}
