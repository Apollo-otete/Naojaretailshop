// Package postgres implements audit.Writer against administration.audit_logs — the append-only
// record ERD §5.9 requires. The append-only part is the schema's job (a trigger refuses UPDATE
// and DELETE); this package's job is only to INSERT, and to refuse an entry that would land as an
// unreadable row.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/audit"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/database"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/observability"
)

type Writer struct {
	pool *pgxpool.Pool
}

var _ audit.Writer = (*Writer)(nil)

// New refuses a nil pool at wiring time: the alternative is a panic inside the first privileged
// write, after that write has already been committed and before its audit entry has.
func New(pool *pgxpool.Pool) (*Writer, error) {
	if pool == nil {
		return nil, database.ErrNoPool
	}
	return &Writer{pool: pool}, nil
}

// Record appends one entry to whatever transaction ctx already carries. That is the entire
// mechanism behind T-10: the use case opens one unit of work, the domain adapter and this one
// both resolve their Execer from that same context, and neither of them can commit without the
// other.
func (w *Writer) Record(ctx context.Context, e audit.Entry) error {
	if err := e.Validate(); err != nil {
		return err
	}

	previous, err := jsonColumn(e.PreviousValue)
	if err != nil {
		return fmt.Errorf("audit entry %q: %w", e.Action, err)
	}
	next, err := jsonColumn(e.NewValue)
	if err != nil {
		return fmt.Errorf("audit entry %q: %w", e.Action, err)
	}

	// A zero timestamp means the caller did not set one, which for an audit row means "now" —
	// the column default cannot be used because the INSERT names the column explicitly, and a
	// literal NULL there violates NOT NULL rather than falling back to the default.
	occurred := e.OccurredAt
	if occurred.IsZero() {
		occurred = time.Now()
	}

	_, err = database.DB(ctx, w.pool).Exec(ctx, `
		INSERT INTO administration.audit_logs
			(actor_id, actor_kind, action, resource_type, resource_id,
			 previous_value, new_value, reason, ip, correlation_id, occurred_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6::jsonb, $7::jsonb, NULLIF($8, ''),
		        NULLIF($9, '')::inet, NULLIF($10, ''), $11)`,
		e.ActorID, string(e.ActorKind), e.Action, e.ResourceType, e.ResourceID,
		previous, next, e.Reason, e.IP, observability.RequestID(ctx), occurred)
	if err != nil {
		return fmt.Errorf("recording audit entry %q: %w", e.Action, err)
	}
	return nil
}

// jsonColumn is the boundary between "the actor changed two fields" and "the actor changed a
// value that could not be serialised". An entry whose before-state silently disappeared is a
// gap in the trail, so a marshalling failure is an error rather than a NULL.
func jsonColumn(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("value of type %T cannot be stored: %w", v, err)
	}
	// A typed nil pointer marshals to the JSON literal null, which jsonb would store as a value.
	// SQL NULL is the answer to "there was nothing there", so the two are not interchangeable.
	if string(b) == "null" {
		return nil, nil
	}
	return string(b), nil
}
