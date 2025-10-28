package core

import (
	"sort"
	"time"
)

// ConsumptionAllocation describes how a consumption is valued against purchases.
type ConsumptionAllocation struct {
	PurchaseID ID    `json:"purchase_id"`
	Bags       int   `json:"bags"`
	UnitPrice  Money `json:"unit_price_cents"`
	TotalPrice Money `json:"total_price_cents"`
}

// ConsumptionCost details the FIFO valuation of a consumption entry.
type ConsumptionCost struct {
	Consumption Consumption             `json:"consumption"`
	Allocations []ConsumptionAllocation `json:"allocations"`
	TotalBags   int                     `json:"total_bags"`
	TotalPrice  Money                   `json:"total_price_cents"`
}

// BrandInventory summarizes the remaining inventory for a brand.
type BrandInventory struct {
	BrandID   ID      `json:"brand_id"`
	BrandName string  `json:"brand_name"`
	Bags      int     `json:"bags"`
	WeightKg  float64 `json:"weight_kg"`
	TotalCost Money   `json:"total_cost_cents"`
}

// InventorySummary captures the global inventory position.
type InventorySummary struct {
	TotalBags     int              `json:"total_bags"`
	TotalWeightKg float64          `json:"total_weight_kg"`
	TotalCost     Money            `json:"total_cost_cents"`
	Brands        []BrandInventory `json:"brands"`
}

// MonthlyBags tracks the number of bags consumed in a specific month.
type MonthlyBags struct {
	Month time.Time `json:"month"`
	Bags  int       `json:"bags"`
}

// ComputeInvesti returns the total amount invested in purchases within the optional range.
func ComputeInvesti(ds *DataStore, from, to time.Time) Money {
	if ds == nil {
		return 0
	}

	var total Money
	for _, purchase := range ds.Purchases {
		if !withinRange(purchase.PurchasedAt, from, to) {
			continue
		}
		total += purchase.TotalPriceCents
	}
	return total
}

// ComputeConsoValue returns the FIFO valuation for consumptions within the optional range.
func ComputeConsoValue(ds *DataStore, from, to time.Time) (Money, []ConsumptionCost, error) {
	if ds == nil {
		return 0, nil, nil
	}

	calculations, _, err := computeFIFOResults(ds)
	if err != nil {
		return 0, nil, err
	}

	var total Money
	var details []ConsumptionCost
	for _, calc := range calculations {
		if !withinRange(calc.consumption.ConsumedAt, from, to) {
			continue
		}

		detail := ConsumptionCost{
			Consumption: calc.consumption,
			Allocations: append([]ConsumptionAllocation(nil), calc.allocations...),
			TotalPrice:  calc.total,
			TotalBags:   calc.consumption.Bags,
		}
		total += calc.total
		details = append(details, detail)
	}

	return total, details, nil
}

// ComputeInventaire calculates the remaining inventory per brand after FIFO consumption.
func ComputeInventaire(ds *DataStore) (InventorySummary, error) {
	if ds == nil {
		return InventorySummary{}, nil
	}

	_, tracker, err := computeFIFOResults(ds)
	if err != nil {
		return InventorySummary{}, err
	}

	return tracker.inventorySummary(ds.Brands), nil
}

// ComputeSacsParMois aggregates the number of bags consumed per month within the range.
func ComputeSacsParMois(ds *DataStore, from, to time.Time) ([]MonthlyBags, error) {
	if ds == nil {
		return nil, nil
	}

	calculations, _, err := computeFIFOResults(ds)
	if err != nil {
		return nil, err
	}

	buckets := make(map[time.Time]int)
	for _, calc := range calculations {
		if !withinRange(calc.consumption.ConsumedAt, from, to) {
			continue
		}
		month := time.Date(calc.consumption.ConsumedAt.Year(), calc.consumption.ConsumedAt.Month(), 1, 0, 0, 0, 0, time.UTC)
		buckets[month] += calc.consumption.Bags
	}

	results := make([]MonthlyBags, 0, len(buckets))
	for month, bags := range buckets {
		results = append(results, MonthlyBags{Month: month, Bags: bags})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Month.Before(results[j].Month)
	})

	return results, nil
}

// ComputeCoutMoyenParSac returns the average FIFO cost per bag consumed within the range.
func ComputeCoutMoyenParSac(ds *DataStore, from, to time.Time) (Money, error) {
	if ds == nil {
		return 0, nil
	}

	calculations, _, err := computeFIFOResults(ds)
	if err != nil {
		return 0, err
	}

	var totalCost Money
	var totalBags int
	for _, calc := range calculations {
		if !withinRange(calc.consumption.ConsumedAt, from, to) {
			continue
		}
		totalCost += calc.total
		totalBags += calc.consumption.Bags
	}

	if totalBags == 0 {
		return 0, nil
	}

	avgCents := roundHalfEven(float64(totalCost.Int64()) / float64(totalBags))
	return Money(int64(avgCents)), nil
}

func withinRange(ts, from, to time.Time) bool {
	if !from.IsZero() && ts.Before(from) {
		return false
	}
	if !to.IsZero() && ts.After(to) {
		return false
	}
	return true
}

type consumptionCalculation struct {
	consumption Consumption
	allocations []ConsumptionAllocation
	total       Money
}

type purchaseLot struct {
	id           ID
	unitPrice    Money
	remaining    int
	weightPerBag float64
}

type fifoState struct {
	lots  []*purchaseLot
	index int
}

type fifoTracker struct {
	states map[ID]*fifoState
}

func computeFIFOResults(ds *DataStore) ([]consumptionCalculation, *fifoTracker, error) {
	tracker := newFIFOTracker(ds)
	consumptions := append([]Consumption(nil), ds.Consumptions...)
	sort.Slice(consumptions, func(i, j int) bool {
		if consumptions[i].ConsumedAt.Equal(consumptions[j].ConsumedAt) {
			return string(consumptions[i].ID) < string(consumptions[j].ID)
		}
		return consumptions[i].ConsumedAt.Before(consumptions[j].ConsumedAt)
	})

	results := make([]consumptionCalculation, 0, len(consumptions))
	for _, consumption := range consumptions {
		allocations, total, err := tracker.consume(consumption)
		if err != nil {
			return nil, nil, err
		}
		results = append(results, consumptionCalculation{
			consumption: consumption,
			allocations: allocations,
			total:       total,
		})
	}

	return results, tracker, nil
}

func newFIFOTracker(ds *DataStore) *fifoTracker {
	tracker := &fifoTracker{states: make(map[ID]*fifoState)}
	purchases := append([]Purchase(nil), ds.Purchases...)
	sort.Slice(purchases, func(i, j int) bool {
		if purchases[i].PurchasedAt.Equal(purchases[j].PurchasedAt) {
			return string(purchases[i].ID) < string(purchases[j].ID)
		}
		return purchases[i].PurchasedAt.Before(purchases[j].PurchasedAt)
	})

	for _, purchase := range purchases {
		if purchase.Bags <= 0 {
			continue
		}
		state := tracker.states[purchase.BrandID]
		if state == nil {
			state = &fifoState{}
			tracker.states[purchase.BrandID] = state
		}
		weightPerBag := 0.0
		if purchase.Bags > 0 {
			weightPerBag = purchase.WeightKg / float64(purchase.Bags)
		}
		state.lots = append(state.lots, &purchaseLot{
			id:           purchase.ID,
			unitPrice:    purchase.UnitPriceCents,
			remaining:    purchase.Bags,
			weightPerBag: weightPerBag,
		})
	}

	return tracker
}

func (t *fifoTracker) consume(consumption Consumption) ([]ConsumptionAllocation, Money, error) {
	if consumption.Bags <= 0 {
		return nil, 0, nil
	}

	state := t.states[consumption.BrandID]
	if state == nil {
		return nil, 0, ErrInsufficientInventory
	}

	var allocations []ConsumptionAllocation
	var total Money
	remainingBags := consumption.Bags

	for remainingBags > 0 {
		lot := state.nextLot()
		if lot == nil {
			return nil, 0, ErrInsufficientInventory
		}

		take := remainingBags
		if take > lot.remaining {
			take = lot.remaining
		}

		lot.remaining -= take
		if lot.remaining == 0 {
			state.index++
		}

		cost := lot.unitPrice.MulInt(take)
		allocations = append(allocations, ConsumptionAllocation{
			PurchaseID: lot.id,
			Bags:       take,
			UnitPrice:  lot.unitPrice,
			TotalPrice: cost,
		})
		total += cost
		remainingBags -= take
	}

	return allocations, total, nil
}

func (s *fifoState) nextLot() *purchaseLot {
	for s != nil && s.index < len(s.lots) {
		lot := s.lots[s.index]
		if lot.remaining > 0 {
			return lot
		}
		s.index++
	}
	return nil
}

func (t *fifoTracker) inventorySummary(brands []Brand) InventorySummary {
	brandNames := make(map[ID]string, len(brands))
	for _, brand := range brands {
		brandNames[brand.ID] = brand.Name
	}

	summary := InventorySummary{}
	for brandID, state := range t.states {
		var bags int
		var weight float64
		var cost Money

		if state != nil {
			for idx := state.index; idx < len(state.lots); idx++ {
				lot := state.lots[idx]
				if lot.remaining <= 0 {
					continue
				}
				bags += lot.remaining
				weight += float64(lot.remaining) * lot.weightPerBag
				cost += lot.unitPrice.MulInt(lot.remaining)
			}
		}

		if bags == 0 && weight == 0 && cost == 0 {
			continue
		}

		summary.Brands = append(summary.Brands, BrandInventory{
			BrandID:   brandID,
			BrandName: brandNames[brandID],
			Bags:      bags,
			WeightKg:  weight,
			TotalCost: cost,
		})
		summary.TotalBags += bags
		summary.TotalWeightKg += weight
		summary.TotalCost += cost
	}

	sort.Slice(summary.Brands, func(i, j int) bool {
		if summary.Brands[i].BrandName == summary.Brands[j].BrandName {
			return string(summary.Brands[i].BrandID) < string(summary.Brands[j].BrandID)
		}
		return summary.Brands[i].BrandName < summary.Brands[j].BrandName
	})

	return summary
}
