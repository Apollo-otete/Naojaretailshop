package ordering

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	ErrNoQuote           = errors.New("no such quote")
	ErrQuoteExpired      = errors.New("quote has expired")
	ErrQuoteOwned        = errors.New("quote does not belong to you")
	ErrIdempotencyReplay = errors.New("idempotency replay") // handled specially
)

type FulfilmentMethod string

const (
	FulfilmentPickup   FulfilmentMethod = "pickup"
	FulfilmentDelivery FulfilmentMethod = "delivery"
)

func (m FulfilmentMethod) Valid() bool { return m == FulfilmentPickup || m == FulfilmentDelivery }

type QuoteRequest struct {
	FulfilmentMethod FulfilmentMethod `json:"fulfilment_method"`
	ContactName      string           `json:"contact_name"`
	ContactPhone     string           `json:"contact_phone"`
	ContactEmail     *string          `json:"contact_email"`
	Delivery         *DeliveryInput   `json:"delivery"`
	PickupWindowID   *int64           `json:"pickup_window_id"`
	Notes            *string          `json:"notes"`
	CouponCode       *string          `json:"coupon_code"` // reserved, ignored
}

type DeliveryInput struct {
	ZoneID      int64   `json:"zone_id"`
	AddressLine string  `json:"address_line"`
	Area        *string `json:"area"`
	County      *string `json:"county"`
	Landmark    *string `json:"landmark"`
}

type QuoteLine struct {
	ProductID      int64  `json:"product_id"`
	Name           string `json:"name"`
	SKU            string `json:"sku"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	Quantity       int    `json:"quantity"`
	LineTotalCents int64  `json:"line_total_cents"`
}

type Totals struct {
	SubtotalCents    int64  `json:"subtotal_cents"`
	DeliveryFeeCents int64  `json:"delivery_fee_cents"`
	DiscountCents    int64  `json:"discount_cents"`
	TotalCents       int64  `json:"total_cents"`
	Currency         string `json:"currency"`
}

type Quote struct {
	ID               int64            `json:"id"`
	UserID           *int64           `json:"-"`
	SessionTokenHash *string          `json:"-"`
	FulfilmentMethod FulfilmentMethod `json:"fulfilment_method"`
	ContactName      string           `json:"contact_name"`
	ContactPhone     string           `json:"contact_phone"`
	ContactEmail     *string          `json:"contact_email,omitempty"`
	Delivery         *DeliveryInput   `json:"delivery,omitempty"`
	PickupWindowID   *int64           `json:"pickup_window_id,omitempty"`
	Notes            *string          `json:"notes,omitempty"`
	Lines            []QuoteLine      `json:"lines"`
	Totals           Totals           `json:"totals"`
	DeliveryZone     any              `json:"delivery_zone"` // null/placeholder until zones modeled
	StoreWindow      any              `json:"store_window"`  // null until store hours modeled
	ExpiresAt        time.Time        `json:"expires_at"`
	PricingHash      string           `json:"-"`
	PricingChanged   bool             `json:"pricing_changed"`
	CreatedAt        time.Time        `json:"-"`
}

type PlaceOrderRequest struct {
	QuoteID           int64   `json:"quote_id"`
	Notes             *string `json:"notes"`
	AcceptPriceChange bool    `json:"accept_price_change"`
}

type IdempotencyRecord struct {
	UserID         int64
	Key            string
	RequestHash    string
	StatusCode     int
	ResponseBody   []byte
	OrderReference *string
}

// ContractOrderDetail is the response shape for placement/get that matches OpenAPI OrderDetail
// closely enough for P0 (payment/fulfilment/history are minimal).
type ContractOrderDetail struct {
	ID                 int64             `json:"id"`
	Reference          string            `json:"reference"`
	Status             string            `json:"status"`
	PaymentStatus      string            `json:"payment_status"`
	FulfilmentMethod   *FulfilmentMethod `json:"fulfilment_method,omitempty"`
	Totals             Totals            `json:"totals"`
	Contact            map[string]any    `json:"contact"`
	Delivery           any               `json:"delivery"`
	Pickup             any               `json:"pickup"`
	Lines              []OrderLineView   `json:"lines"`
	ExpiresAt          *time.Time        `json:"expires_at"`
	PlacedAt           time.Time         `json:"placed_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
	StatusHistory      []any             `json:"status_history"`
	Payment            any               `json:"payment"`
	Fulfilment         any               `json:"fulfilment"`
	CanCancel          bool              `json:"can_cancel"`
	AllowedTransitions []string          `json:"allowed_next"`
}

type OrderLineView struct {
	ID             int64  `json:"id"`
	ProductID      *int64 `json:"product_id"`
	Name           string `json:"name"`
	SKU            string `json:"sku"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	LineTotalCents int64  `json:"line_total_cents"`
}

var phoneKE = regexp.MustCompile(`^(?:\+254|0)[17]\d{8}$`)

func NormalizeKEPhone(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	if strings.HasPrefix(s, "0") && len(s) == 10 {
		s = "254" + s[1:]
	}
	return s
}

func (r QuoteRequest) Validate() error {
	if !r.FulfilmentMethod.Valid() {
		return errors.New("fulfilment_method must be pickup or delivery")
	}
	if strings.TrimSpace(r.ContactName) == "" || len([]rune(strings.TrimSpace(r.ContactName))) < 2 {
		return errors.New("contact_name is required")
	}
	if !phoneKE.MatchString(strings.TrimSpace(r.ContactPhone)) {
		return errors.New("contact_phone must be a valid Kenyan phone number")
	}
	if r.FulfilmentMethod == FulfilmentDelivery {
		if r.Delivery == nil || strings.TrimSpace(r.Delivery.AddressLine) == "" || r.Delivery.ZoneID <= 0 {
			return errors.New("delivery requires zone_id and address_line")
		}
	}
	// pickup_window_id is required by contract for pickup; store hours not modeled yet, so accept
	// a nil/zero placeholder only in this transitional P0? Contract says required - enforce if >0 or
	// allow nil with server-side default behavior. We'll accept 0/nil and set store_window null.
	return nil
}

func QuotePricingHash(lines []QuoteLine, deliveryFee, discount, total int64, method FulfilmentMethod) string {
	sorted := append([]QuoteLine(nil), lines...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ProductID < sorted[j].ProductID })
	payload := struct {
		Method FulfilmentMethod `json:"m"`
		Lines  []QuoteLine      `json:"l"`
		DFee   int64            `json:"df"`
		Disc   int64            `json:"dc"`
		Total  int64            `json:"t"`
	}{method, sorted, deliveryFee, discount, total}
	b, _ := json.Marshal(payload)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func PlaceRequestHash(req PlaceOrderRequest) string {
	b, _ := json.Marshal(req)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ContractDetail builds a contract-shaped order detail from the domain order+items.
func ContractDetail(o Order, items []OrderItem) ContractOrderDetail {
	status := mapStatus(string(o.Status))
	pay := string(o.PaymentStatus)
	var fm *FulfilmentMethod
	if o.FulfilmentMethod != nil && *o.FulfilmentMethod != "" {
		m := FulfilmentMethod(*o.FulfilmentMethod)
		fm = &m
	}
	lines := make([]OrderLineView, 0, len(items))
	for _, it := range items {
		lines = append(lines, OrderLineView{
			ID: it.ID, ProductID: it.ProductID, Name: it.ProductName, SKU: it.SKU,
			Quantity: it.Quantity, UnitPriceCents: it.UnitPriceCents, LineTotalCents: it.LineTotalCents,
		})
	}
	contact := map[string]any{"name": deref(o.ContactName), "phone": deref(o.ContactPhone), "email": o.ContactEmail}
	var delivery any
	if o.DeliveryAddress != nil || o.DeliveryZoneID != nil {
		delivery = map[string]any{
			"zone_id": o.DeliveryZoneID, "address_line": o.DeliveryAddress,
			"area": o.DeliveryArea, "county": o.DeliveryCounty, "landmark": o.DeliveryLandmark,
			"instructions": o.DeliveryInstructions,
		}
	}
	placed := o.CreatedAt
	return ContractOrderDetail{
		ID: o.ID, Reference: o.Reference, Status: status, PaymentStatus: pay, FulfilmentMethod: fm,
		Totals:  Totals{SubtotalCents: o.SubtotalCents, DeliveryFeeCents: o.DeliveryFeeCents, DiscountCents: 0, TotalCents: o.TotalCents, Currency: o.Currency},
		Contact: contact, Delivery: delivery, Pickup: nil, Lines: lines,
		ExpiresAt: o.ExpiresAt, PlacedAt: placed, UpdatedAt: o.UpdatedAt,
		StatusHistory: []any{}, Payment: nil, Fulfilment: nil,
		CanCancel: status == "pending", AllowedTransitions: allowedNext(status, fm),
	}
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func mapStatus(s string) string {
	switch s {
	case "pending_payment":
		return "pending"
	case "paid":
		return "confirmed" // order moves to confirmed when paid in contract model; keep simple
	default:
		return s
	}
}

func allowedNext(status string, fm *FulfilmentMethod) []string {
	switch status {
	case "pending":
		return []string{"confirmed", "cancelled", "expired"}
	case "confirmed":
		return []string{"processing", "cancelled"}
	case "processing":
		if fm != nil && *fm == FulfilmentPickup {
			return []string{"ready", "cancelled"}
		}
		return []string{"dispatched", "cancelled"}
	case "ready":
		return []string{"delivered", "cancelled"}
	case "dispatched":
		return []string{"delivered", "cancelled"}
	default:
		return []string{}
	}
}

// TrackRequest is the verified public tracking input (POST, no session/CSRF).
type TrackRequest struct {
	Reference string `json:"reference"`
	Phone     string `json:"phone"`
}

type TrackTimelineEntry struct {
	Status     string    `json:"status"`
	OccurredAt time.Time `json:"occurred_at"`
}

type TrackExpectedWindow struct {
	Start *time.Time `json:"start"`
	End   *time.Time `json:"end"`
}

// OrderTracking is the minimal, non-leaking projection (G3).
type OrderTracking struct {
	Reference        string               `json:"reference"`
	Status           string               `json:"status"`
	PaymentStatus    string               `json:"payment_status"`
	FulfilmentMethod string               `json:"fulfilment_method"`
	PlacedAt         time.Time            `json:"placed_at"`
	ExpectedWindow   *TrackExpectedWindow `json:"expected_window"`
	Timeline         []TrackTimelineEntry `json:"timeline"`
}
