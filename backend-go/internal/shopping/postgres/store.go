package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/database"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/shopping"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) (*Store, error) {
	if pool == nil {
		return nil, database.ErrNoPool
	}
	return &Store{pool: pool}, nil
}

var _ shopping.CartStore = (*Store)(nil)

type Tx struct{ runner *database.TxRunner }

func NewTx(pool *pgxpool.Pool) *Tx { return &Tx{runner: database.NewTxRunner(pool)} }

func (t *Tx) RunInTx(ctx context.Context, work func(ctx context.Context) error) error {
	return t.runner.RunInTx(ctx, work)
}

func (s *Store) exec(ctx context.Context) database.Execer { return database.DB(ctx, s.pool) }

const cartColumns = `id, user_id, session_token_hash, merged_into_cart_id, created_at, updated_at`

func (s *Store) scanCart(row pgx.Row) (shopping.Cart, error) {
	var c shopping.Cart
	var userID, merged *int64
	var sessionHash *string
	err := row.Scan(&c.ID, &userID, &sessionHash, &merged, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return shopping.Cart{}, err
	}
	c.UserID = userID
	c.SessionTokenHash = sessionHash
	c.MergedIntoCartID = merged
	return c, nil
}

func (s *Store) GetCartByUser(ctx context.Context, userID int64) (shopping.Cart, error) {
	row := s.exec(ctx).QueryRow(ctx, `SELECT `+cartColumns+` FROM commerce.carts WHERE user_id = $1 ORDER BY id DESC LIMIT 1`, userID)
	c, err := s.scanCart(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return shopping.Cart{}, shopping.ErrNoCart
	}
	if err != nil {
		return shopping.Cart{}, fmt.Errorf("reading cart: %w", err)
	}
	return c, nil
}

func (s *Store) GetCartBySession(ctx context.Context, sessionHash string) (shopping.Cart, error) {
	row := s.exec(ctx).QueryRow(ctx, `SELECT `+cartColumns+` FROM commerce.carts WHERE session_token_hash = $1`, sessionHash)
	c, err := s.scanCart(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return shopping.Cart{}, shopping.ErrNoCart
	}
	if err != nil {
		return shopping.Cart{}, fmt.Errorf("reading cart: %w", err)
	}
	return c, nil
}

func (s *Store) CreateCart(ctx context.Context, c *shopping.Cart) error {
	row := s.exec(ctx).QueryRow(ctx, `
		INSERT INTO commerce.carts (user_id, session_token_hash, merged_into_cart_id)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at`,
		c.UserID, c.SessionTokenHash, c.MergedIntoCartID)
	var created, updated time.Time
	if err := row.Scan(&c.ID, &created, &updated); err != nil {
		return fmt.Errorf("creating cart: %w", err)
	}
	c.CreatedAt, c.UpdatedAt = created, updated
	return nil
}

func (s *Store) GetCartByID(ctx context.Context, id int64) (shopping.Cart, error) {
	row := s.exec(ctx).QueryRow(ctx, `SELECT `+cartColumns+` FROM commerce.carts WHERE id = $1`, id)
	c, err := s.scanCart(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return shopping.Cart{}, shopping.ErrNoCart
	}
	if err != nil {
		return shopping.Cart{}, fmt.Errorf("reading cart: %w", err)
	}
	return c, nil
}

func (s *Store) AddItem(ctx context.Context, cartID, productID int64, qty int, priceSnapshot, addedAtPrice int64) error {
	_, err := s.exec(ctx).Exec(ctx, `
		INSERT INTO commerce.cart_items (cart_id, product_id, quantity, price_snapshot_cents, added_at_price_cents)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (cart_id, product_id) DO UPDATE
		SET quantity = commerce.cart_items.quantity + EXCLUDED.quantity,
		    price_snapshot_cents = EXCLUDED.price_snapshot_cents,
		    updated_at = now()`,
		cartID, productID, qty, priceSnapshot, addedAtPrice)
	if err != nil {
		return fmt.Errorf("adding cart item: %w", err)
	}
	// touch cart updated_at
	_, _ = s.exec(ctx).Exec(ctx, `UPDATE commerce.carts SET updated_at = now() WHERE id = $1`, cartID)
	return nil
}

func (s *Store) UpdateItemQty(ctx context.Context, cartID, productID int64, qty int) error {
	tag, err := s.exec(ctx).Exec(ctx, `
		UPDATE commerce.cart_items SET quantity = $3, updated_at = now()
		WHERE cart_id = $1 AND product_id = $2`, cartID, productID, qty)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return shopping.ErrNoCart
	}
	_, _ = s.exec(ctx).Exec(ctx, `UPDATE commerce.carts SET updated_at = now() WHERE id = $1`, cartID)
	return nil
}

func (s *Store) RemoveItem(ctx context.Context, cartID, productID int64) error {
	_, err := s.exec(ctx).Exec(ctx, `DELETE FROM commerce.cart_items WHERE cart_id = $1 AND product_id = $2`, cartID, productID)
	if err != nil {
		return err
	}
	_, _ = s.exec(ctx).Exec(ctx, `UPDATE commerce.carts SET updated_at = now() WHERE id = $1`, cartID)
	return nil
}

func (s *Store) ListItems(ctx context.Context, cartID int64) ([]shopping.CartItem, error) {
	rows, err := s.exec(ctx).Query(ctx, `
		SELECT cart_id, product_id, quantity, price_snapshot_cents, added_at_price_cents, created_at, updated_at
		FROM commerce.cart_items WHERE cart_id = $1 ORDER BY created_at ASC`, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []shopping.CartItem{}
	for rows.Next() {
		var it shopping.CartItem
		if err := rows.Scan(&it.CartID, &it.ProductID, &it.Quantity, &it.PriceSnapshotCents, &it.AddedAtPriceCents, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *Store) ClearCart(ctx context.Context, cartID int64) error {
	_, err := s.exec(ctx).Exec(ctx, `DELETE FROM commerce.cart_items WHERE cart_id = $1`, cartID)
	return err
}

func (s *Store) DeleteCart(ctx context.Context, id int64) error {
	_, err := s.exec(ctx).Exec(ctx, `DELETE FROM commerce.carts WHERE id = $1`, id)
	return err
}
