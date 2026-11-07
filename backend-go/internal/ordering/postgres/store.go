package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/database"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/ordering"
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

var _ ordering.OrderStore = (*Store)(nil)

type Tx struct{ runner *database.TxRunner }

func NewTx(pool *pgxpool.Pool) *Tx { return &Tx{runner: database.NewTxRunner(pool)} }

func (t *Tx) RunInTx(ctx context.Context, work func(ctx context.Context) error) error {
	return t.runner.RunInTx(ctx, work)
}

func (s *Store) exec(ctx context.Context) database.Execer { return database.DB(ctx, s.pool) }

const orderColumns = `id, reference, user_id, status, payment_status, subtotal_cents, delivery_fee_cents, total_cents, currency, delivery_address, contact_phone, delivery_instructions, expires_at, created_at, updated_at, paid_at, cancelled_at`

func (s *Store) scanOrder(row pgx.Row) (ordering.Order, error) {
	var o ordering.Order
	var userID *int64
	var deliveryAddr, contactPhone, deliveryInstr *string
	var expiresAt, paidAt, cancelledAt *time.Time
	err := row.Scan(
		&o.ID, &o.Reference, &userID, &o.Status, &o.PaymentStatus, &o.SubtotalCents, &o.DeliveryFeeCents,
		&o.TotalCents, &o.Currency, &deliveryAddr, &contactPhone, &deliveryInstr, &expiresAt, &o.CreatedAt,
		&o.UpdatedAt, &paidAt, &cancelledAt,
	)
	if err != nil {
		return ordering.Order{}, err
	}
	o.UserID = userID
	o.DeliveryAddress = deliveryAddr
	o.ContactPhone = contactPhone
	o.DeliveryInstructions = deliveryInstr
	o.ExpiresAt = expiresAt
	o.PaidAt = paidAt
	o.CancelledAt = cancelledAt
	return o, nil
}

func (s *Store) CreateOrder(ctx context.Context, o *ordering.Order) error {
	row := s.exec(ctx).QueryRow(ctx, `
		INSERT INTO commerce.orders (reference, user_id, status, payment_status, subtotal_cents, delivery_fee_cents, total_cents, currency, delivery_address, contact_phone, delivery_instructions, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, created_at, updated_at`,
		o.Reference, o.UserID, string(o.Status), string(o.PaymentStatus), o.SubtotalCents, o.DeliveryFeeCents,
		o.TotalCents, o.Currency, o.DeliveryAddress, o.ContactPhone, o.DeliveryInstructions, o.ExpiresAt)
	var created, updated time.Time
	if err := row.Scan(&o.ID, &created, &updated); err != nil {
		return fmt.Errorf("creating order: %w", err)
	}
	o.CreatedAt, o.UpdatedAt = created, updated
	return nil
}

func (s *Store) OrderByID(ctx context.Context, id int64) (ordering.Order, error) {
	row := s.exec(ctx).QueryRow(ctx, `SELECT `+orderColumns+` FROM commerce.orders WHERE id = $1`, id)
	o, err := s.scanOrder(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return ordering.Order{}, ordering.ErrNoOrder
	}
	if err != nil {
		return ordering.Order{}, fmt.Errorf("reading order: %w", err)
	}
	return o, nil
}

func (s *Store) OrderByReference(ctx context.Context, ref string) (ordering.Order, error) {
	row := s.exec(ctx).QueryRow(ctx, `SELECT `+orderColumns+` FROM commerce.orders WHERE reference = $1`, ref)
	o, err := s.scanOrder(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return ordering.Order{}, ordering.ErrNoOrder
	}
	if err != nil {
		return ordering.Order{}, fmt.Errorf("reading order: %w", err)
	}
	return o, nil
}

func (s *Store) OrdersByUser(ctx context.Context, userID int64) ([]ordering.Order, error) {
	rows, err := s.exec(ctx).Query(ctx, `SELECT `+orderColumns+` FROM commerce.orders WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ordering.Order{}
	for rows.Next() {
		o, err := s.scanOrder(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, o)
	}
	return items, rows.Err()
}

func (s *Store) ListOrders(ctx context.Context, status *ordering.OrderStatus, page, pageSize int) ([]ordering.Order, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	where := ""
	args := []any{}
	if status != nil {
		where = "WHERE status = $1"
		args = append(args, string(*status))
	}
	var total int64
	countQ := "SELECT count(*) FROM commerce.orders " + where
	if err := s.exec(ctx).QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)
	q := fmt.Sprintf(`SELECT %s FROM commerce.orders %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		orderColumns, where, len(args)-1, len(args))
	rows, err := s.exec(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []ordering.Order{}
	for rows.Next() {
		o, err := s.scanOrder(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, o)
	}
	return items, total, rows.Err()
}

func (s *Store) AddOrderItem(ctx context.Context, it *ordering.OrderItem) error {
	row := s.exec(ctx).QueryRow(ctx, `
		INSERT INTO commerce.order_items (order_id, product_id, product_name, sku, quantity, unit_price_cents, line_total_cents)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, created_at`,
		it.OrderID, it.ProductID, it.ProductName, it.SKU, it.Quantity, it.UnitPriceCents, it.LineTotalCents)
	var created time.Time
	if err := row.Scan(&it.ID, &created); err != nil {
		return fmt.Errorf("adding order item: %w", err)
	}
	it.CreatedAt = created
	return nil
}

func (s *Store) OrderItems(ctx context.Context, orderID int64) ([]ordering.OrderItem, error) {
	rows, err := s.exec(ctx).Query(ctx, `
		SELECT id, order_id, product_id, product_name, sku, quantity, unit_price_cents, line_total_cents, created_at
		FROM commerce.order_items WHERE order_id = $1 ORDER BY id ASC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ordering.OrderItem{}
	for rows.Next() {
		var it ordering.OrderItem
		var productID *int64
		if err := rows.Scan(&it.ID, &it.OrderID, &productID, &it.ProductName, &it.SKU, &it.Quantity, &it.UnitPriceCents, &it.LineTotalCents, &it.CreatedAt); err != nil {
			return nil, err
		}
		it.ProductID = productID
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *Store) AddStatusHistory(ctx context.Context, h *ordering.OrderStatusHistory) error {
	row := s.exec(ctx).QueryRow(ctx, `
		INSERT INTO commerce.order_status_history (order_id, from_status, to_status, reason, actor_id, occurred_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id`,
		h.OrderID, h.FromStatus, string(h.ToStatus), h.Reason, h.ActorID, h.OccurredAt)
	if err := row.Scan(&h.ID); err != nil {
		return fmt.Errorf("adding status history: %w", err)
	}
	return nil
}

func (s *Store) UpdateStatus(ctx context.Context, id int64, status ordering.OrderStatus, paidAt, cancelledAt *time.Time) error {
	_, err := s.exec(ctx).Exec(ctx, `
		UPDATE commerce.orders
		SET status=$2,
		    payment_status = CASE WHEN $2='paid' THEN 'paid' WHEN $2 IN ('failed','expired','cancelled') THEN 'failed' ELSE payment_status END,
		    paid_at=COALESCE($3, paid_at), cancelled_at=COALESCE($4, cancelled_at), updated_at=now()
		WHERE id=$1`, id, string(status), paidAt, cancelledAt)
	return err
}
