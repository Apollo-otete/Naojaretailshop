package identity

import "context"

// Principal is who a request is coming from, as the rest of the system needs to know it: the
// account, the roles it holds, and the permissions that follow from them.
//
// Permissions is the *effective* set — role grants unioned with unexpired exceptional grants —
// because that is what contracts/openapi.yaml promises in the Session schema. Computing it is
// the evaluator's job (§13 of the plan); holding it here means an authorization decision and a
// "who am I" answer cannot disagree.
type Principal struct {
	User        User
	Roles       []string
	Permissions []string

	// Conditions is the attribute half of the same read: which of Permissions carry a declared
	// condition (ADR-024), keyed by permission. It travels with the principal for the reason
	// Permissions does — the snapshot that answered "may this caller" must be the same one that
	// answered "who is this", and both come from one GrantsFor per request. Nothing here is
	// cached across requests: a stale condition would be a stale authorisation answer, which is
	// exactly what B11 and B12 forbid.
	Conditions map[string][]string

	// SessionID is which session carried this principal, not who the principal is. Logout has
	// to revoke one row rather than every row an account happens to have open elsewhere.
	SessionID int64

	// CSRFToken travels with the principal because it belongs to the session that carried it,
	// and the two things that need it are both transport answers: GET /auth/session hands it
	// back to the client, and the CSRF middleware compares it against X-CSRF-Token. It is not a
	// secret from the account's own point of view — the same browser that has the cookie can
	// read this value — which is why it is stored in the clear while the session id is hashed.
	CSRFToken string
}

func (p Principal) UserID() int64 { return p.User.ID }

func (p Principal) Has(permission string) bool {
	for _, have := range p.Permissions {
		if have == permission {
			return true
		}
	}
	return false
}

// IsAdmin is the session-lifetime switch, not an authority check. Anything that decides what a
// caller may do must ask about a permission; only the idle timeout is allowed to ask this.
func (p Principal) IsAdmin() bool { return p.User.Account == KindAdmin }

type principalKey struct{}

// WithPrincipal attaches the authenticated caller to the context. The middleware that verified
// the session is the only production caller; a handler that wants a principal has to have gone
// through that middleware, which is what the unexported key guarantees.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom returns the caller, or false if nothing authenticated this request. A handler
// that treats "false" as "anonymous" is the bug this signature exists to prevent: anonymous is
// a request with no principal, unauthenticated is a request that should never have reached here.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}
