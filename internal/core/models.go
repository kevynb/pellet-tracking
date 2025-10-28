package core

import (
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
	WeightKg        float64   `json:"weight_kg"`
	UnitPriceCents  Money     `json:"unit_price_cents"`
	TotalPriceCents Money     `json:"total_price_cents"`
	Notes           string    `json:"notes,omitempty"`
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
