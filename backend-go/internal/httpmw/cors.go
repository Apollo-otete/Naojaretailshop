package httpmw

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpjson"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
)

// allowedHeaders mirrors what contracts/openapi.yaml actually asks a client to send. A
// wildcard would work today and hide the fact that a new required header is missing from the
// contract, so the list is explicit and fails loudly when the two drift apart.
const allowedHeaders = "Accept, Content-Type, X-CSRF-Token, X-Request-Id, Idempotency-Key"
const exposedHeaders = "X-Request-Id, Retry-After"

// CORS permits credentialed cross-origin requests from an explicit origin list (ADR-019).
//
// The Express service this replaces allowed "*" whenever FRONTEND_URL was unset while still
// sending credentials: true — a combination browsers refuse, which is why the failure looked
// like a login bug rather than a configuration bug. Wildcards are rejected here at config
// validation instead, before the process is serving traffic.
func CORS(l *slog.Logger, allowed []string) gin.HandlerFunc {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		allowedSet[o] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if _, ok := allowedSet[origin]; ok {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Credentials", "true")
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				c.Header("Access-Control-Allow-Headers", allowedHeaders)
				c.Header("Access-Control-Expose-Headers", exposedHeaders)
				c.Header("Access-Control-Max-Age", "600")
				// A cached preflight is per-origin; without Vary a second origin gets the
				// first origin's answer from cache.
				c.Header("Vary", "Origin")
			} else {
				l.Warn("blocked cross-origin request",
					slog.String("origin", origin),
					slog.String("route", c.FullPath()),
				)
			}
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Authorize guards one operation with the permission keys declared in the contract. It runs
// per route rather than globally because the permission a request needs is a property of the
// operation, and the only way to keep the contract and the enforcement from diverging is to
// bind them at the same line of code where the route is registered.
func Authorize(l *slog.Logger, a port.Authorizer, permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := a.Authorize(c.Request.Context(), permissions); err != nil {
			httpjson.Fail(c, l, err)
			return
		}
		c.Next()
	}
}
