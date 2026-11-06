package authz

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
)

// RBAC is the evaluator for the ratified model: permissions come from roles (unioned with the
// unexpired exceptional grants), and a permission that declares a condition is narrowed by it.
//
// It holds nothing. There is no cache, no memo, no per-service map of resolved grants, and that
// is ADR-019 rather than a style choice: the grants are read again for every request, so
// suspending an account or deleting a role_permissions row changes the *next* request (B11, B12).
// The moment this type stores an answer is the moment those two assertions become false.
type RBAC struct {
	Logger *slog.Logger
}

var _ port.Authorizer = RBAC{}

func (r RBAC) Authorize(ctx context.Context, req port.Request) error {
	l := r.logger()

	// First, and before anything is read: no principal, no decision. A request that never
	// authenticated has no authority to narrow, and answering it with PERMISSION_DENIED would
	// describe an authority the caller was never asked about.
	p, ok := identity.PrincipalFrom(ctx)
	if !ok {
		return apierr.Unauthorized("This action needs a signed-in caller, and this request carried no usable session.")
	}

	// x-permissions: [] is the contract's "any authenticated principal" — ending your own
	// session needs no grant. Reaching here means that is satisfied.
	if len(req.Permissions) == 0 {
		return nil
	}

	// Any-of (contract principle 3), evaluated in declared order. The first key the caller holds
	// *and whose conditions it satisfies* wins, which is what lets one operation serve both a
	// `[own]` customer and an unscoped staff member: GET /orders/{reference} lists
	// ordering.orders.read and ordering.orders.read.all, and a member of both who is not the
	// owner must still be allowed — the ownership rule belongs to the key that carries it, not
	// to the operation.
	var held []string
	for _, perm := range req.Permissions {
		if p.Has(perm) {
			held = append(held, perm)
		}
	}
	if len(held) == 0 {
		// Naming one key when several were required is not a compromise the contract leaves
		// open — PermissionError.missing_permission is a single string — so it names the first,
		// which is deterministic and the key the operation's own description leads with.
		return apierr.Forbidden(req.Permissions[0])
	}

	var unresolved error
	for _, perm := range held {
		ok, err := r.conditionsHold(l, p, perm, req.Resource)
		switch {
		case err != nil:
			// Remembered, and the search continues: another held key may carry no condition at
			// all, and a caller who is legitimately allowed must not be failed for a different
			// key's wiring.
			unresolved = errors.Join(unresolved, err)
		case ok:
			return nil
		}
	}

	// A permission that requires a resource and was not given one is a defect in this service,
	// not a decision about the caller. It surfaces as a 500 so it cannot be mistaken for a
	// refusal the client deserved, and so it cannot quietly become one for as long as the route
	// stays unfixed.
	if unresolved != nil {
		return apierr.Internal(unresolved)
	}

	// Every held key was narrowed and none of the narrowings matched. Not 403: the contract's
	// NotFound reads "No such resource, or none the caller is allowed to be told about", and
	// GET /orders/{reference} states the reason outright — "a second customer asking for someone
	// else's order reference gets 404, not 403 — 403 confirms the reference exists".
	return apierr.NotFound(req.Resource.Type)
}

// conditionsHold is the second gate for one permission the caller holds. All of a permission's
// declared conditions must be satisfied; there is no "any" variant, because a catalogue entry
// that promises two narrowing rules and is satisfied by one of them is not the entry the
// matrix wrote.
func (r RBAC) conditionsHold(l *slog.Logger, p identity.Principal, perm string, res port.Resource) (bool, error) {
	names := p.Conditions[perm]
	if len(names) == 0 {
		return true, nil
	}
	if res.Type == "" {
		return false, fmt.Errorf("permission %q declares %v but the request named no resource", perm, names)
	}

	for _, name := range names {
		f, implemented := conditions[name]
		if !implemented {
			// Reachable only by a migration that got ahead of this map — 0004's CHECK refuses
			// unknown names — and it refuses here too. The alternative, treating an unimplemented
			// rule as "not applicable", is the one place a data change could widen access.
			l.Error("condition has no implementation, refusing",
				slog.String("permission", perm), slog.String("condition", name))
			return false, nil
		}
		if !f(p, res) {
			return false, nil
		}
	}
	return true, nil
}

func (r RBAC) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}
