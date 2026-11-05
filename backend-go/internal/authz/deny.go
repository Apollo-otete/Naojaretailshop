// Package authz holds the port.Authorizer implementations. Phase 3 replaces DenyAll with the
// RBAC evaluator that reads identity.user_roles and identity.user_permissions; until that
// exists there is no honest way to answer "may this caller?", and the safe answer is no.
package authz

import (
	"context"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
)

// DenyAll refuses every protected operation, including ones that ask for no permissions at
// all (which still require an authenticated principal that Phase 2 does not have).
//
// This is §10's deny-by-default rule made physical. The alternative — an allow-stub so the
// skeleton "works" — is how an unauthenticated endpoint ships by accident.
type DenyAll struct{}

func (DenyAll) Authorize(_ context.Context, required []string) error {
	if len(required) == 0 {
		return apierr.Unauthorized("This action needs a signed-in caller, and authentication is not available yet.")
	}
	return apierr.Forbidden(required[0])
}

// Compile-time proof that DenyAll satisfies the port Phase 3 will re-implement.
var _ port.Authorizer = DenyAll{}
