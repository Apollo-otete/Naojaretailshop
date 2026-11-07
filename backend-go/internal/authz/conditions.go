// Package authz holds the port.Authorizer implementations. The model is the matrix's: a role
// holds permissions (§3), and a permission may additionally declare an attribute condition that
// narrows it (ADR-024). The first gate is RBAC, the second is the registry below, and the order
// is the point — a condition can only ever refuse a request the permissions already allowed, so
// no ownership match can compensate for a missing grant.
//
// Deny-by-default is physical here rather than a habit: every path that does not actively return
// nil returns an error, including the ones that indicate a bug (an unimplemented condition name,
// a condition with no resource to check).
package authz

import (
	"sort"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
)

// ConditionOwnerIsCaller is the Go-side name of the one condition the catalogue declares today:
// migration 0004 writes it into identity.permissions.conditions for the 15 keys the matrix marks
// `[own]`.
const ConditionOwnerIsCaller = "owner_is_caller"

// conditionFunc answers the narrowed question: this caller holds this permission — is this
// resource theirs to act on? It takes the whole resource rather than just its owner so that a
// future condition (order state, quote age) has somewhere to live without another signature.
type conditionFunc func(p identity.Principal, res port.Resource) bool

// conditions is a closed registry, and its closedness is the safety property: the catalogue can
// name a rule, and if Go has no implementation for it, the request is refused. A pluggable map
// that defaulted to "unknown means not applicable" would turn a typo in a migration into an
// open door.
//
// The CHECK constraint in 0004 is the mirror of this map and TestKnownConditionsMatchTheRegistry
// is what proves they agree.
var conditions = map[string]conditionFunc{
	ConditionOwnerIsCaller: func(p identity.Principal, res port.Resource) bool {
		// OwnerID 0 is "nobody", not "the caller": a resource that has not had an owner
		// resolved must never satisfy an ownership rule, because the failure mode of that
		// shortcut is every customer reading every order.
		return res.OwnerID != 0 && res.OwnerID == p.UserID()
	},
}

// KnownConditions lists the condition names this build implements, in a stable order. It exists
// for the test that reads the database's allowed set out of 0004 and compares, and for nothing
// else — nothing in the request path consults it.
func KnownConditions() []string {
	out := make([]string, 0, len(conditions))
	for name := range conditions {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
