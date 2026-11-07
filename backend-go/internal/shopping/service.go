package shopping

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/audit"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/catalogue"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
)

type Deps struct {
	Carts      CartStore
	Products   ProductReader
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
	if d.Carts == nil || d.Products == nil || d.Tx == nil {
		return nil, errors.New("shopping service is not fully wired")
	}
	if d.Clock == nil {
		d.Clock = time.Now
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Service{d: d}, nil
}

// resolveCart returns the cart for the current request, creating one if needed.
func (s *Service) resolveCart(ctx context.Context, caller *identity.Principal, sessionToken string) (Cart, error) {
	if caller != nil {
		c, err := s.d.Carts.GetCartByUser(ctx, caller.UserID())
		if errors.Is(err, ErrNoCart) {
			uid := caller.UserID()
			c = Cart{UserID: &uid}
			if err := s.d.Carts.CreateCart(ctx, &c); err != nil {
				return Cart{}, err
			}
			return c, nil
		}
		if err != nil {
			return Cart{}, err
		}
		return c, nil
	}
	// guest
	if strings.TrimSpace(sessionToken) == "" {
		return Cart{}, ErrGuestSessionRequired
	}
	hash := HashToken(sessionToken)
	c, err := s.d.Carts.GetCartBySession(ctx, hash)
	if errors.Is(err, ErrNoCart) {
		h := hash
		c = Cart{SessionTokenHash: &h}
		if err := s.d.Carts.CreateCart(ctx, &c); err != nil {
			return Cart{}, err
		}
		return c, nil
	}
	if err != nil {
		return Cart{}, err
	}
	return c, nil
}

func (s *Service) authorize(ctx context.Context, caller *identity.Principal, perm string) error {
	if caller == nil {
		return nil // guest: public cart operations
	}
	return s.d.Authorizer.Authorize(ctx, port.Request{
		Permissions: []string{perm},
		Resource:    port.Resource{Type: "cart", ID: fmt.Sprintf("%d", caller.UserID()), OwnerID: caller.UserID()},
	})
}

func (s *Service) ViewCart(ctx context.Context, caller *identity.Principal, sessionToken string) (CartView, error) {
	if caller != nil {
		if err := s.authorize(ctx, caller, "shopping.cart.read"); err != nil {
			return CartView{}, err
		}
	}
	cart, err := s.resolveCart(ctx, caller, sessionToken)
	if err != nil {
		return CartView{}, err
	}
	items, err := s.d.Carts.ListItems(ctx, cart.ID)
	if err != nil {
		return CartView{}, err
	}
	view := CartView{Items: []CartLine{}}
	for _, it := range items {
		prod, err := s.d.Products.ProductByID(ctx, it.ProductID)
		if err != nil {
			continue
		}
		var imgURL *string
		imgs, _ := s.d.Products.ImagesByProduct(ctx, prod.ID)
		if len(imgs) > 0 {
			for _, im := range imgs {
				if im.IsPrimary {
					u := im.URL
					imgURL = &u
					break
				}
			}
			if imgURL == nil && len(imgs) > 0 {
				u := imgs[0].URL
				imgURL = &u
			}
		}
		lineTotal := prod.PriceCents * int64(it.Quantity)
		view.Items = append(view.Items, CartLine{
			ProductID:         prod.ID,
			SKU:               prod.SKU,
			Name:              prod.Name,
			ImageURL:          imgURL,
			Quantity:          it.Quantity,
			UnitPriceCents:    prod.PriceCents,
			AddedAtPriceCents: it.AddedAtPriceCents,
			LineTotalCents:    lineTotal,
			AvailableQuantity: 0,
			Availability:      "in_stock",
		})
		view.SubtotalCents += lineTotal
		view.ItemCount += it.Quantity
	}
	return view, nil
}

func (s *Service) AddItem(ctx context.Context, caller *identity.Principal, sessionToken string, productID int64, qty int) error {
	if err := CheckQuantity(qty); err != nil {
		return err
	}
	if caller != nil {
		if err := s.authorize(ctx, caller, "shopping.cart.modify"); err != nil {
			return err
		}
	}
	prod, err := s.d.Products.ProductByID(ctx, productID)
	if err != nil {
		return err
	}
	if prod.PublicationState != catalogue.StatePublished {
		return errors.New("product is not available")
	}
	cart, err := s.resolveCart(ctx, caller, sessionToken)
	if err != nil {
		return err
	}
	return s.d.Carts.AddItem(ctx, cart.ID, productID, qty, prod.PriceCents, prod.PriceCents)
}

func (s *Service) UpdateItem(ctx context.Context, caller *identity.Principal, sessionToken string, productID int64, qty int) error {
	if qty == 0 {
		return s.RemoveItem(ctx, caller, sessionToken, productID)
	}
	if err := CheckQuantity(qty); err != nil {
		return err
	}
	if caller != nil {
		if err := s.authorize(ctx, caller, "shopping.cart.modify"); err != nil {
			return err
		}
	}
	cart, err := s.resolveCart(ctx, caller, sessionToken)
	if err != nil {
		return err
	}
	return s.d.Carts.UpdateItemQty(ctx, cart.ID, productID, qty)
}

func (s *Service) RemoveItem(ctx context.Context, caller *identity.Principal, sessionToken string, productID int64) error {
	if caller != nil {
		if err := s.authorize(ctx, caller, "shopping.cart.modify"); err != nil {
			return err
		}
	}
	cart, err := s.resolveCart(ctx, caller, sessionToken)
	if err != nil {
		return err
	}
	return s.d.Carts.RemoveItem(ctx, cart.ID, productID)
}

// ClearCart empties the caller's cart. Clearing is a write about the cart, so it carries the same
// `shopping.cart.modify` key the add/remove paths use; guests clear their own session cart the
// same way a signed-in caller clears theirs, and both answer "nothing was there" as success.
func (s *Service) ClearCart(ctx context.Context, caller *identity.Principal, sessionToken string) error {
	if err := s.authorize(ctx, caller, "shopping.cart.modify"); err != nil {
		return err
	}
	cart, err := s.resolveCart(ctx, caller, sessionToken)
	if err != nil {
		return err
	}
	return s.d.Carts.ClearCart(ctx, cart.ID)
}

// MergeGuestIntoUser merges an unmerged guest cart (identified by raw session token) into the
// authenticated user's cart. It is idempotent and safe to call when no guest cart exists.
func (s *Service) MergeGuestIntoUser(ctx context.Context, userID int64, guestSessionToken string) error {
	if userID <= 0 || strings.TrimSpace(guestSessionToken) == "" {
		return nil
	}
	hash := HashToken(guestSessionToken)
	return s.d.Tx.RunInTx(ctx, func(ctx context.Context) error {
		return s.d.Carts.MergeGuestIntoUser(ctx, userID, hash)
	})
}
