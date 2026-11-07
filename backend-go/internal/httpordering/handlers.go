package httpordering

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpjson"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/ordering"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
)

type TrackLimiter interface {
	Allow(key string, now time.Time) bool
	RetryAfter(key string, now time.Time) time.Duration
}

type Service interface {
	// Contract P0: idempotent order placement from a quote.
	PlaceOrder(ctx context.Context, caller identity.Principal, idemKey string, req ordering.PlaceOrderRequest) (ordering.ContractOrderDetail, bool, error)
	GetOrder(ctx context.Context, caller identity.Principal, ref string) (ordering.Order, []ordering.OrderItem, error)
	ListOrders(ctx context.Context, caller identity.Principal, status *ordering.OrderStatus, page, pageSize int) ([]ordering.Order, int64, error)
	ProcessStatus(ctx context.Context, caller identity.Principal, ref string, to ordering.OrderStatus, reason string) (ordering.Order, error)
	TrackOrder(ctx context.Context, ref, phone string) (ordering.OrderTracking, error)
}

type Deps struct {
	Ordering     Service
	Authorizer   port.Authorizer
	Logger       *slog.Logger
	TrackLimiter TrackLimiter
}

func Mount(r gin.IRouter, d Deps) {
	r.POST("/orders", placeOrder(d))
	r.POST("/orders/track", trackOrder(d)) // public, verified, rate-limited (G3) - before :reference GET ambiguity irrelevant (different method/shape)
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

func placeOrder(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := caller(c)
		if !ok {
			httpjson.Write(c, d.Logger, apierr.Unauthorized("Sign in to place an order."))
			return
		}
		idemKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
		if idemKey == "" {
			httpjson.Write(c, d.Logger, apierr.BadRequest("Idempotency-Key header is required."))
			return
		}
		var req ordering.PlaceOrderRequest
		if err := decode(c, &req); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		detail, replayed, err := d.Ordering.PlaceOrder(c.Request.Context(), p, idemKey, req)
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		_ = replayed // same semantic response; 201 is fine for idempotent replay in this API
		httpjson.Created(c, detail)
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
		httpjson.OK(c, ordering.ContractDetail(o, items))
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

var _ = time.Now

func trackOrder(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.TrackLimiter != nil && !d.TrackLimiter.Allow(c.ClientIP(), time.Now()) {
			retry := int(d.TrackLimiter.RetryAfter(c.ClientIP(), time.Now()).Seconds())
			if retry < 1 {
				retry = 1
			}
			f := apierr.New(http.StatusTooManyRequests, apierr.CodeRateLimited, "Too many requests have been made from this address. Try again later.")
			f.RetryIn = retry
			httpjson.Write(c, d.Logger, f)
			return
		}
		var in ordering.TrackRequest
		if err := decode(c, &in); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		if strings.TrimSpace(in.Reference) == "" || strings.TrimSpace(in.Phone) == "" {
			httpjson.Write(c, d.Logger, apierr.NotFound("order"))
			return
		}
		out, err := d.Ordering.TrackOrder(c.Request.Context(), in.Reference, in.Phone)
		if err != nil {
			// Indistinguishable from "no such order" (anti-enumeration).
			httpjson.Write(c, d.Logger, apierr.NotFound("order"))
			return
		}
		httpjson.OK(c, out)
	}
}
