package ordering

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/audit"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/catalogue"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/inventory"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/shopping"
)

type Deps struct {
	Orders     OrderStore
	Products   ProductReader
	Carts      CartReader
	Quotes     QuoteStore
	Idem       IdempotencyStore
	Settings   SettingsReader
	Inventory  inventory.Store
	Authorizer port.Authorizer
	Tx         Transactor
	Audit      audit.Writer
	Clock      func() time.Time
	Logger     *slog.Logger
}

type Service struct {
	d Deps
}

func NewService(d Deps) (*Service, error) {
	if d.Orders == nil || d.Products == nil || d.Carts == nil || d.Tx == nil {
		return nil, errors.New("ordering service is not fully wired")
	}
	if d.Clock == nil {
		d.Clock = time.Now
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Service{d: d}, nil
}

func (s *Service) authorize(ctx context.Context, caller identity.Principal, perm string, orderID int64, ownerID int64) error {
	if s.d.Authorizer == nil {
		return nil
	}
	return s.d.Authorizer.Authorize(ctx, port.Request{
		Permissions: []string{perm},
		Resource:    port.Resource{Type: "order", ID: fmt.Sprintf("%d", orderID), OwnerID: ownerID},
	})
}

func (s *Service) callerPtr(caller *identity.Principal) (int64, *int64, string) {
	if caller == nil {
		return 0, nil, ""
	}
	uid := caller.UserID()
	return uid, &uid, ""
}

func (s *Service) resolveCart(ctx context.Context, caller *identity.Principal, sessionToken string) (shopping.Cart, error) {
	if caller != nil && caller.UserID() > 0 {
		return s.d.Carts.GetCartByUser(ctx, caller.UserID())
	}
	if sessionToken != "" {
		return s.d.Carts.GetCartBySession(ctx, shopping.HashToken(sessionToken))
	}
	return shopping.Cart{}, errors.New("no cart")
}

func (s *Service) CreateFromCart(ctx context.Context, caller identity.Principal, sessionToken string, in CreateOrderInput) (Order, error) {
	if err := s.authorize(ctx, caller, "ordering.orders.create", 0, caller.UserID()); err != nil {
		return Order{}, err
	}

	cart, err := s.resolveCart(ctx, &caller, sessionToken)
	if err != nil {
		return Order{}, err
	}
	items, err := s.d.Carts.ListItems(ctx, cart.ID)
	if err != nil {
		return Order{}, err
	}
	if len(items) == 0 {
		return Order{}, errors.New("cart is empty")
	}

	now := s.d.Clock()
	uid := caller.UserID()
	order := Order{
		UserID:               &uid,
		Status:               StatusPendingPayment,
		PaymentStatus:        PayPending,
		Currency:             "KES",
		DeliveryAddress:      in.DeliveryAddress,
		ContactPhone:         in.ContactPhone,
		DeliveryInstructions: in.DeliveryInstructions,
		DeliveryFeeCents:     in.DeliveryFeeCents,
		ExpiresAt:            timePtr(now.Add(30 * time.Minute)),
	}
	var subtotal int64
	orderItems := make([]OrderItem, 0, len(items))
	for _, it := range items {
		prod, err := s.d.Products.ProductByID(ctx, it.ProductID)
		if err != nil {
			return Order{}, err
		}
		lineTotal := prod.PriceCents * int64(it.Quantity)
		subtotal += lineTotal
		orderItems = append(orderItems, OrderItem{
			ProductID:      &prod.ID,
			ProductName:    prod.Name,
			SKU:            prod.SKU,
			Quantity:       it.Quantity,
			UnitPriceCents: prod.PriceCents,
			LineTotalCents: lineTotal,
		})
	}
	order.SubtotalCents = subtotal
	order.TotalCents = subtotal + in.DeliveryFeeCents
	order.Reference = fmt.Sprintf("NJ-%d-%04d", now.Year(), rand.Intn(10000))

	err = s.d.Tx.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.d.Orders.CreateOrder(ctx, &order); err != nil {
			return err
		}
		for i := range orderItems {
			orderItems[i].OrderID = order.ID
			if err := s.d.Orders.AddOrderItem(ctx, &orderItems[i]); err != nil {
				return err
			}
		}
		hs := "pending_payment"
		aid := caller.UserID()
		h := OrderStatusHistory{
			OrderID:    order.ID,
			FromStatus: nil,
			ToStatus:   StatusPendingPayment,
			Reason:     &hs,
			ActorID:    &aid,
			OccurredAt: now,
		}
		if err := s.d.Orders.AddStatusHistory(ctx, &h); err != nil {
			return err
		}
		return s.d.Carts.ClearCart(ctx, cart.ID)
	})
	if err != nil {
		return Order{}, err
	}
	return order, nil
}

// ── P0: contract checkout (quote + idempotent placement with reservation) ───

func (s *Service) CreateQuote(ctx context.Context, caller *identity.Principal, sessionToken string, req QuoteRequest) (Quote, error) {
	if err := req.Validate(); err != nil {
		return Quote{}, apierr.Validation(apierr.Detail{Field: "quote", Issue: err.Error()})
	}
	if s.d.Quotes == nil || s.d.Settings == nil {
		return Quote{}, apierr.Internal(errors.New("checkout is not fully configured"))
	}
	if caller != nil && s.d.Authorizer != nil {
		if err := s.d.Authorizer.Authorize(ctx, port.Request{
			Permissions: []string{"ordering.orders.create"},
			Resource:    port.Resource{Type: "order", ID: "0", OwnerID: caller.UserID()},
		}); err != nil {
			return Quote{}, err
		}
	}
	cart, err := s.resolveCart(ctx, caller, sessionToken)
	if err != nil {
		return Quote{}, apierr.BadRequest("A cart with items is required.")
	}
	items, err := s.d.Carts.ListItems(ctx, cart.ID)
	if err != nil {
		return Quote{}, err
	}
	if len(items) == 0 {
		return Quote{}, apierr.Validation(apierr.Detail{Field: "cart", Issue: "cart is empty"})
	}

	deliveryFee, _ := s.d.Settings.IntSetting(ctx, "delivery_standard_fee_cents", 15000)
	if req.FulfilmentMethod == FulfilmentPickup {
		deliveryFee = 0
	}
	ttlMin, _ := s.d.Settings.IntSetting(ctx, "reservation_ttl_minutes", 30)
	if ttlMin <= 0 {
		ttlMin = 30
	}

	lines := make([]QuoteLine, 0, len(items))
	var subtotal int64
	for _, it := range items {
		prod, err := s.d.Products.ProductByID(ctx, it.ProductID)
		if err != nil {
			return Quote{}, err
		}
		if prod.PublicationState != catalogue.StatePublished {
			return Quote{}, apierr.Conflict(apierr.CodeConflict, fmt.Sprintf("Product %q is not available.", prod.Name))
		}
		lt := prod.PriceCents * int64(it.Quantity)
		subtotal += lt
		lines = append(lines, QuoteLine{
			ProductID: prod.ID, Name: prod.Name, SKU: prod.SKU,
			UnitPriceCents: prod.PriceCents, Quantity: it.Quantity, LineTotalCents: lt,
		})
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].ProductID < lines[j].ProductID })
	discount := int64(0)
	total := subtotal + deliveryFee - discount
	if total < 0 {
		total = 0
	}
	now := s.d.Clock()
	q := Quote{
		FulfilmentMethod: req.FulfilmentMethod,
		ContactName:      strings.TrimSpace(req.ContactName),
		ContactPhone:     NormalizeKEPhone(req.ContactPhone),
		ContactEmail:     req.ContactEmail,
		Delivery:         req.Delivery,
		PickupWindowID:   req.PickupWindowID,
		Notes:            req.Notes,
		Lines:            lines,
		Totals:           Totals{SubtotalCents: subtotal, DeliveryFeeCents: deliveryFee, DiscountCents: discount, TotalCents: total, Currency: "KES"},
		DeliveryZone:     nil,
		StoreWindow:      nil,
		ExpiresAt:        now.Add(time.Duration(ttlMin) * time.Minute),
		PricingHash:      QuotePricingHash(lines, deliveryFee, discount, total, req.FulfilmentMethod),
		PricingChanged:   false,
	}
	if caller != nil {
		uid := caller.UserID()
		q.UserID = &uid
	} else if sessionToken != "" {
		h := shopping.HashToken(sessionToken)
		q.SessionTokenHash = &h
	}
	if err := s.d.Quotes.CreateQuote(ctx, &q); err != nil {
		return Quote{}, err
	}
	return q, nil
}

// PlaceOrder is the contract T-01 operation: idempotent placement with atomic stock reservation.
func (s *Service) PlaceOrder(ctx context.Context, caller identity.Principal, idemKey string, req PlaceOrderRequest) (ContractOrderDetail, bool, error) {
	if idemKey == "" {
		return ContractOrderDetail{}, false, apierr.BadRequest("Idempotency-Key header is required.")
	}
	if req.QuoteID <= 0 {
		return ContractOrderDetail{}, false, apierr.Validation(apierr.Detail{Field: "quote_id", Issue: "quote_id is required"})
	}
	if s.d.Quotes == nil || s.d.Idem == nil || s.d.Inventory == nil || s.d.Settings == nil {
		return ContractOrderDetail{}, false, apierr.Internal(errors.New("checkout is not fully configured"))
	}
	if err := s.authorize(ctx, caller, "ordering.orders.create", 0, caller.UserID()); err != nil {
		return ContractOrderDetail{}, false, err
	}

	reqHash := PlaceRequestHash(req)
	// Idempotency replay/conflict.
	if rec, ok, err := s.d.Idem.GetIdempotency(ctx, caller.UserID(), idemKey); err != nil {
		return ContractOrderDetail{}, false, err
	} else if ok {
		if rec.RequestHash != reqHash {
			return ContractOrderDetail{}, false, apierr.IdempotencyConflict("This Idempotency-Key was already used with a different request body.")
		}
		var detail ContractOrderDetail
		if err := json.Unmarshal(rec.ResponseBody, &detail); err == nil && rec.OrderReference != nil {
			return detail, true, nil
		}
		// Fallback: load by reference.
		if rec.OrderReference != nil {
			o, items, err := s.GetOrder(ctx, caller, *rec.OrderReference)
			if err == nil {
				return ContractDetail(o, items), true, nil
			}
		}
	}

	quote, err := s.d.Quotes.QuoteByID(ctx, req.QuoteID)
	if err != nil {
		if errors.Is(err, ErrNoQuote) {
			return ContractOrderDetail{}, false, apierr.NotFound("quote")
		}
		return ContractOrderDetail{}, false, err
	}
	// Ownership: quote must belong to caller (or be an unowned guest quote only if we supported guest
	// placement; P0 requires authenticated placement, so enforce user match).
	if quote.UserID == nil || *quote.UserID != caller.UserID() {
		return ContractOrderDetail{}, false, apierr.NotFound("quote")
	}
	now := s.d.Clock()
	if !quote.ExpiresAt.After(now) {
		return ContractOrderDetail{}, false, apierr.QuoteStale(nil)
	}

	// Reprice and detect stale/moved prices.
	stale, repricedLines, repricedTotals := s.repriceQuote(ctx, quote)
	if len(stale) > 0 && !req.AcceptPriceChange {
		return ContractOrderDetail{}, false, apierr.QuoteStale(stale)
	}
	// If repriced, we honor the repriced (current) figures with explicit acceptance.
	useLines := quote.Lines
	useTotals := quote.Totals
	pricingChanged := false
	if repricedTotals.TotalCents != quote.Totals.TotalCents || repricedTotals.SubtotalCents != quote.Totals.SubtotalCents || repricedTotals.DeliveryFeeCents != quote.Totals.DeliveryFeeCents {
		if !req.AcceptPriceChange {
			return ContractOrderDetail{}, false, apierr.QuoteStale(stale)
		}
		useLines = repricedLines
		useTotals = repricedTotals
		pricingChanged = true
	}
	_ = pricingChanged

	ttlMin, _ := s.d.Settings.IntSetting(ctx, "reservation_ttl_minutes", 30)
	if ttlMin <= 0 {
		ttlMin = 30
	}
	expiresAt := now.Add(time.Duration(ttlMin) * time.Minute)

	ref, err := s.d.Idem.NextOrderReference(ctx, now.Year())
	if err != nil {
		return ContractOrderDetail{}, false, err
	}

	reserveLines := make([]inventory.ReserveLine, 0, len(useLines))
	for _, ln := range useLines {
		reserveLines = append(reserveLines, inventory.ReserveLine{ProductID: ln.ProductID, Name: ln.Name, Quantity: ln.Quantity})
	}
	sort.Slice(reserveLines, func(i, j int) bool { return reserveLines[i].ProductID < reserveLines[j].ProductID })

	uid := caller.UserID()
	fm := string(quote.FulfilmentMethod)
	var deliveryAddr *string
	var zoneID *int64
	var area, county, landmark *string
	if quote.Delivery != nil {
		if quote.Delivery.AddressLine != "" {
			a := quote.Delivery.AddressLine
			deliveryAddr = &a
		}
		if quote.Delivery.ZoneID > 0 {
			z := quote.Delivery.ZoneID
			zoneID = &z
		}
		area, county, landmark = quote.Delivery.Area, quote.Delivery.County, quote.Delivery.Landmark
	}
	notes := req.Notes
	if notes == nil {
		notes = quote.Notes
	}
	contactName := quote.ContactName
	contactPhone := quote.ContactPhone
	order := Order{
		Reference: ref, UserID: &uid, Status: StatusPending, PaymentStatus: PayPending,
		SubtotalCents: useTotals.SubtotalCents, DeliveryFeeCents: useTotals.DeliveryFeeCents, TotalCents: useTotals.TotalCents,
		Currency: "KES", DeliveryAddress: deliveryAddr, ContactPhone: &contactPhone, ExpiresAt: &expiresAt,
		FulfilmentMethod: &fm, QuoteID: &quote.ID, ContactName: &contactName, ContactEmail: quote.ContactEmail,
		DeliveryZoneID: zoneID, DeliveryArea: area, DeliveryCounty: county, DeliveryLandmark: landmark,
		PickupWindowID: quote.PickupWindowID, Notes: notes,
	}
	orderItems := make([]OrderItem, 0, len(useLines))
	for _, ln := range useLines {
		pid := ln.ProductID
		orderItems = append(orderItems, OrderItem{
			ProductID: &pid, ProductName: ln.Name, SKU: ln.SKU, Quantity: ln.Quantity,
			UnitPriceCents: ln.UnitPriceCents, LineTotalCents: ln.LineTotalCents,
		})
	}

	var placedDetail ContractOrderDetail
	err = s.d.Tx.RunInTx(ctx, func(ctx context.Context) error {
		// 1) Create order to get ID.
		if err := s.d.Orders.CreateOrder(ctx, &order); err != nil {
			return err
		}
		// 2) Reserve stock against this order (atomic conditional updates). Shortfall => conflict, rollback.
		for i := range reserveLines {
			reserveLines[i].OrderID = order.ID
		}
		if shorts, rerr := s.d.Inventory.Reserve(ctx, order.ID, &expiresAt, reserveLines); rerr != nil || len(shorts) > 0 {
			if len(shorts) > 0 || errors.Is(rerr, inventory.ErrShortfall) {
				items := make([]apierr.InsufficientItem, 0, len(shorts))
				for _, sh := range shorts {
					items = append(items, apierr.InsufficientItem{ProductID: sh.ProductID, Name: sh.Name, Requested: sh.Requested, Available: sh.Available})
				}
				return apierr.StockInsufficient(items)
			}
			return rerr
		}
		// 3) Items + history.
		for i := range orderItems {
			orderItems[i].OrderID = order.ID
			if err := s.d.Orders.AddOrderItem(ctx, &orderItems[i]); err != nil {
				return err
			}
		}
		aid := caller.UserID()
		reason := "order placed (stock reserved)"
		to := StatusPending
		h := OrderStatusHistory{OrderID: order.ID, FromStatus: nil, ToStatus: to, Reason: &reason, ActorID: &aid, OccurredAt: now}
		if err := s.d.Orders.AddStatusHistory(ctx, &h); err != nil {
			return err
		}
		// 4) Clear cart.
		if caller.UserID() > 0 {
			if cart, cerr := s.d.Carts.GetCartByUser(ctx, caller.UserID()); cerr == nil {
				_ = s.d.Carts.ClearCart(ctx, cart.ID)
			}
		}
		placedDetail = ContractDetail(order, orderItems)
		body, _ := json.Marshal(placedDetail)
		refCopy := order.Reference
		if err := s.d.Idem.SaveIdempotency(ctx, IdempotencyRecord{
			UserID: caller.UserID(), Key: idemKey, RequestHash: reqHash,
			StatusCode: http.StatusCreated, ResponseBody: body, OrderReference: &refCopy,
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return ContractOrderDetail{}, false, err
	}
	return placedDetail, false, nil
}

func (s *Service) repriceQuote(ctx context.Context, q Quote) ([]apierr.StaleLine, []QuoteLine, Totals) {
	lines := make([]QuoteLine, 0, len(q.Lines))
	var stale []apierr.StaleLine
	var subtotal int64
	for _, ln := range q.Lines {
		prod, err := s.d.Products.ProductByID(ctx, ln.ProductID)
		if err != nil {
			stale = append(stale, apierr.StaleLine{ProductID: ln.ProductID, Code: "UNPUBLISHED"})
			continue
		}
		if prod.PublicationState != catalogue.StatePublished {
			stale = append(stale, apierr.StaleLine{ProductID: ln.ProductID, Code: "UNPUBLISHED"})
			continue
		}
		if prod.PriceCents != ln.UnitPriceCents {
			from, to := ln.UnitPriceCents, prod.PriceCents
			code := "PRICE_INCREASED"
			if to < from {
				code = "PRICE_DECREASED"
			}
			stale = append(stale, apierr.StaleLine{ProductID: ln.ProductID, Code: code, FromCents: &from, ToCents: &to})
		}
		lt := prod.PriceCents * int64(ln.Quantity)
		subtotal += lt
		lines = append(lines, QuoteLine{ProductID: prod.ID, Name: prod.Name, SKU: prod.SKU, UnitPriceCents: prod.PriceCents, Quantity: ln.Quantity, LineTotalCents: lt})
	}
	deliveryFee := q.Totals.DeliveryFeeCents
	// refresh delivery fee from settings for accuracy
	if s.d.Settings != nil {
		if df, err := s.d.Settings.IntSetting(ctx, "delivery_standard_fee_cents", deliveryFee); err == nil {
			if q.FulfilmentMethod == FulfilmentPickup {
				deliveryFee = 0
			} else {
				deliveryFee = df
			}
		}
	}
	total := subtotal + deliveryFee - q.Totals.DiscountCents
	if total < 0 {
		total = 0
	}
	return stale, lines, Totals{SubtotalCents: subtotal, DeliveryFeeCents: deliveryFee, DiscountCents: q.Totals.DiscountCents, TotalCents: total, Currency: "KES"}
}

func (s *Service) GetOrder(ctx context.Context, caller identity.Principal, ref string) (Order, []OrderItem, error) {
	staff := caller.Has("ordering.orders.read.all")
	order, err := s.d.Orders.OrderByReference(ctx, ref)
	if err != nil {
		return Order{}, nil, err
	}
	ownerID := int64(0)
	if order.UserID != nil {
		ownerID = *order.UserID
	}
	if !staff && ownerID != caller.UserID() {
		// Hide existence from non-owners/non-staff (contract: 404, not 403).
		return Order{}, nil, ErrNoOrder
	}
	items, err := s.d.Orders.OrderItems(ctx, order.ID)
	if err != nil {
		return Order{}, nil, err
	}
	return order, items, nil
}

func (s *Service) ListOrders(ctx context.Context, caller identity.Principal, status *OrderStatus, page, pageSize int) ([]Order, int64, error) {
	if caller.Has("ordering.orders.read.all") {
		return s.d.Orders.ListOrders(ctx, status, page, pageSize)
	}
	orders, err := s.d.Orders.OrdersByUser(ctx, caller.UserID())
	if err != nil {
		return nil, 0, err
	}
	return orders, int64(len(orders)), nil
}

func (s *Service) ProcessStatus(ctx context.Context, caller identity.Principal, ref string, to OrderStatus, reason string) (Order, error) {
	if !caller.Has("ordering.orders.process") && !caller.Has("ordering.orders.status.override") {
		return Order{}, errors.New("permission denied")
	}
	order, err := s.d.Orders.OrderByReference(ctx, ref)
	if err != nil {
		return Order{}, err
	}
	from := string(order.Status)
	now := s.d.Clock()
	var paidAt, cancelledAt *time.Time
	if to == StatusPaid || to == StatusConfirmed {
		paidAt = &now
		order.PaymentStatus = PayPaid
	}
	if to == StatusCancelled {
		cancelledAt = &now
	}
	err = s.d.Tx.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.d.Orders.UpdateStatus(ctx, order.ID, to, paidAt, cancelledAt); err != nil {
			return err
		}
		r := reason
		aid := caller.UserID()
		h := OrderStatusHistory{
			OrderID:    order.ID,
			FromStatus: &from,
			ToStatus:   to,
			Reason:     &r,
			ActorID:    &aid,
			OccurredAt: now,
		}
		return s.d.Orders.AddStatusHistory(ctx, &h)
	})
	if err != nil {
		return Order{}, err
	}
	order.Status = to
	if paidAt != nil {
		order.PaidAt = paidAt
	}
	if cancelledAt != nil {
		order.CancelledAt = cancelledAt
	}
	return order, nil
}

func timePtr(t time.Time) *time.Time { return &t }

var _ = rand.Intn

// ExpirePendingOrders releases reservations and marks expired any orders past expires_at that are
// still awaiting payment. Intended as a sweeper/cron hook; idempotent and safe to run repeatedly.
func (s *Service) ExpirePendingOrders(ctx context.Context, limit int) (int, error) {
	if s.d.Orders == nil {
		return 0, nil
	}
	now := s.d.Clock()
	orders, err := s.d.Orders.ListExpiredPending(ctx, now, limit)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, o := range orders {
		err := s.d.Tx.RunInTx(ctx, func(ctx context.Context) error {
			if s.d.Inventory != nil {
				_ = s.d.Inventory.ReleaseOrder(ctx, o.ID, "reservation_expired")
			}
			at := now
			return s.d.Orders.UpdateStatus(ctx, o.ID, StatusExpired, nil, &at)
		})
		if err != nil {
			if s.d.Logger != nil {
				s.d.Logger.Warn("expire pending order failed", "order_id", o.ID, "error", err)
			}
			continue
		}
		_ = s.d.Orders.AddStatusHistory(ctx, &OrderStatusHistory{
			OrderID: o.ID, ToStatus: StatusExpired, Reason: strPtr("reservation expired"), OccurredAt: now,
		})
		n++
	}
	return n, nil
}

func strPtr(s string) *string { return &s }

func normalizeTrackPhone(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "+")
	if strings.HasPrefix(s, "0") && len(s) == 10 {
		s = "254" + s[1:]
	}
	// keep digits only
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			b = append(b, s[i])
		}
	}
	return string(b)
}

func phonesMatch(stored, provided string) bool {
	a, b := normalizeTrackPhone(stored), normalizeTrackPhone(provided)
	if a == "" || b == "" {
		return false
	}
	return a == b
}

// TrackOrder verifies reference+phone and returns a minimal non-leaking projection.
// Any mismatch is indistinguishable from "no such order".
func (s *Service) TrackOrder(ctx context.Context, ref, phone string) (OrderTracking, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.TrimSpace(phone) == "" {
		return OrderTracking{}, ErrNoOrder
	}
	o, err := s.d.Orders.OrderByReference(ctx, ref)
	if err != nil {
		if errors.Is(err, ErrNoOrder) {
			return OrderTracking{}, ErrNoOrder
		}
		return OrderTracking{}, ErrNoOrder // anti-enumeration on unexpected errors too? keep safe
	}
	storedPhone := ""
	if o.ContactPhone != nil {
		storedPhone = *o.ContactPhone
	}
	if !phonesMatch(storedPhone, phone) {
		return OrderTracking{}, ErrNoOrder
	}
	hist, _ := s.d.Orders.ListStatusHistory(ctx, o.ID)
	timeline := make([]TrackTimelineEntry, 0, len(hist)+1)
	if len(hist) == 0 {
		timeline = append(timeline, TrackTimelineEntry{Status: string(o.Status), OccurredAt: o.CreatedAt})
	} else {
		for _, h := range hist {
			timeline = append(timeline, TrackTimelineEntry{Status: string(h.ToStatus), OccurredAt: h.OccurredAt})
		}
	}
	method := "delivery"
	if o.FulfilmentMethod != nil && *o.FulfilmentMethod != "" {
		method = *o.FulfilmentMethod
	}
	return OrderTracking{
		Reference:        o.Reference,
		Status:           string(o.Status),
		PaymentStatus:    string(o.PaymentStatus),
		FulfilmentMethod: method,
		PlacedAt:         o.CreatedAt,
		ExpectedWindow:   nil,
		Timeline:         timeline,
	}, nil
}
