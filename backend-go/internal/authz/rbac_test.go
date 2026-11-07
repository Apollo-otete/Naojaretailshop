package authz

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/testhelper"
)

// These are the assertions the matrix makes about *how* a decision is reached, at the layer that
// reaches it — not through HTTP. B11 and B12 are the reason: both are claims that the answer is
// computed from what the tables say right now, and the only way to keep that true is for this
// type to hold nothing between calls.

const orderRead = "ordering.orders.read"

func ctxWith(p *identity.Principal) context.Context {
	if p == nil {
		return context.Background()
	}
	return identity.WithPrincipal(context.Background(), *p)
}

// customer holds the [own] key and nothing else. OwnerID 7 is "the account itself" in these
// cases, which is the shape every [own] permission has: the rule is equality, not a number.
func customer() *identity.Principal {
	return &identity.Principal{
		User:        identity.User{ID: 7},
		Roles:       []string{"customer"},
		Permissions: []string{orderRead},
		Conditions:  map[string][]string{orderRead: {ConditionOwnerIsCaller}},
	}
}

// staff holds both sides of the pair the contract declares for one operation: the scoped key and
// the unscoped variant.
func staff() *identity.Principal {
	p := customer()
	p.User.ID = 11
	p.Roles = []string{"admin_1_business"}
	p.Permissions = []string{"ordering.orders.read.all", "ordering.orders.process"}
	p.Conditions = map[string][]string{orderRead: {ConditionOwnerIsCaller}}
	return p
}

func failureOf(t *testing.T, err error) apierr.Failure {
	t.Helper()
	if err == nil {
		return apierr.Failure{Status: 0}
	}
	var f apierr.Failure
	if !errors.As(err, &f) {
		t.Fatalf("an authorizer must answer with a Failure the envelope can render, got %T: %v", err, err)
	}
	return f
}

func TestAnonymousRequestIsRefusedBeforeAnythingIsNarrowed(t *testing.T) {
	r := RBAC{}

	cases := []struct {
		name string
		req  port.Request
	}{
		{"a declared permission", port.Request{Permissions: []string{orderRead}}},
		// "any authenticated principal" is still authenticated: an empty list is not a public
		// route, it is a route that needs a session and no grant.
		{"no permission declared", port.Request{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := failureOf(t, r.Authorize(context.Background(), tc.req))
			if f.Status != 401 || f.Code != apierr.CodeNotAuthenticated {
				t.Fatalf("status=%d code=%q, want 401 NOT_AUTHENTICATED", f.Status, f.Code)
			}
			// A PermissionError body would promise a missing_permission that does not exist:
			// nothing was decided about authority here.
			if f.Missing != "" {
				t.Errorf("missing_permission = %q on an unauthenticated request", f.Missing)
			}
		})
	}
}

func TestAnyOfTheDeclaredPermissionsSuffices(t *testing.T) {
	r := RBAC{}

	// The order matters and is the contract's: GET /orders/{reference} lists the customer key
	// first. A staff caller who holds only the second must get through, or the any-of reading is
	// a fiction the middleware tells itself.
	if err := r.Authorize(ctxWith(staff()), port.Request{
		Permissions: []string{orderRead, "ordering.orders.read.all"},
		Resource:    port.Resource{Type: "order", ID: "NAO-1", OwnerID: 999},
	}); err != nil {
		t.Fatalf("holder of the second declared key refused: %v", err)
	}

	// And a caller holding neither is refused by naming the first key, which is the only one the
	// contract's single-string missing_permission can honestly carry.
	f := failureOf(t, r.Authorize(ctxWith(customer()), port.Request{
		Permissions: []string{"catalogue.products.read.draft", "ordering.orders.read.all"},
	}))
	if f.Status != 403 || f.Code != apierr.CodePermissionDenied {
		t.Fatalf("status=%d code=%q", f.Status, f.Code)
	}
	if f.Missing != "catalogue.products.read.draft" {
		t.Errorf("missing_permission = %q", f.Missing)
	}
}

func TestEmptyPermissionListMeansAnyAuthenticatedCaller(t *testing.T) {
	// Contract: `x-permissions: []` on logout — "any authenticated principal; ending your own
	// session needs no grant".
	if err := (RBAC{}).Authorize(ctxWith(customer()), port.Request{}); err != nil {
		t.Fatalf("a signed-in caller was refused an operation that declares no permission: %v", err)
	}
}

func TestOwnershipNarrowsAPermissionTheCallerHolds(t *testing.T) {
	r := RBAC{}
	req := port.Request{Permissions: []string{orderRead}, Resource: port.Resource{Type: "order", ID: "NAO-1"}}

	t.Run("the owner is admitted", func(t *testing.T) {
		req.Resource.OwnerID = 7
		if err := r.Authorize(ctxWith(customer()), req); err != nil {
			t.Fatalf("the caller who owns the order was refused: %v", err)
		}
	})

	t.Run("a non-owner is refused without confirming the reference", func(t *testing.T) {
		req.Resource.OwnerID = 8 // someone else's order
		f := failureOf(t, r.Authorize(ctxWith(customer()), req))
		// contracts/openapi.yaml, GET /orders/{reference}: "a second customer asking for
		// someone else's order reference gets 404, not 403 — 403 confirms the reference exists".
		if f.Status != 404 || f.Code != apierr.CodeNotFound {
			t.Fatalf("status=%d code=%q, want 404 NOT_FOUND", f.Status, f.Code)
		}
	})

	t.Run("a resource with no resolved owner is refused", func(t *testing.T) {
		// The dangerous direction of a missing lookup: OwnerID 0 reads like "matches nobody",
		// and an implementation that compared only equality would let 0 == 0 admit a caller
		// whose id was never loaded.
		req.Resource.OwnerID = 0
		if err := r.Authorize(ctxWith(customer()), req); err == nil {
			t.Fatal("owner_is_caller accepted a resource with no owner")
		}
	})

	t.Run("a caller with no condition attached is not narrowed", func(t *testing.T) {
		p := customer()
		p.Conditions = nil
		req.Resource.OwnerID = 8
		if err := r.Authorize(ctxWith(p), req); err != nil {
			t.Fatalf("an unconditional grant was narrowed by a condition it does not declare: %v", err)
		}
	})
}

// The condition belongs to the key, not to the operation. This is the case that a single
// "is this request scoped?" flag would get wrong: the same route, the same resource, and the
// staff member who happens to also hold the [own] key must still be served by their unscoped key.
func TestConditionBelongsToTheKeyThatCarriesIt(t *testing.T) {
	both := staff()
	both.Permissions = []string{orderRead, "ordering.orders.read.all"}
	both.Conditions = map[string][]string{orderRead: {ConditionOwnerIsCaller}}

	err := (RBAC{}).Authorize(ctxWith(both), port.Request{
		Permissions: []string{orderRead, "ordering.orders.read.all"},
		Resource:    port.Resource{Type: "order", ID: "NAO-1", OwnerID: 8},
	})
	if err != nil {
		t.Fatalf("a holder of the unscoped variant was refused another customer's order: %v", err)
	}
}

func TestUnknownConditionDeniesRatherThanPasses(t *testing.T) {
	l, buf := testhelper.Logger(t, 0)

	p := customer()
	p.Conditions = map[string][]string{orderRead: {"owner_is_the_moon"}}

	err := (RBAC{Logger: l}).Authorize(ctxWith(p), port.Request{
		Permissions: []string{orderRead},
		Resource:    port.Resource{Type: "order", ID: "NAO-1", OwnerID: 7},
	})
	if err == nil {
		t.Fatal("a condition with no implementation widened access — the registry must be closed")
	}
	if buf.Find(t, "condition has no implementation, refusing") == nil {
		t.Errorf("a rule the code cannot evaluate was not logged:\n%s", buf)
	}
}

// A `[own]` route that never resolved a resource is a defect in this service. Refusing it as 403
// or 404 would dress a wiring bug up as a decision about the caller, and it would stay broken
// for as long as the response looked plausible.
func TestConditionalPermissionWithNoResourceNamedIsAServerError(t *testing.T) {
	f := failureOf(t, (RBAC{}).Authorize(ctxWith(customer()), port.Request{
		Permissions: []string{orderRead},
	}))
	if f.Status != 500 || f.Code != apierr.CodeInternal {
		t.Fatalf("status=%d code=%q, want 500 INTERNAL_ERROR", f.Status, f.Code)
	}
	if f.Unwrap() == nil {
		t.Error("the cause must survive to the log even though the client sees nothing")
	}

	// ...and a second held key that needs no resource still wins: the bug in one key must not
	// deny an operation the caller is legitimately allowed to perform.
	if err := (RBAC{}).Authorize(ctxWith(staff()), port.Request{
		Permissions: []string{orderRead, "ordering.orders.read.all"},
	}); err != nil {
		t.Fatalf("an unscoped holder was refused because another key was mis-wired: %v", err)
	}
}

// B11 and B12 are claims about *time*: a suspension or a removed permission changes the next
// request, not the next login. The way that breaks is a cache, so the assertion here is that
// there is nowhere to put one — one evaluator instance, two callers whose grants differ, and no
// state carried between the calls.
func TestOneEvaluatorAnswersForEveryCallerWithoutRememberingAny(t *testing.T) {
	r := RBAC{}
	req := port.Request{Permissions: []string{orderRead}, Resource: port.Resource{Type: "order", OwnerID: 7}}

	if err := r.Authorize(ctxWith(customer()), req); err != nil {
		t.Fatalf("owner refused: %v", err)
	}

	// The same account, one permission deleted from its role (B12's DELETE FROM role_permissions),
	// presented as the fresh read the next request would make.
	revoked := customer()
	revoked.Permissions = nil
	f := failureOf(t, r.Authorize(ctxWith(revoked), req))
	if f.Status != 403 {
		t.Fatalf("status = %d after the grant was removed, want 403 on the very next call", f.Status)
	}

	// And a second caller must not inherit the first's answer.
	other := customer()
	other.User.ID = 21
	if err := r.Authorize(ctxWith(other), req); err == nil {
		t.Fatal("user 21 was served user 7's decision")
	}

	// The structural half: RBAC's fields are dependencies, never data. A map, a slice or a mutex
	// added here is a cache, and a cache is what makes the two assertions above false in
	// production while they stay green here.
	typ := reflect.TypeOf(RBAC{})
	for i := range typ.NumField() {
		f := typ.Field(i)
		if f.Type != reflect.TypeOf(r.Logger) {
			t.Errorf("RBAC.%s is %s — only the logger may be held between requests", f.Name, f.Type)
		}
	}
}

func TestKnownConditionsIsTheRegistry(t *testing.T) {
	got := KnownConditions()
	want := []string{ConditionOwnerIsCaller}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("KnownConditions() = %v, want %v — and note that migration 0004's CHECK constraint"+
			" lists the same names separately, so both must be edited together", got, want)
	}
}
