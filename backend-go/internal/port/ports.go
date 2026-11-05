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

// Authorizer answers the question every protected operation asks. Phase 3 supplies the real
// RBAC evaluator; until then the only implementation is a deny-all, which is the safe
// direction to fail (§10: deny by default).
type Authorizer interface {
	// Authorize returns nil when the request in ctx may proceed. requiredPermissions are the
	// x-permissions keys declared in contracts/openapi.yaml for the matched operation; an
	// empty slice means "any authenticated principal".
	Authorize(ctx context.Context, requiredPermissions []string) error
}
