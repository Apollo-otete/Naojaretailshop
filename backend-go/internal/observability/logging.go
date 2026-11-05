// Package observability provides the structured logging the reform requires: JSON records,
// a level from configuration, and a correlation id that rides along with every log line
// produced while handling one request (§10: "structured logs with correlation/request IDs").
package observability

import (
	"context"
	"log/slog"
	"os"

	"github.com/google/uuid"
)

const requestIDKey contextKey = "request_id"

// UnmatchedRoute is what the `route` log field says for a request no pattern claimed. Both the
// access log and the rejection log use it, so one grep finds both lines for the same failure.
const UnmatchedRoute = "<unmatched>"

type contextKey string

// NewLogger builds the process logger. Everything goes to stdout: in the target deployment
// the container runtime owns log rotation, and a second file-based sink would just create a
// second, unsynchronised copy of the truth.
func NewLogger(level slog.Level) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(h).With(slog.String("service", "naoja-api"))
}

// NewRequestID produces an opaque identifier. A UUID is used rather than a counter because
// counters collide across processes the moment the API runs more than one replica.
func NewRequestID() string { return uuid.NewString() }

// WithRequestID stores the id on the context so use cases can log against it without taking
// an HTTP type — the domain must not import the transport layer.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestID returns the id for ctx, or "" when there is none (background jobs, migrations).
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// From returns a logger carrying the request id, so a single handler body can log through
// the whole call chain without threading the id by hand.
func From(l *slog.Logger, ctx context.Context) *slog.Logger {
	if id := RequestID(ctx); id != "" {
		return l.With(slog.String("request_id", id))
	}
	return l
}
