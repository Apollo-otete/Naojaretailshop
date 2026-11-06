// Package port holds the interfaces the transport and application layers depend on. Following
// §8, an adapter (Postgres, Daraja, SMTP, an RBAC evaluator) implements a port; nothing in
// this package imports Gin, so a use case can be tested without an HTTP server.
package port

import "context"

// Checker is one dependency that readiness reporting can ask a question of. Health checks
// must be a list of these rather than a hardcoded call to the database, because the service
// accumulates dependencies (Daraja, Cloudinary, SMTP) and /readyz has to tell them apart —
// a shop owner reading "unhealthy" needs to know which thing is down.
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

// Resource names the thing a request is about, so that an attribute condition has something to
// check against (ADR-024). It is deliberately three flat fields rather than a map: a condition
// that reads `attrs["owner_id"]` fails on a typo by returning nothing, and nothing is what an
// authoriser must never return quietly.
type Resource struct {
	// Type is the singular noun the contract's 404 body uses — "order", "cart", "account". It
	// is also the marker that a resource has been resolved at all: an empty Type means the
	// caller named none, which is legitimate for a permission with no conditions and a wiring
	// bug for one with them.
	Type string

	// ID is the client-visible reference (a numeric id, or an order reference). Nothing decides
	// with it today; a denial that cannot say which resource it refused is undebuggable.
	ID string

	// OwnerID is whose resource this is. Zero means the resource has no owner — a global
	// setting, a published product — and a condition that asks for an owner then refuses.
	OwnerID int64
}

// Request is what one authorization decision is made from.
type Request struct {
	// Permissions is the matched operation's `x-permissions` list, and it is **any-of**:
	// contract principle 3 and GET /orders/{reference} both describe a caller holding one key
	// of the list being allowed ("a caller holding neither gets 403"), which is how one
	// operation serves a customer's own records and a staff member's all-records view without
	// a third code path. An empty list means "any authenticated principal" — the contract's
	// `x-permissions: []` on logout.
	Permissions []string

	// Resource is what the request touches. Supply it whenever the operation names one, even
	// for a permission with no conditions: the conditions live in the catalogue and can gain an
	// entry without any route changing, and a route that never says what it touched would then
	// fail closed in the least debuggable way.
	Resource Resource
}

// Authorizer answers the question every protected operation asks: may this caller, doing this
// thing, proceed? §10's deny-by-default rule means every path that does not actively say yes
// says no, including here — an unrecognised condition, a resource with no owner, a request with
// no principal.
type Authorizer interface {
	Authorize(ctx context.Context, req Request) error
}
