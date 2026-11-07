package httpordering

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpjson"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/ordering"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
)

type Service interface {
	CreateFromCart(ctx context.Context, caller identity.Principal, sessionToken string, in ordering.CreateOrderInput) (ordering.Order, error)
	GetOrder(ctx context.Context, caller identity.Principal, ref string) (ordering.Order, []ordering.OrderItem, error)
	ListOrders(ctx context.Context, caller identity.Principal, status *ordering.OrderStatus, page, pageSize int) ([]ordering.Order, int64, error)
	ProcessStatus(ctx context.Context, caller identity.Principal, ref string, to ordering.OrderStatus, reason string) (ordering.Order, error)
}

type Deps struct {
	Ordering   Service
	Authorizer port.Authorizer
	Logger     *slog.Logger
}

func Mount(r gin.IRouter, d Deps) {
	r.POST("/orders", createOrder(d))
	r.GET("/orders", listOrders(d))
	r.GET("/orders/:reference", getOrder(d))
	r.POST("/orders/:reference/status", processStatus(d))
}

func decode(c *gin.Context, target any) error {
	if err := json.NewDecoder(c.Request.Body).Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return apierr.BadRequest("A JSON request body is required.")
		}
		return apierr.BadRequest("The request body is not valid JSON.")
	}
	return nil
}

func caller(c *gin.Context) (identity.Principal, bool) {
	return identity.PrincipalFrom(c.Request.Context())
}

func sessionToken(c *gin.Context) string {
	if v, err := c.Cookie("naoja_cart"); err == nil && v != "" {
		return v
	}
	if v := c.GetHeader("X-Cart-Session"); v != "" {
		return v
	}
	return ""
}

func createOrder(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := caller(c)
		if !ok {
			httpjson.Write(c, d.Logger, apierr.Unauthorized("Sign in to place an order."))
			return
		}
		var in struct {
			DeliveryAddress      *string `json:"delivery_address"`
			ContactPhone         *string `json:"contact_phone"`
			DeliveryInstructions *string `json:"delivery_instructions"`
			DeliveryFeeCents     int64   `json:"delivery_fee_cents"`
		}
		if err := decode(c, &in); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		o, err := d.Ordering.CreateFromCart(c.Request.Context(), p, sessionToken(c), ordering.CreateOrderInput{
			DeliveryAddress: in.DeliveryAddress, ContactPhone: in.ContactPhone,
			DeliveryInstructions: in.DeliveryInstructions, DeliveryFeeCents: in.DeliveryFeeCents,
		})
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		httpjson.Created(c, orderViewOf(o, nil))
	}
}

func getOrder(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := caller(c)
		if !ok {
			httpjson.Write(c, d.Logger, apierr.Unauthorized("Sign in."))
			return
		}
		o, items, err := d.Ordering.GetOrder(c.Request.Context(), p, c.Param("reference"))
		if err != nil {
			if errors.Is(err, ordering.ErrNoOrder) {
				httpjson.Write(c, d.Logger, apierr.NotFound("order"))
				return
			}
			httpjson.Fail(c, d.Logger, err)
			return
		}
		httpjson.OK(c, orderViewOf(o, items))
	}
}

func listOrders(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := caller(c)
		if !ok {
			httpjson.Write(c, d.Logger, apierr.Unauthorized("Sign in."))
			return
		}
		page := queryInt(c, "page", 1)
		pageSize := queryInt(c, "page_size", 20)
		var status *ordering.OrderStatus
		if v := c.Query("status"); v != "" {
			s := ordering.OrderStatus(v)
			status = &s
		}
		orders, total, err := d.Ordering.ListOrders(c.Request.Context(), p, status, page, pageSize)
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		data := make([]map[string]any, 0, len(orders))
		for _, o := range orders {
			data = append(data, orderMapOf(o))
		}
		httpjson.OK(c, gin.H{"data": data, "meta": gin.H{"page": page, "page_size": pageSize, "total": total}})
	}
}

func processStatus(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := caller(c)
		if !ok {
			httpjson.Write(c, d.Logger, apierr.Unauthorized("Sign in."))
			return
		}
		var in struct {
			Status ordering.OrderStatus `json:"status"`
			Reason string               `json:"reason"`
		}
		if err := decode(c, &in); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		o, err := d.Ordering.ProcessStatus(c.Request.Context(), p, c.Param("reference"), in.Status, in.Reason)
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		httpjson.OK(c, orderMapOf(o))
	}
}

type orderView struct {
	ID                   int64                  `json:"id"`
	Reference            string                 `json:"reference"`
	Status               ordering.OrderStatus   `json:"status"`
	PaymentStatus        ordering.PaymentStatus `json:"payment_status"`
	SubtotalCents        int64                  `json:"subtotal_cents"`
	DeliveryFeeCents     int64                  `json:"delivery_fee_cents"`
	TotalCents           int64                  `json:"total_cents"`
	Currency             string                 `json:"currency"`
	DeliveryAddress      *string                `json:"delivery_address"`
	ContactPhone         *string                `json:"contact_phone"`
	DeliveryInstructions *string                `json:"delivery_instructions"`
	ExpiresAt            *time.Time             `json:"expires_at"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
	PaidAt               *time.Time             `json:"paid_at"`
	CancelledAt          *time.Time             `json:"cancelled_at"`
	Items                []ordering.OrderItem   `json:"items"`
}

func orderViewOf(o ordering.Order, items []ordering.OrderItem) orderView {
	if items == nil {
		items = []ordering.OrderItem{}
	}
	return orderView{
		ID: o.ID, Reference: o.Reference, Status: o.Status, PaymentStatus: o.PaymentStatus,
		SubtotalCents: o.SubtotalCents, DeliveryFeeCents: o.DeliveryFeeCents, TotalCents: o.TotalCents,
		Currency: o.Currency, DeliveryAddress: o.DeliveryAddress, ContactPhone: o.ContactPhone,
		DeliveryInstructions: o.DeliveryInstructions, ExpiresAt: o.ExpiresAt, CreatedAt: o.CreatedAt,
		UpdatedAt: o.UpdatedAt, PaidAt: o.PaidAt, CancelledAt: o.CancelledAt, Items: items,
	}
}

func orderMapOf(o ordering.Order) map[string]any {
	return map[string]any{
		"id": o.ID, "reference": o.Reference, "status": o.Status, "payment_status": o.PaymentStatus,
		"subtotal_cents": o.SubtotalCents, "delivery_fee_cents": o.DeliveryFeeCents, "total_cents": o.TotalCents,
		"currency": o.Currency, "created_at": o.CreatedAt,
	}
}

func queryInt(c *gin.Context, key string, def int) int {
	v := c.Query(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

var _ = http.StatusOK
