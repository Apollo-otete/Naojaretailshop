package shopping

import (
	"context"
	"time"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/catalogue"
)

type CartStore interface {
	GetCartByUser(ctx context.Context, userID int64) (Cart, error)
	GetCartBySession(ctx context.Context, sessionHash string) (Cart, error)
	CreateCart(ctx context.Context, c *Cart) error
	GetCartByID(ctx context.Context, id int64) (Cart, error)
	AddItem(ctx context.Context, cartID, productID int64, qty int, priceSnapshot, addedAtPrice int64) error
	UpdateItemQty(ctx context.Context, cartID, productID int64, qty int) error
	RemoveItem(ctx context.Context, cartID, productID int64) error
	ListItems(ctx context.Context, cartID int64) ([]CartItem, error)
	ClearCart(ctx context.Context, cartID int64) error
	DeleteCart(ctx context.Context, id int64) error
	// MergeGuestIntoUser moves an unmerged guest cart into the user's cart (idempotent, caps qty at 999).
	MergeGuestIntoUser(ctx context.Context, userID int64, sessionHash string) error
}

type Transactor interface {
	RunInTx(ctx context.Context, work func(ctx context.Context) error) error
}

// ProductReader is the minimal product access shopping needs. Satisfied structurally by
// catalogue's ProductStore.
type ProductReader interface {
	ProductByID(ctx context.Context, id int64) (catalogue.Product, error)
	ImagesByProduct(ctx context.Context, productID int64) ([]catalogue.ProductImage, error)
}

var _ = time.Now
