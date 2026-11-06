package httpmw

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpjson"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
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

// OwnAccount is the resource type for a `[own]` rule whose resource is the caller's own record,
// so an operation like POST /auth/password/change can be authorised through the same evaluator as
// one whose ownership lives in a row. It is a port.Resource.Type, and the contract's matrix writes
// the same idea as `[own]`.
const OwnAccount = "account"

// AuthorizeOwnAccount guards an operation whose resource *is* the principal: there is no id in the
// request to check against, and the only owner is whoever is signed in.
//
// Authorize cannot serve these. identity.account.update carries owner_is_caller (ADR-024), so
// handing it a request with no resource would return a 500 by design — and the resource here is
// knowable at the middleware, because it is the caller. OwnerID is set to the caller's own id
// rather than left zero: a zero owner means "nothing owns this", and the condition refuses that, so
// copying the id from the principal is what states the rule honestly — the account being changed is
// the one carried by the session, which is the whole reason the contract says there is no path from
// this operation to another user's password.
func AuthorizeOwnAccount(l *slog.Logger, a port.Authorizer, permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := identity.PrincipalFrom(c.Request.Context())
		if !ok {
			httpjson.Write(c, l, apierr.Unauthorized("Sign in to change a password."))
			return
		}
		me := caller.UserID()
		req := port.Request{
			Permissions: permissions,
			Resource:    port.Resource{Type: OwnAccount, ID: strconv.FormatInt(me, 10), OwnerID: me},
		}
		if err := a.Authorize(c.Request.Context(), req); err != nil {
			httpjson.Fail(c, l, err)
			return
		}
		c.Next()
	}
}
