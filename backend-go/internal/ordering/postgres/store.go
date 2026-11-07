package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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
var _ ordering.QuoteStore = (*Store)(nil)
var _ ordering.IdempotencyStore = (*Store)(nil)
var _ ordering.SettingsReader = (*Store)(nil)

type Tx struct{ runner *database.TxRunner }

func NewTx(pool *pgxpool.Pool) *Tx { return &Tx{runner: database.NewTxRunner(pool)} }

func (t *Tx) RunInTx(ctx context.Context, work func(ctx context.Context) error) error {
	return t.runner.RunInTx(ctx, work)
}

func (s *Store) exec(ctx context.Context) database.Execer { return database.DB(ctx, s.pool) }

const orderColumns = `id, reference, user_id, status, payment_status, subtotal_cents, delivery_fee_cents, total_cents, currency, delivery_address, contact_phone, delivery_instructions, expires_at, created_at, updated_at, paid_at, cancelled_at, fulfilment_method, quote_id, contact_name, contact_email, delivery_zone_id, delivery_area, delivery_county, delivery_landmark, pickup_window_id, notes`

func (s *Store) scanOrder(row pgx.Row) (ordering.Order, error) {
	var o ordering.Order
	var userID, quoteID, deliveryZoneID, pickupWindowID *int64
	var deliveryAddr, contactPhone, deliveryInstr, fulfilmentMethod, contactName, contactEmail, deliveryArea, deliveryCounty, deliveryLandmark, notes *string
	var expiresAt, paidAt, cancelledAt *time.Time
	err := row.Scan(
		&o.ID, &o.Reference, &userID, &o.Status, &o.PaymentStatus, &o.SubtotalCents, &o.DeliveryFeeCents,
		&o.TotalCents, &o.Currency, &deliveryAddr, &contactPhone, &deliveryInstr, &expiresAt, &o.CreatedAt,
		&o.UpdatedAt, &paidAt, &cancelledAt,
		&fulfilmentMethod, &quoteID, &contactName, &contactEmail, &deliveryZoneID, &deliveryArea, &deliveryCounty, &deliveryLandmark, &pickupWindowID, &notes,
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
	o.FulfilmentMethod = fulfilmentMethod
	o.QuoteID = quoteID
	o.ContactName = contactName
	o.ContactEmail = contactEmail
	o.DeliveryZoneID = deliveryZoneID
	o.DeliveryArea = deliveryArea
	o.DeliveryCounty = deliveryCounty
	o.DeliveryLandmark = deliveryLandmark
	o.PickupWindowID = pickupWindowID
	o.Notes = notes
	return o, nil
}

func (s *Store) CreateOrder(ctx context.Context, o *ordering.Order) error {
	row := s.exec(ctx).QueryRow(ctx, `
		INSERT INTO commerce.orders (reference, user_id, status, payment_status, subtotal_cents, delivery_fee_cents, total_cents, currency, delivery_address, contact_phone, delivery_instructions, expires_at, fulfilment_method, quote_id, contact_name, contact_email, delivery_zone_id, delivery_area, delivery_county, delivery_landmark, pickup_window_id, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
		RETURNING id, created_at, updated_at`,
		o.Reference, o.UserID, string(o.Status), string(o.PaymentStatus), o.SubtotalCents, o.DeliveryFeeCents,
		o.TotalCents, o.Currency, o.DeliveryAddress, o.ContactPhone, o.DeliveryInstructions, o.ExpiresAt,
		o.FulfilmentMethod, o.QuoteID, o.ContactName, o.ContactEmail, o.DeliveryZoneID, o.DeliveryArea, o.DeliveryCounty, o.DeliveryLandmark, o.PickupWindowID, o.Notes)
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
		    payment_status = CASE
		      WHEN $2='paid' OR $2='confirmed' THEN 'paid'
		      WHEN $2 IN ('failed','expired','cancelled') THEN 'failed'
		      WHEN $2='refunded' THEN 'refunded'
		      ELSE payment_status END,
		    paid_at=COALESCE($3, paid_at), cancelled_at=COALESCE($4, cancelled_at), updated_at=now()
		WHERE id=$1`, id, string(status), paidAt, cancelledAt)
	return err
}

// ── Quotes (P0) ─────────────────────────────────────────────────────────────

func (s *Store) CreateQuote(ctx context.Context, q *ordering.Quote) error {
	var del *ordering.DeliveryInput
	if q.Delivery != nil {
		del = q.Delivery
	}
	var zoneID *int64
	var addr, area, county, landmark *string
	if del != nil {
		zoneID = &del.ZoneID
		if del.AddressLine != "" {
			a := del.AddressLine
			addr = &a
		}
		area = del.Area
		county = del.County
		landmark = del.Landmark
	}
	row := s.exec(ctx).QueryRow(ctx, `
		INSERT INTO commerce.checkout_quotes (user_id, cart_id, session_token_hash, fulfilment_method, contact_name, contact_phone, contact_email, delivery_zone_id, delivery_address_line, delivery_area, delivery_county, delivery_landmark, pickup_window_id, notes, subtotal_cents, delivery_fee_cents, discount_cents, total_cents, currency, pricing_hash, pricing_changed, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
		RETURNING id, created_at`,
		q.UserID, nil, q.SessionTokenHash, string(q.FulfilmentMethod), q.ContactName, q.ContactPhone, q.ContactEmail,
		zoneID, addr, area, county, landmark, q.PickupWindowID, q.Notes,
		q.Totals.SubtotalCents, q.Totals.DeliveryFeeCents, q.Totals.DiscountCents, q.Totals.TotalCents, q.Totals.Currency,
		q.PricingHash, q.PricingChanged, q.ExpiresAt)
	if err := row.Scan(&q.ID, &q.CreatedAt); err != nil {
		return fmt.Errorf("creating quote: %w", err)
	}
	for i := range q.Lines {
		ln := q.Lines[i]
		_, err := s.exec(ctx).Exec(ctx, `
			INSERT INTO commerce.checkout_quote_lines (quote_id, product_id, name, sku, unit_price_cents, quantity, line_total_cents)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			q.ID, ln.ProductID, ln.Name, ln.SKU, ln.UnitPriceCents, ln.Quantity, ln.LineTotalCents)
		if err != nil {
			return fmt.Errorf("creating quote line: %w", err)
		}
	}
	return nil
}

func (s *Store) QuoteByID(ctx context.Context, id int64) (ordering.Quote, error) {
	var q ordering.Quote
	var userID, zoneID, pickupWindowID *int64
	var sessionHash, contactEmail, addr, area, county, landmark, notes *string
	var method, pricingHash, currency string
	err := s.exec(ctx).QueryRow(ctx, `
		SELECT id, user_id, session_token_hash, fulfilment_method, contact_name, contact_phone, contact_email,
		       delivery_zone_id, delivery_address_line, delivery_area, delivery_county, delivery_landmark,
		       pickup_window_id, notes, subtotal_cents, delivery_fee_cents, discount_cents, total_cents, currency,
		       pricing_hash, pricing_changed, expires_at, created_at
		FROM commerce.checkout_quotes WHERE id = $1`, id).Scan(
		&q.ID, &userID, &sessionHash, &method, &q.ContactName, &q.ContactPhone, &contactEmail,
		&zoneID, &addr, &area, &county, &landmark, &pickupWindowID, &notes,
		&q.Totals.SubtotalCents, &q.Totals.DeliveryFeeCents, &q.Totals.DiscountCents, &q.Totals.TotalCents, &currency,
		&pricingHash, &q.PricingChanged, &q.ExpiresAt, &q.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ordering.Quote{}, ordering.ErrNoQuote
	}
	if err != nil {
		return ordering.Quote{}, fmt.Errorf("reading quote: %w", err)
	}
	q.UserID = userID
	q.SessionTokenHash = sessionHash
	q.FulfilmentMethod = ordering.FulfilmentMethod(method)
	q.ContactEmail = contactEmail
	q.PickupWindowID = pickupWindowID
	q.Notes = notes
	q.Totals.Currency = currency
	q.PricingHash = pricingHash
	if zoneID != nil || addr != nil {
		d := &ordering.DeliveryInput{}
		if zoneID != nil {
			d.ZoneID = *zoneID
		}
		if addr != nil {
			d.AddressLine = *addr
		}
		d.Area, d.County, d.Landmark = area, county, landmark
		q.Delivery = d
	}
	rows, err := s.exec(ctx).Query(ctx, `
		SELECT product_id, name, sku, unit_price_cents, quantity, line_total_cents
		FROM commerce.checkout_quote_lines WHERE quote_id = $1 ORDER BY product_id ASC`, id)
	if err != nil {
		return ordering.Quote{}, err
	}
	defer rows.Close()
	q.Lines = []ordering.QuoteLine{}
	for rows.Next() {
		var ln ordering.QuoteLine
		if err := rows.Scan(&ln.ProductID, &ln.Name, &ln.SKU, &ln.UnitPriceCents, &ln.Quantity, &ln.LineTotalCents); err != nil {
			return ordering.Quote{}, err
		}
		q.Lines = append(q.Lines, ln)
	}
	if err := rows.Err(); err != nil {
		return ordering.Quote{}, err
	}
	return q, nil
}

// ── Idempotency + references (P0) ───────────────────────────────────────────

func (s *Store) GetIdempotency(ctx context.Context, userID int64, key string) (ordering.IdempotencyRecord, bool, error) {
	var rec ordering.IdempotencyRecord
	var body []byte
	var orderRef *string
	err := s.exec(ctx).QueryRow(ctx, `
		SELECT user_id, idem_key, request_hash, status_code, response_body, order_reference
		FROM commerce.order_idempotency_keys WHERE user_id = $1 AND idem_key = $2`, userID, key).
		Scan(&rec.UserID, &rec.Key, &rec.RequestHash, &rec.StatusCode, &body, &orderRef)
	if errors.Is(err, pgx.ErrNoRows) {
		return ordering.IdempotencyRecord{}, false, nil
	}
	if err != nil {
		return ordering.IdempotencyRecord{}, false, err
	}
	rec.ResponseBody = body
	rec.OrderReference = orderRef
	return rec, true, nil
}

func (s *Store) SaveIdempotency(ctx context.Context, rec ordering.IdempotencyRecord) error {
	_, err := s.exec(ctx).Exec(ctx, `
		INSERT INTO commerce.order_idempotency_keys (user_id, idem_key, request_hash, status_code, response_body, order_reference)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (user_id, idem_key) DO NOTHING`,
		rec.UserID, rec.Key, rec.RequestHash, rec.StatusCode, rec.ResponseBody, rec.OrderReference)
	return err
}

func (s *Store) NextOrderReference(ctx context.Context, year int) (string, error) {
	var n int64
	if err := s.exec(ctx).QueryRow(ctx, `SELECT nextval('commerce.order_ref_seq')`).Scan(&n); err != nil {
		return "", fmt.Errorf("next order ref: %w", err)
	}
	// Keep a stable, readable NJ-YYYY-NNNN form.
	return fmt.Sprintf("NJ-%d-%04d", year, n%10000), nil
}

// ── Settings (P0 pricing inputs) ─────────────────────────────────────────────

func (s *Store) IntSetting(ctx context.Context, key string, def int64) (int64, error) {
	var v string
	err := s.exec(ctx).QueryRow(ctx, `SELECT value FROM administration.system_settings WHERE key = $1`, key).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return def, nil
	}
	if err != nil {
		return def, err
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def, nil
	}
	return n, nil
}

func (s *Store) ListExpiredPending(ctx context.Context, now time.Time, limit int) ([]ordering.Order, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.exec(ctx).Query(ctx, `
		SELECT `+orderColumns+` FROM commerce.orders
		WHERE status IN ('pending','pending_payment') AND expires_at IS NOT NULL AND expires_at < $1
		ORDER BY expires_at ASC LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ordering.Order
	for rows.Next() {
		o, err := s.scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) ListStatusHistory(ctx context.Context, orderID int64) ([]ordering.OrderStatusHistory, error) {
	rows, err := s.exec(ctx).Query(ctx, `
		SELECT id, order_id, from_status, to_status, reason, actor_id, occurred_at
		FROM commerce.order_status_history WHERE order_id=$1 ORDER BY occurred_at ASC, id ASC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ordering.OrderStatusHistory
	for rows.Next() {
		var h ordering.OrderStatusHistory
		var from, reason *string
		var actor *int64
		var to string
		if err := rows.Scan(&h.ID, &h.OrderID, &from, &to, &reason, &actor, &h.OccurredAt); err != nil {
			return nil, err
		}
		if from != nil {
			f := *from
			h.FromStatus = &f
		}
		h.ToStatus = ordering.OrderStatus(to)
		h.Reason = reason
		h.ActorID = actor
		out = append(out, h)
	}
	return out, rows.Err()
}
