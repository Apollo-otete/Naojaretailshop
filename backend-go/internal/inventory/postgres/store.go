package postgres

import (
	"context"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/database"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/inventory"
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

var _ inventory.Store = (*Store)(nil)

func (s *Store) exec(ctx context.Context) database.Execer { return database.DB(ctx, s.pool) }

// Reserve implements T-01: atomic conditional reservation per line in ascending product_id order.
// It must run inside a transaction that the caller owns; on shortfall it returns ErrShortfall and
// the caller rolls back, so partial reservations never commit.
func (s *Store) Reserve(ctx context.Context, orderID int64, expiresAt *time.Time, lines []inventory.ReserveLine) ([]inventory.Shortfall, error) {
	if len(lines) == 0 {
		return nil, nil
	}
	// Deterministic order for deadlock prevention.
	ordered := append([]inventory.ReserveLine(nil), lines...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ProductID < ordered[j].ProductID })

	var shortfalls []inventory.Shortfall
	for _, ln := range ordered {
		// Ensure an inventory row exists (safe default: 0 on hand, tracked). Bootstrap migration
		// seeds existing products; new products get a row here with no available stock until set.
		_, err := s.exec(ctx).Exec(ctx, `
			INSERT INTO inventory.inventory (product_id, on_hand, reserved, track_stock)
			VALUES ($1, 0, 0, TRUE)
			ON CONFLICT (product_id) DO NOTHING`, ln.ProductID)
		if err != nil {
			return nil, err
		}

		// Atomic conditional reserve: only succeeds when enough is available (or not tracked).
		var onHand, reserved int
		var track bool
		err = s.exec(ctx).QueryRow(ctx, `
			UPDATE inventory.inventory
			SET reserved = reserved + $2, updated_at = now()
			WHERE product_id = $1
			  AND (NOT track_stock OR (on_hand - reserved) >= $2)
			RETURNING on_hand, reserved, track_stock`,
			ln.ProductID, ln.Quantity).Scan(&onHand, &reserved, &track)
		if err != nil {
			// No row updated => insufficient (or row missing, handled above). Read current availability.
			_ = s.exec(ctx).QueryRow(ctx, `SELECT on_hand, reserved, track_stock FROM inventory.inventory WHERE product_id = $1`, ln.ProductID).
				Scan(&onHand, &reserved, &track)
			avail := 0
			if track {
				avail = onHand - reserved
				if avail < 0 {
					avail = 0
				}
			} else {
				avail = ln.Quantity // not tracked: report as sufficient in effect
			}
			if !track {
				// Not tracked: force reserve without the availability constraint? Simpler: update reserved anyway.
				_ = s.exec(ctx).QueryRow(ctx, `
					UPDATE inventory.inventory SET reserved = reserved + $2, updated_at = now()
					WHERE product_id = $1 RETURNING on_hand, reserved, track_stock`,
					ln.ProductID, ln.Quantity).Scan(&onHand, &reserved, &track)
				avail = ln.Quantity
			} else {
				shortfalls = append(shortfalls, inventory.Shortfall{
					ProductID: ln.ProductID, Name: ln.Name, Requested: ln.Quantity, Available: avail,
				})
				continue
			}
		}

		// Idempotent reservation row per (order, product).
		var resID int64
		err = s.exec(ctx).QueryRow(ctx, `
			INSERT INTO inventory.stock_reservations (order_id, product_id, quantity, status, expires_at)
			VALUES ($1, $2, $3, 'active', $4)
			ON CONFLICT (order_id, product_id) DO UPDATE
			  SET quantity = EXCLUDED.quantity, status = 'active', expires_at = EXCLUDED.expires_at, updated_at = now()
			RETURNING id`, orderID, ln.ProductID, ln.Quantity, expiresAt).Scan(&resID)
		if err != nil {
			return nil, err
		}
		_, err = s.exec(ctx).Exec(ctx, `
			INSERT INTO inventory.stock_movements (product_id, order_id, reservation_id, delta_on_hand, delta_reserved, on_hand_after, reserved_after, kind, reason)
			VALUES ($1, $2, $3, 0, $4, $5, $6, 'reservation', $7)`,
			ln.ProductID, orderID, resID, ln.Quantity, onHand, reserved, "order placement")
		if err != nil {
			return nil, err
		}
	}

	if len(shortfalls) > 0 {
		return shortfalls, inventory.ErrShortfall
	}
	return nil, nil
}

func (s *Store) ReleaseOrder(ctx context.Context, orderID int64, reason string) error {
	// Release active reservations idempotently: decrement reserved once per active reservation.
	rows, err := s.exec(ctx).Query(ctx, `
		SELECT id, product_id, quantity FROM inventory.stock_reservations
		WHERE order_id = $1 AND status = 'active' ORDER BY product_id ASC FOR UPDATE`, orderID)
	if err != nil {
		return err
	}
	defer rows.Close()
	type rel struct {
		id, productID int64
		qty           int
	}
	var rels []rel
	for rows.Next() {
		var r rel
		if err := rows.Scan(&r.id, &r.productID, &r.qty); err != nil {
			return err
		}
		rels = append(rels, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range rels {
		var onHand, reserved int
		err := s.exec(ctx).QueryRow(ctx, `
			UPDATE inventory.inventory SET reserved = GREATEST(reserved - $2, 0), updated_at = now()
			WHERE product_id = $1 RETURNING on_hand, reserved`, r.productID, r.qty).Scan(&onHand, &reserved)
		if err != nil {
			return err
		}
		if _, err := s.exec(ctx).Exec(ctx, `UPDATE inventory.stock_reservations SET status='released', updated_at=now() WHERE id=$1`, r.id); err != nil {
			return err
		}
		if _, err := s.exec(ctx).Exec(ctx, `
			INSERT INTO inventory.stock_movements (product_id, order_id, reservation_id, delta_on_hand, delta_reserved, on_hand_after, reserved_after, kind, reason)
			VALUES ($1,$2,$3,0,$4,$5,$6,'reservation_release',$7)`,
			r.productID, orderID, r.id, -r.qty, onHand, reserved, reason); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ConsumeOrder(ctx context.Context, orderID int64) error {
	// On confirmed/paid consumption: convert active reservation to consumed, decrement on_hand+reserved.
	rows, err := s.exec(ctx).Query(ctx, `
		SELECT id, product_id, quantity FROM inventory.stock_reservations
		WHERE order_id = $1 AND status = 'active' ORDER BY product_id ASC FOR UPDATE`, orderID)
	if err != nil {
		return err
	}
	defer rows.Close()
	type rel struct {
		id, productID int64
		qty           int
	}
	var rels []rel
	for rows.Next() {
		var r rel
		if err := rows.Scan(&r.id, &r.productID, &r.qty); err != nil {
			return err
		}
		rels = append(rels, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range rels {
		var onHand, reserved int
		err := s.exec(ctx).QueryRow(ctx, `
			UPDATE inventory.inventory
			SET on_hand = GREATEST(on_hand - $2, 0), reserved = GREATEST(reserved - $2, 0), updated_at = now()
			WHERE product_id = $1 RETURNING on_hand, reserved`, r.productID, r.qty).Scan(&onHand, &reserved)
		if err != nil {
			return err
		}
		if _, err := s.exec(ctx).Exec(ctx, `UPDATE inventory.stock_reservations SET status='consumed', updated_at=now() WHERE id=$1`, r.id); err != nil {
			return err
		}
		if _, err := s.exec(ctx).Exec(ctx, `
			INSERT INTO inventory.stock_movements (product_id, order_id, reservation_id, delta_on_hand, delta_reserved, on_hand_after, reserved_after, kind, reason)
			VALUES ($1,$2,$3,$4,$4,$5,$6,'consumption',$7)`,
			r.productID, orderID, r.id, -r.qty, onHand, reserved, "payment confirmed"); err != nil {
			return err
		}
	}
	return nil
}
