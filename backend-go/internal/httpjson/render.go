// Package httpjson is the only place that writes an HTTP body. Handlers call OK/Created/
// Fail; nothing else in the service assembles a response, so the contract's shapes cannot be
// re-invented per endpoint.
package httpjson

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/observability"
)

// Fail renders any error through the one envelope this service uses. Call it from handlers
// and from the recovery middleware, and nowhere else.
func Fail(c *gin.Context, l *slog.Logger, err error) {
	Write(c, l, apierr.AsFailure(err))
}

func Write(c *gin.Context, l *slog.Logger, f apierr.Failure) {
	if c.Writer.Written() {
		return // a body has already gone out; a second one would corrupt the first
	}

	correlationID := observability.RequestID(c.Request.Context())
	// The rejection line must be findable by the id the client was just given — that is the
	// entire exchange between a support ticket and a root cause.
	l = observability.From(l, c.Request.Context())
	route := c.FullPath()
	if route == "" {
		route = observability.UnmatchedRoute // a NoRoute or NoMethod match has no pattern
	}
	fields := []any{
		slog.String("route", route),
		slog.String("method", c.Request.Method),
		slog.String("code", string(f.Code)),
		slog.Int("status", f.Status),
	}

	// An unplanned failure is logged with its cause; the client still sees only the generic
	// message, because a driver error text can contain a query and a query can contain data.
	if f.Code == apierr.CodeInternal {
		if f.Unwrap() != nil {
			l.With(fields...).Error("request failed", slog.Any("error", f.Unwrap()))
		} else {
			l.With(fields...).Error("request failed")
		}
	} else {
		l.With(fields...).Info("request rejected")
	}

	if f.RetryIn > 0 {
		c.Header("Retry-After", strconv.Itoa(f.RetryIn))
	}

	c.AbortWithStatusJSON(f.Status, apierr.EnvelopeFor(f, correlationID))
}

func OK(c *gin.Context, payload any)       { c.JSON(http.StatusOK, payload) }
func Created(c *gin.Context, payload any)  { c.JSON(http.StatusCreated, payload) }
func Accepted(c *gin.Context, payload any) { c.JSON(http.StatusAccepted, payload) }
func NoContent(c *gin.Context)             { c.Status(http.StatusNoContent) }
