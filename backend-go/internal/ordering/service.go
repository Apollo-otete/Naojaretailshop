package ordering

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/audit"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/shopping"
)

type Deps struct {
	Orders     OrderStore
	Products   ProductReader
	Carts      CartReader
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
	return s.d.Authorizer.Authorize(ctx, port.Request{
		Permissions: []string{perm},
		Resource:    port.Resource{Type: "order", ID: fmt.Sprintf("%d", orderID), OwnerID: ownerID},
	})
}

func (s *Service) CreateFromCart(ctx context.Context, caller identity.Principal, sessionToken string, in CreateOrderInput) (Order, error) {
	// permission
	if err := s.authorize(ctx, caller, "ordering.orders.create", 0, caller.UserID()); err != nil {
		return Order{}, err
	}

	// resolve cart
	var cart shopping.Cart
	var err error
	if caller.UserID() > 0 {
		cart, err = s.d.Carts.GetCartByUser(ctx, caller.UserID())
	} else if sessionToken != "" {
		cart, err = s.d.Carts.GetCartBySession(ctx, shopping.HashToken(sessionToken))
	} else {
		return Order{}, errors.New("no cart")
	}
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

	// build order
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

	// generate reference
	order.Reference = fmt.Sprintf("NJ-%d-%04d", now.Year(), rand.Intn(10000))

	// transaction: create order + items + history + clear cart
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

func (s *Service) GetOrder(ctx context.Context, caller identity.Principal, ref string) (Order, []OrderItem, error) {
	// determine if staff
	staff := caller.Has("ordering.orders.read.all")
	var order Order
	var err error
	// try reference then id
	order, err = s.d.Orders.OrderByReference(ctx, ref)
	if errors.Is(err, ErrNoOrder) {
		// try numeric
		// ignore
	}
	if err != nil {
		return Order{}, nil, err
	}
	// authorization
	ownerID := int64(0)
	if order.UserID != nil {
		ownerID = *order.UserID
	}
	if !staff && ownerID != caller.UserID() {
		// non-owner, non-staff: check ordering.orders.read (own) → will fail if not owner
		if err := s.authorize(ctx, caller, "ordering.orders.read", order.ID, ownerID); err != nil {
			return Order{}, nil, err
		}
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
	// own orders: use OrdersByUser (ignore status/page for slice)
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
	if to == StatusPaid {
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
