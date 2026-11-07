// Package inventory owns stock truth and reservations (ERD 5.5, T-01): on_hand/reserved are
// enforced in the database and reservations prevent overselling under concurrency.
package inventory

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNoInventory = errors.New("no inventory row for product")
	ErrShortfall   = errors.New("insufficient stock")
)

type Shortfall struct {
	ProductID int64
	Name      string
	Requested int
	Available int
}

type ReserveLine struct {
	OrderID   int64
	ProductID int64
	Name      string
	Quantity  int
}

type Store interface {
	// Reserve reserves qty for order lines atomically. Lines MUST be processed in ascending
	// product_id order by the implementation (deadlock prevention, T-01). On any shortfall the
	// caller must roll back the transaction; implementations should return all shortfalls found
	// for the lines they checked before failing.
	Reserve(ctx context.Context, orderID int64, expiresAt *time.Time, lines []ReserveLine) ([]Shortfall, error)
	ReleaseOrder(ctx context.Context, orderID int64, reason string) error
	ConsumeOrder(ctx context.Context, orderID int64) error
}
