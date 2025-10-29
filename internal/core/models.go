package core

import (
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// ID represents the identifier type for domain entities.
type ID string

// Meta captures metadata for persisted entities.
type Meta struct {
	ID        ID        `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Money represents a monetary amount stored in euro cents.
type Money int64

// Float64 returns the euro amount as a float64 for display purposes.
func (m Money) Float64() float64 {
	return float64(m) / 100.0
}

// Int64 exposes the raw cents representation.
func (m Money) Int64() int64 {
	return int64(m)
}

// MulInt multiplies the money amount by an integer factor.
func (m Money) MulInt(count int) Money {
	return Money(int64(m) * int64(count))
}

// ParseMoney converts a float euro amount into Money using round half even.
func ParseMoney(amount float64) Money {
	return Money(int64(roundHalfEven(amount * 100)))
}

// Brand describes a pellets brand.
type Brand struct {
	Meta
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ImageBase64 string `json:"image_base64,omitempty"`
}

// Purchase records a pellets purchase.
type Purchase struct {
	Meta
	BrandID         ID        `json:"brand_id"`
	PurchasedAt     time.Time `json:"purchased_at"`
	Bags            int       `json:"bags"`
	BagWeightKg     float64   `json:"bag_weight_kg"`
	TotalWeightKg   float64   `json:"total_weight_kg"`
	UnitPriceCents  Money     `json:"unit_price_cents"`
	TotalPriceCents Money     `json:"total_price_cents"`
	Notes           string    `json:"notes,omitempty"`
}

// MarshalJSON emits both the per-bag and total weight fields while keeping
// compatibility with the historical weight_kg property that represented the
// total purchase weight.
func (p Purchase) MarshalJSON() ([]byte, error) {
	type alias Purchase
	return json.Marshal(struct {
		alias
		WeightKg float64 `json:"weight_kg,omitempty"`
	}{
		alias:    alias(p),
		WeightKg: p.TotalWeightKg,
	})
}

// UnmarshalJSON supports decoding both the new bag_weight_kg/total_weight_kg
// fields and the legacy weight_kg field that stored the total weight directly.
func (p *Purchase) UnmarshalJSON(data []byte) error {
	type alias Purchase
	aux := struct {
		alias
		WeightKg float64 `json:"weight_kg"`
	}{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*p = Purchase(aux.alias)
	if p.TotalWeightKg == 0 && aux.WeightKg > 0 {
		p.TotalWeightKg = aux.WeightKg
	}
	if p.BagWeightKg == 0 && p.Bags > 0 && p.TotalWeightKg > 0 {
		p.BagWeightKg = p.TotalWeightKg / float64(p.Bags)
	}
	if p.TotalWeightKg == 0 && p.BagWeightKg > 0 && p.Bags > 0 {
		p.TotalWeightKg = p.BagWeightKg * float64(p.Bags)
	}
	return nil
}

// Consumption records pellets consumption events.
type Consumption struct {
	Meta
	BrandID    ID        `json:"brand_id"`
	ConsumedAt time.Time `json:"consumed_at"`
	Bags       int       `json:"bags"`
	Notes      string    `json:"notes,omitempty"`
}

// DataStore contains the complete persisted dataset.
type DataStore struct {
	Meta
	Brands       []Brand       `json:"brands"`
	Purchases    []Purchase    `json:"purchases"`
	Consumptions []Consumption `json:"consumptions"`
}

// NewID creates a new ULID identifier.
func NewID() ID {
	return ID(ulid.Make().String())
}

func roundHalfEven(value float64) float64 {
	integral, frac := math.Modf(value)
	if math.Abs(math.Abs(frac)-0.5) <= 1e-9 {
		if int64(math.Abs(integral))%2 == 0 {
			return integral
		}
		if value > 0 {
			return integral + 1
		}
		return integral - 1
	}
	return math.Round(value)
}

// NormalizeName trims and collapses whitespace in names for consistency.
func NormalizeName(name string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(name)), " ")
}
