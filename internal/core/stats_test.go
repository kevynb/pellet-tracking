package core

import (
	"testing"
	"time"
)

func TestComputeInvesti(t *testing.T) {
	ds := &DataStore{
		Purchases: []Purchase{
			{
				Meta:            Meta{ID: "p1"},
				PurchasedAt:     time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
				Bags:            10,
				TotalPriceCents: Money(30000),
			},
			{
				Meta:            Meta{ID: "p2"},
				PurchasedAt:     time.Date(2023, 3, 5, 0, 0, 0, 0, time.UTC),
				Bags:            5,
				TotalPriceCents: Money(16000),
			},
		},
	}

	from := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC)

	got := ComputeInvesti(ds, from, to)
	if got != Money(30000) {
		t.Fatalf("expected 30000, got %d", got)
	}
}

func TestComputeConsoValueFIFO(t *testing.T) {
	brandID := ID("b1")
	purchase1 := Purchase{
		Meta:            Meta{ID: "p1"},
		BrandID:         brandID,
		PurchasedAt:     time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		Bags:            10,
		WeightKg:        150,
		UnitPriceCents:  Money(3000),
		TotalPriceCents: Money(30000),
	}
	purchase2 := Purchase{
		Meta:            Meta{ID: "p2"},
		BrandID:         brandID,
		PurchasedAt:     time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
		Bags:            5,
		WeightKg:        75,
		UnitPriceCents:  Money(3200),
		TotalPriceCents: Money(16000),
	}

	consumption1 := Consumption{
		Meta:       Meta{ID: "c1"},
		BrandID:    brandID,
		ConsumedAt: time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC),
		Bags:       8,
	}
	consumption2 := Consumption{
		Meta:       Meta{ID: "c2"},
		BrandID:    brandID,
		ConsumedAt: time.Date(2023, 3, 10, 0, 0, 0, 0, time.UTC),
		Bags:       3,
	}
	consumption3 := Consumption{
		Meta:       Meta{ID: "c3"},
		BrandID:    brandID,
		ConsumedAt: time.Date(2023, 3, 20, 0, 0, 0, 0, time.UTC),
		Bags:       2,
	}

	ds := &DataStore{
		Brands:       []Brand{{Meta: Meta{ID: brandID}, Name: "Pellets+"}},
		Purchases:    []Purchase{purchase1, purchase2},
		Consumptions: []Consumption{consumption1, consumption2, consumption3},
	}

	from := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2023, 2, 28, 23, 59, 59, 0, time.UTC)

	total, details, err := ComputeConsoValue(ds, from, to)
	if err != nil {
		t.Fatalf("ComputeConsoValue returned error: %v", err)
	}

	if total != Money(24000) {
		t.Fatalf("expected total 24000, got %d", total)
	}

	if len(details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(details))
	}

	d := details[0]
	if d.TotalBags != 8 {
		t.Fatalf("expected 8 bags, got %d", d.TotalBags)
	}
	if len(d.Allocations) != 1 || d.Allocations[0].PurchaseID != purchase1.ID || d.Allocations[0].TotalPrice != Money(24000) {
		t.Fatalf("unexpected allocations: %+v", d.Allocations)
	}

	// Ensure later consumptions do not break FIFO state
	_, _, err = ComputeConsoValue(ds, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("ComputeConsoValue full range error: %v", err)
	}
}

func TestComputeConsoValueInsufficientInventory(t *testing.T) {
	ds := &DataStore{
		Brands: []Brand{{Meta: Meta{ID: "b1"}, Name: "Brand"}},
		Consumptions: []Consumption{{
			Meta:       Meta{ID: "c1"},
			BrandID:    ID("b1"),
			ConsumedAt: time.Now(),
			Bags:       2,
		}},
	}

	_, _, err := ComputeConsoValue(ds, time.Time{}, time.Time{})
	if err != ErrInsufficientInventory {
		t.Fatalf("expected ErrInsufficientInventory, got %v", err)
	}
}

func TestComputeInventaire(t *testing.T) {
	brandID := ID("b1")
	ds := &DataStore{
		Brands: []Brand{{Meta: Meta{ID: brandID}, Name: "Pellets+"}},
		Purchases: []Purchase{{
			Meta:            Meta{ID: "p1"},
			BrandID:         brandID,
			PurchasedAt:     time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			Bags:            10,
			WeightKg:        150,
			UnitPriceCents:  Money(3000),
			TotalPriceCents: Money(30000),
		}, {
			Meta:            Meta{ID: "p2"},
			BrandID:         brandID,
			PurchasedAt:     time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
			Bags:            5,
			WeightKg:        75,
			UnitPriceCents:  Money(3200),
			TotalPriceCents: Money(16000),
		}},
		Consumptions: []Consumption{{
			Meta:       Meta{ID: "c1"},
			BrandID:    brandID,
			ConsumedAt: time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC),
			Bags:       8,
		}, {
			Meta:       Meta{ID: "c2"},
			BrandID:    brandID,
			ConsumedAt: time.Date(2023, 3, 10, 0, 0, 0, 0, time.UTC),
			Bags:       3,
		}, {
			Meta:       Meta{ID: "c3"},
			BrandID:    brandID,
			ConsumedAt: time.Date(2023, 3, 20, 0, 0, 0, 0, time.UTC),
			Bags:       2,
		}},
	}

	summary, err := ComputeInventaire(ds)
	if err != nil {
		t.Fatalf("ComputeInventaire returned error: %v", err)
	}

	if summary.TotalBags != 2 {
		t.Fatalf("expected total bags 2, got %d", summary.TotalBags)
	}
	if summary.TotalCost != Money(6400) {
		t.Fatalf("expected total cost 6400, got %d", summary.TotalCost)
	}
	if len(summary.Brands) != 1 {
		t.Fatalf("expected 1 brand summary, got %d", len(summary.Brands))
	}
	brandSummary := summary.Brands[0]
	if brandSummary.Bags != 2 {
		t.Fatalf("expected brand bags 2, got %d", brandSummary.Bags)
	}
	if brandSummary.TotalCost != Money(6400) {
		t.Fatalf("expected brand cost 6400, got %d", brandSummary.TotalCost)
	}
	if brandSummary.WeightKg < 29.9 || brandSummary.WeightKg > 30.1 {
		t.Fatalf("expected weight around 30kg, got %f", brandSummary.WeightKg)
	}
}

func TestComputeSacsParMois(t *testing.T) {
	brandID := ID("b1")
	ds := &DataStore{
		Purchases: []Purchase{{
			Meta:            Meta{ID: "p1"},
			BrandID:         brandID,
			PurchasedAt:     time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			Bags:            20,
			WeightKg:        300,
			UnitPriceCents:  Money(3000),
			TotalPriceCents: Money(60000),
		}},
		Consumptions: []Consumption{{
			Meta:       Meta{ID: "c1"},
			BrandID:    brandID,
			ConsumedAt: time.Date(2023, 1, 5, 0, 0, 0, 0, time.UTC),
			Bags:       5,
		}, {
			Meta:       Meta{ID: "c2"},
			BrandID:    brandID,
			ConsumedAt: time.Date(2023, 1, 20, 0, 0, 0, 0, time.UTC),
			Bags:       4,
		}, {
			Meta:       Meta{ID: "c3"},
			BrandID:    brandID,
			ConsumedAt: time.Date(2023, 3, 2, 0, 0, 0, 0, time.UTC),
			Bags:       6,
		}},
	}

	from := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2023, 3, 31, 23, 59, 59, 0, time.UTC)
	months, err := ComputeSacsParMois(ds, from, to)
	if err != nil {
		t.Fatalf("ComputeSacsParMois returned error: %v", err)
	}

	if len(months) != 2 {
		t.Fatalf("expected 2 months, got %d", len(months))
	}
	if months[0].Month.Year() != 2023 || months[0].Month.Month() != time.January || months[0].Bags != 9 {
		t.Fatalf("unexpected january summary: %+v", months[0])
	}
	if months[1].Month.Year() != 2023 || months[1].Month.Month() != time.March || months[1].Bags != 6 {
		t.Fatalf("unexpected march summary: %+v", months[1])
	}
}

func TestComputeCoutMoyenParSac(t *testing.T) {
	brandID := ID("b1")
	ds := &DataStore{
		Purchases: []Purchase{{
			Meta:            Meta{ID: "p1"},
			BrandID:         brandID,
			PurchasedAt:     time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			Bags:            10,
			WeightKg:        150,
			UnitPriceCents:  Money(3000),
			TotalPriceCents: Money(30000),
		}, {
			Meta:            Meta{ID: "p2"},
			BrandID:         brandID,
			PurchasedAt:     time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
			Bags:            5,
			WeightKg:        75,
			UnitPriceCents:  Money(3200),
			TotalPriceCents: Money(16000),
		}},
		Consumptions: []Consumption{{
			Meta:       Meta{ID: "c1"},
			BrandID:    brandID,
			ConsumedAt: time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC),
			Bags:       8,
		}, {
			Meta:       Meta{ID: "c2"},
			BrandID:    brandID,
			ConsumedAt: time.Date(2023, 3, 10, 0, 0, 0, 0, time.UTC),
			Bags:       3,
		}, {
			Meta:       Meta{ID: "c3"},
			BrandID:    brandID,
			ConsumedAt: time.Date(2023, 3, 20, 0, 0, 0, 0, time.UTC),
			Bags:       2,
		}},
	}

	avg, err := ComputeCoutMoyenParSac(ds, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("ComputeCoutMoyenParSac returned error: %v", err)
	}

	if avg != Money(3046) {
		t.Fatalf("expected average 3046 cents, got %d", avg)
	}

	avg, err = ComputeCoutMoyenParSac(ds, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("ComputeCoutMoyenParSac empty range error: %v", err)
	}
	if avg != 0 {
		t.Fatalf("expected zero average for empty range, got %d", avg)
	}
}
