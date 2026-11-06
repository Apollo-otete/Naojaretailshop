package httpmw

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpjson"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
)

// Authorize guards one operation with the permission keys declared in the contract. It runs
// per route rather than globally because the permission a request needs is a property of the
// operation, and the only way to keep the contract and the enforcement from diverging is to
// bind them at the same line of code where the route is registered.
//
// The keys are any-of, exactly as contracts/openapi.yaml declares them (principle 3): an
// operation that serves a customer and a staff member from one route lists both
// `ordering.orders.read` and `ordering.orders.read.all`.
//
// This is the gate for an operation that touches no addressable resource. An operation whose
// permissions carry an attribute condition (ADR-024) has to be authorised where the owner is
// known — in the use case, which matrix §2 rule 2 allows as a placement, and which is the only
// place that can read the order before deciding who may see it. Declaring such a key here would
// be a request with no resource, and the evaluator answers that with a 500 rather than a guess,
// so a `[own]` route wired through this middleware fails immediately instead of quietly.
func Authorize(l *slog.Logger, a port.Authorizer, permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := a.Authorize(c.Request.Context(), port.Request{Permissions: permissions}); err != nil {
			httpjson.Fail(c, l, err)
			return
		}
		c.Next()
	}
}
