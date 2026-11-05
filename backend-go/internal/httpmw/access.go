package httpmw

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpjson"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/observability"
)

// statusRecorder captures the final status code so the access log can report it.
type statusRecorder struct {
	gin.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Access writes exactly one log line per request. It records the matched route pattern rather
// than the raw path: /orders/NJ-2026-417 turns customer identifiers into log content, and the
// query string is where a phone number would land if a client ever put one there.
func Access(l *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Writer = &statusRecorder{ResponseWriter: c.Writer, status: http.StatusOK}
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = observability.UnmatchedRoute // a 404 from Gin has no pattern; recording it separately is the point
		}

		status := c.Writer.Status()
		attrs := []any{
			slog.String("method", c.Request.Method),
			slog.String("route", route),
			slog.Int("status", status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.String("request_id", observability.RequestID(c.Request.Context())),
		}

		// Health polling is the noisiest thing in the system; it stays visible at debug level
		// instead of being silenced entirely, so a broken probe can still be traced.
		if isProbe(route) {
			l.Debug("probe", attrs...)
			return
		}
		switch {
		case status >= 500:
			l.Error("request", attrs...)
		case status >= 400:
			l.Warn("request", attrs...)
		default:
			l.Info("request", attrs...)
		}
	}
}

func isProbe(route string) bool {
	return route == "/healthz" || route == "/readyz"
}

// Recover converts a panic into the contract's 500 envelope. Without it Gin prints a stack to
// stderr and aborts the response mid-write, which leaves the client with a truncated body and
// no correlation id to report.
func Recover(l *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				l.With(slog.String("request_id", observability.RequestID(c.Request.Context()))).
					Error("panic recovered",
						slog.Any("panic", rec),
						slog.String("method", c.Request.Method),
						slog.String("path", c.Request.URL.Path),
						slog.String("stack", string(debug.Stack())),
					)
				httpjson.Write(c, l, apierr.Internal(nil))
			}
		}()
		c.Next()
	}
}
