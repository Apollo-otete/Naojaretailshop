// Package ordering is the Ordering bounded context (§7).
package ordering

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type OrderStatus string

const (
	StatusPendingPayment OrderStatus = "pending_payment"
	StatusPaid           OrderStatus = "paid"
	StatusProcessing     OrderStatus = "processing"
	StatusReady          OrderStatus = "ready"
	StatusDispatched     OrderStatus = "dispatched"
	StatusDelivered      OrderStatus = "delivered"
	StatusCancelled      OrderStatus = "cancelled"
	StatusFailed         OrderStatus = "failed"
	StatusExpired        OrderStatus = "expired"
)

func (s OrderStatus) Valid() bool {
	switch s {
	case StatusPendingPayment, StatusPaid, StatusProcessing, StatusReady, StatusDispatched, StatusDelivered, StatusCancelled, StatusFailed, StatusExpired:
		return true
	}
	return false
}

type PaymentStatus string

const (
	PayPending  PaymentStatus = "pending"
	PayPaid     PaymentStatus = "paid"
	PayFailed   PaymentStatus = "failed"
	PayRefunded PaymentStatus = "refunded"
)

type Order struct {
	ID                   int64         `json:"id"`
	Reference            string        `json:"reference"`
	UserID               *int64        `json:"user_id"`
	Status               OrderStatus   `json:"status"`
	PaymentStatus        PaymentStatus `json:"payment_status"`
	SubtotalCents        int64         `json:"subtotal_cents"`
	DeliveryFeeCents     int64         `json:"delivery_fee_cents"`
	TotalCents           int64         `json:"total_cents"`
	Currency             string        `json:"currency"`
	DeliveryAddress      *string       `json:"delivery_address"`
	ContactPhone         *string       `json:"contact_phone"`
	DeliveryInstructions *string       `json:"delivery_instructions"`
	ExpiresAt            *time.Time    `json:"expires_at"`
	CreatedAt            time.Time     `json:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at"`
	PaidAt               *time.Time    `json:"paid_at"`
	CancelledAt          *time.Time    `json:"cancelled_at"`
}

type OrderItem struct {
	ID             int64     `json:"id"`
	OrderID        int64     `json:"order_id"`
	ProductID      *int64    `json:"product_id"`
	ProductName    string    `json:"product_name"`
	SKU            string    `json:"sku"`
	Quantity       int       `json:"quantity"`
	UnitPriceCents int64     `json:"unit_price_cents"`
	LineTotalCents int64     `json:"line_total_cents"`
	CreatedAt      time.Time `json:"created_at"`
}

type OrderStatusHistory struct {
	ID         int64
	OrderID    int64
	FromStatus *string
	ToStatus   OrderStatus
	Reason     *string
	ActorID    *int64
	OccurredAt time.Time
}

var (
	ErrNoOrder = errors.New("no such order")
)

type CreateOrderInput struct {
	DeliveryAddress      *string
	ContactPhone         *string
	DeliveryInstructions *string
	DeliveryFeeCents     int64
}

// OrderView is the read shape.
type OrderView struct {
	Order
	Items []OrderItem `json:"items"`
}

var _ = context.Background
var _ = fmt.Sprintf
var _ = strings.TrimSpace
var _ = time.Now
