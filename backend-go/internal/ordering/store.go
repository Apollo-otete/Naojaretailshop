package ordering

import (
	"context"
	"time"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/catalogue"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/shopping"
)

type OrderStore interface {
	CreateOrder(ctx context.Context, o *Order) error
	OrderByID(ctx context.Context, id int64) (Order, error)
	OrderByReference(ctx context.Context, ref string) (Order, error)
	OrdersByUser(ctx context.Context, userID int64) ([]Order, error)
	ListOrders(ctx context.Context, status *OrderStatus, page, pageSize int) ([]Order, int64, error)
	AddOrderItem(ctx context.Context, it *OrderItem) error
	OrderItems(ctx context.Context, orderID int64) ([]OrderItem, error)
	AddStatusHistory(ctx context.Context, h *OrderStatusHistory) error
	UpdateStatus(ctx context.Context, id int64, status OrderStatus, paidAt, cancelledAt *time.Time) error
	// ListExpiredPending returns orders that are awaiting payment and past expires_at (sweeper hook).
	ListExpiredPending(ctx context.Context, now time.Time, limit int) ([]Order, error)
	// ListStatusHistory returns status transitions for an order, oldest first.
	ListStatusHistory(ctx context.Context, orderID int64) ([]OrderStatusHistory, error)
}

type QuoteStore interface {
	CreateQuote(ctx context.Context, q *Quote) error
	QuoteByID(ctx context.Context, id int64) (Quote, error)
}

type IdempotencyStore interface {
	GetIdempotency(ctx context.Context, userID int64, key string) (IdempotencyRecord, bool, error)
	SaveIdempotency(ctx context.Context, rec IdempotencyRecord) error
	NextOrderReference(ctx context.Context, year int) (string, error)
}

type SettingsReader interface {
	IntSetting(ctx context.Context, key string, def int64) (int64, error)
}

type ProductReader interface {
	ProductByID(ctx context.Context, id int64) (catalogue.Product, error)
}

type CartReader interface {
	GetCartByUser(ctx context.Context, userID int64) (shopping.Cart, error)
	GetCartBySession(ctx context.Context, sessionHash string) (shopping.Cart, error)
	ListItems(ctx context.Context, cartID int64) ([]shopping.CartItem, error)
	ClearCart(ctx context.Context, cartID int64) error
}

type Transactor interface {
	RunInTx(ctx context.Context, work func(ctx context.Context) error) error
}

var _ = catalogue.Product{}
var _ = shopping.Cart{}
