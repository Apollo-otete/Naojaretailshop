// Package httpmw contains the HTTP middleware chain. Every request that reaches a handler
// carries a correlation id, is logged once, and has passed an authorizer — those are
// properties of the chain, not of individual handlers.
package httpmw

import (
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/observability"
)

// ContextRequestID is the gin context key holding the correlation id.
const ContextRequestID = "request_id"

// RequestID gives every request a correlation id and puts it on the request context, the
// response headers and every log line the request produces (§10).
//
// When trust is true an inbound X-Request-Id is honoured — but only if it is well formed. A
// caller-controlled string that flows into logs and into a response header must be
// constrained, or a header becomes a channel for log forgery and response splitting.
func RequestID(l *slog.Logger, trust bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		inbound := strings.TrimSpace(c.GetHeader("X-Request-Id"))
		id, source := inbound, "inherited"

		if id == "" || !trust || !validRequestID(id) {
			id, source = observability.NewRequestID(), "generated"
		}

		c.Set(ContextRequestID, id)
		c.Header("X-Request-Id", id)
		c.Request = c.Request.WithContext(observability.WithRequestID(c.Request.Context(), id))

		l.With(slog.String("request_id", id)).Debug("request in",
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.String("id_source", source),
		)

		c.Next()
	}
}

// validRequestID accepts only opaque single-token identifiers: no whitespace, no control
// characters, bounded length.
func validRequestID(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}
