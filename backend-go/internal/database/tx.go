package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Execer is the part of *pgxpool.Pool and pgx.Tx that a repository actually uses. Both satisfy
// it, and that is the whole trick behind joining a transaction: a use case asks for the Execer
// belonging to the context it was handed, and gets the pool when nobody opened a transaction and
// the open transaction when somebody did.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKey struct{}

// ErrNoPool is returned when a repository or runner is built without a pool — a wiring bug that
// would otherwise surface as a nil dereference inside the first request.
var ErrNoPool = errors.New("no database pool configured")

// TxFrom returns the transaction ctx carries, if any.
func TxFrom(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}

// DB is the one call every repository makes: the transaction if this context is inside one, the
// pool otherwise. ERD T-10 — "the domain row and its audit entry are one transaction, so an audit
// entry cannot be lost while the change succeeds" — holds because both adapters ask this question
// of the same context, and neither of them decides anything about transactions.
func DB(ctx context.Context, pool *pgxpool.Pool) Execer {
	if tx, ok := TxFrom(ctx); ok {
		return tx
	}
	return pool
}

// TxRunner implements a context's transaction port over one pool. There is one per service, not
// one per bounded context: two runners over one pool would give a use case two transactions to
// join, which is how an "atomic" change stops being atomic.
type TxRunner struct {
	pool *pgxpool.Pool
}

func NewTxRunner(pool *pgxpool.Pool) *TxRunner { return &TxRunner{pool: pool} }

// RunInTx runs work in one transaction. An inner RunInTx joins the open transaction instead of
// starting a nested one, so an inner use case cannot commit ahead of the outer one.
func (r *TxRunner) RunInTx(ctx context.Context, work func(ctx context.Context) error) (err error) {
	if _, nested := TxFrom(ctx); nested {
		return work(ctx)
	}
	if r.pool == nil {
		return ErrNoPool
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	// The transaction travels in the context work receives, not the one handed in: everything
	// below here must be unable to reach the pool for a write.
	cctx := context.WithValue(ctx, txKey{}, tx)

	// A panic in a use case would otherwise leave the transaction open on a pooled connection
	// that the next request inherits, and the next request would run inside someone else's
	// aborted unit of work.
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(context.WithoutCancel(ctx))
			err = fmt.Errorf("panic inside a transaction: %v", p)
		}
	}()

	if err := work(cctx); err != nil {
		// The rollback cannot be allowed to hide the reason the work failed, and the reason
		// cannot be allowed to hide a failed rollback: a transaction still open after an error
		// is the worse of the two.
		if rbErr := tx.Rollback(context.WithoutCancel(ctx)); rbErr != nil {
			return errors.Join(err, fmt.Errorf("rollback: %w", rbErr))
		}
		return err
	}
	// Commit uses a context that cannot be cancelled by the caller: the work has succeeded, and
	// a client that timed out must not be able to leave the outcome indeterminate. Either the
	// change happened or the error says so.
	if err := tx.Commit(context.WithoutCancel(ctx)); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
