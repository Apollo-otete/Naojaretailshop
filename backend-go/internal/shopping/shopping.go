// Package shopping is the Shopping bounded context (§7): carts and cart items.
package shopping

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Cart struct {
	ID               int64
	UserID           *int64
	SessionTokenHash *string
	MergedIntoCartID *int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type CartItem struct {
	CartID             int64
	ProductID          int64
	Quantity           int
	PriceSnapshotCents int64
	AddedAtPriceCents  int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

var (
	ErrNoCart               = errors.New("no such cart")
	ErrEmptyCart            = errors.New("cart is empty")
	ErrBadQuantity          = errors.New("quantity must be between 1 and 999")
	ErrGuestSessionRequired = errors.New("guest cart requires a session token")
)

// CartView is the read shape with product details.
type CartLine struct {
	ProductID         int64   `json:"product_id"`
	SKU               string  `json:"sku"`
	Name              string  `json:"name"`
	ImageURL          *string `json:"image_url"`
	Quantity          int     `json:"quantity"`
	UnitPriceCents    int64   `json:"unit_price_cents"`
	AddedAtPriceCents int64   `json:"added_at_price_cents"`
	LineTotalCents    int64   `json:"line_total_cents"`
	AvailableQuantity int     `json:"available_quantity"`
	Availability      string  `json:"availability"`
}

type CartView struct {
	Items         []CartLine `json:"items"`
	SubtotalCents int64      `json:"subtotal_cents"`
	ItemCount     int        `json:"item_count"`
}

func CheckQuantity(q int) error {
	if q < 1 || q > 999 {
		return ErrBadQuantity
	}
	return nil
}

// HashToken hashes a guest session token so the raw value is never stored (§10).
func HashToken(tok string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(tok)))
	return hex.EncodeToString(h[:])
}

var _ = context.Background
var _ = fmt.Sprintf
var _ = strings.TrimSpace
var _ = time.Now
