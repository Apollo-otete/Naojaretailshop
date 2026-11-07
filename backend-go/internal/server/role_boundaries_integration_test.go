//go:build integration

package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	auditpg "github.com/Apollo-otete/Naojaretailshop/backend-go/internal/audit/postgres"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/authz"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/dbtest"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpjson"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	identitypg "github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity/postgres"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
)

// docs/phase-1-actor-permission-matrix.md §5, tested the way B13 demands: at the API, with the
// grants migration 0003 actually seeds, through the middleware a real route uses.
//
// Three things this rules out that a unit test of the evaluator cannot. The role-to-permission rows
// could have been seeded wrongly; the per-request grant read could have been wired to something that
// always grants; and a denial could have been a 401 in disguise, which is what a broken cookie looks
// like and which would let every assertion below "pass" while proving nothing. So each story asserts
// both directions — the roles that may, and the roles that may not — against one real login per
// actor.
//
// Covered: B1–B9 (the role rows, each in both directions), B10 (ownership narrowing, against a probe
// that declares the same x-permissions pair the contracted GET /orders/{reference} does), B11 and
// B12 (the lifecycle halves — account state and role definitions are re-read per request), B13 (by
// construction: everything here is an HTTP call, no UI and no evaluator shortcut) and B15 (refund
// request versus refund execution).
//
// Not covered, and not coverable yet: B14 needs the invitation slice (an invited identifier to
// match), B16 needs the order state machine to have transitions to refuse and somewhere to record a
// reason, and B17 needs order totals to be stored rather than recomputed. Those are assertions about
// those slices' own rows, so a probe cannot stand in for them the way it can for a permission key.

const boundaryPassword = "a password long enough to pass"

type actor struct {
	name  string
	email string
	role  string
	admin bool
	phone string

	userID int64
	cookie string
}

// boundaries is one engine with one signed-in actor per role plus two customers, which is the
// smallest population that can express every B story: "cannot" needs someone who can.
type boundaries struct {
	t      *testing.T
	engine *gin.Engine
	pool   *pgxpool.Pool
	byName map[string]*actor
	rbac   authz.RBAC
	logger *slog.Logger
}

func setUp(t *testing.T) *boundaries {
	t.Helper()

	pool := dbtest.Pool(t)
	dbtest.Truncate(t, pool, "identity.users")

	ctx := context.Background()
	l := dbtest.Logger()

	store, err := identitypg.New(pool)
	if err != nil {
		t.Fatalf("identity store: %v", err)
	}
	audit, err := auditpg.New(pool)
	if err != nil {
		t.Fatalf("audit writer: %v", err)
	}
	hasher := identity.NewHasher()
	hash, err := hasher.Hash(boundaryPassword)
	if err != nil {
		t.Fatalf("hashing the fixture password: %v", err)
	}

	// One actor per role, and two customers, because B10 is about one of them reaching the other.
	actors := []*actor{
		{name: "super", email: "super.boundary@naoja.test", role: "super_admin", admin: true},
		{name: "business", email: "business.boundary@naoja.test", role: "admin_1_business", admin: true},
		{name: "operations", email: "operations.boundary@naoja.test", role: "admin_2_operations", admin: true},
		{name: "shopperA", email: "shopper.a.boundary@naoja.test", role: "customer", phone: "0712345601"},
		{name: "shopperB", email: "shopper.b.boundary@naoja.test", role: "customer", phone: "0712345602"},
	}

	byName := map[string]*actor{}
	for _, a := range actors {
		var u identity.User
		if a.admin {
			u, err = identity.NewAdmin(a.name+" administrator", a.email, "", hash)
		} else {
			u, err = identity.NewCustomer(a.name+" shopper", a.email, a.phone, hash)
		}
		if err != nil {
			t.Fatalf("building %s: %v", a.name, err)
		}
		if err := store.CreateUser(ctx, &u); err != nil {
			t.Fatalf("creating %s: %v", a.name, err)
		}
		// grantedBy nil: these are the seeded system roles, not exceptional grants a human made,
		// and the column that records who granted something must stay empty when nobody did.
		if err := store.AssignRole(ctx, u.ID, a.role, nil); err != nil {
			t.Fatalf("assigning %s to %s: %v", a.role, a.name, err)
		}
		a.userID = u.ID
		byName[a.name] = a
	}

	svc, err := identity.NewService(identity.Deps{
		Users:    store,
		Sessions: store,
		Grants:   store,
		Tx:       identitypg.NewTx(pool),
		Audit:    audit,
		Hasher:   hasher,
		Policy: identity.Policy{
			Idle: 30 * time.Minute, AdminIdle: 15 * time.Minute, Absolute: 12 * time.Hour,
		},
		Login:  identity.LoginPolicy{MaxFailedAttempts: 5, Lockout: 15 * time.Minute},
		Logger: l,
	})
	if err != nil {
		t.Fatalf("identity service: %v", err)
	}

	// A real login, not a hand-built principal: an actor's permissions have to arrive by way of the
	// same per-request grant read a live request uses.
	for _, a := range actors {
		out, err := svc.Login(ctx, identity.LoginInput{
			Email: a.email, Password: boundaryPassword, IP: "10.0.0.1", UserAgent: "boundary-suite",
		})
		if err != nil {
			t.Fatalf("logging in %s: %v", a.name, err)
		}
		a.cookie = "naoja_session=" + out.Session.Token
	}

	rbac := authz.RBAC{Logger: l}
	d := Deps{
		Logger:     l,
		Cfg:        testCfg("development"),
		Authorizer: rbac,
		Sessions:   svc,
		Identity:   svc,
		Version:    "boundaries",
	}
	r := engine(t, d)

	b := &boundaries{t: t, engine: r, pool: pool, byName: byName, rbac: rbac, logger: l}
	b.mountProbes(d, []string{
		"identity.admins.manage",
		"identity.roles.manage",
		"identity.super_admins.manage",
		"payments.status.override",
		"payments.refund",
		"payments.refund.request",
		"analytics.dashboards.read.business",
		"analytics.targets.manage",
		"administration.system_settings.manage",
		"administration.integrations.manage",
		"administration.audit_logs.read",
	})
	b.mountScopedProbe()
	return b
}

// mountProbes registers one protected GET per permission key, guarded by the same Require a real
// route uses. The path is derived from the key, so the table below stays a statement about
// permissions rather than about endpoints this phase has not built.
func (b *boundaries) mountProbes(d Deps, keys []string) {
	for _, key := range keys {
		path := probePath(key)
		b.engine.GET(path, Require(d, key), func(c *gin.Context) {
			httpjson.OK(c, gin.H{"permitted": key})
		})
	}
}

func probePath(key string) string {
	return "/api/v1/probe/" + strings.ReplaceAll(key, ".", "-")
}

// mountScopedProbe stands in for the ordering slice's GET /orders/{reference}. It is the only way to
// test B10 before orders exist, and it is written the way matrix §2 rule 2 requires: read the
// resource, then authorise the *pair* of caller and owner, then serve.
func (b *boundaries) mountScopedProbe() {
	ownerOf := map[string]int64{
		"NJ-2026-0001": b.byName["shopperA"].userID,
		"NJ-2026-0002": b.byName["shopperB"].userID,
	}

	b.engine.GET("/api/v1/probe-scoped/orders/:reference", func(c *gin.Context) {
		reference := c.Param("reference")
		owner, ok := ownerOf[reference]
		if !ok {
			httpjson.Write(c, b.logger, apierr.NotFound("order"))
			return
		}
		// Both keys, exactly as the contracted operation declares them (openapi.yaml's x-permissions
		// for GET /orders/{reference}) — any-of, so the customer path and the staff path are decided
		// by one call and neither is special-cased here.
		err := b.rbac.Authorize(c.Request.Context(), port.Request{
			Permissions: []string{"ordering.orders.read", "ordering.orders.read.all"},
			Resource:    port.Resource{Type: "order", ID: reference, OwnerID: owner},
		})
		if err != nil {
			httpjson.Fail(c, b.logger, err)
			return
		}
		httpjson.OK(c, gin.H{"reference": reference})
	})
}

// call signs in as one actor, or as nobody when actor is "", and returns what came back.
func (b *boundaries) call(actor, path string) (int, apierr.Envelope) {
	h := http.Header{}
	if a, ok := b.byName[actor]; ok {
		h.Add("Cookie", a.cookie)
	}

	resp, body := get(b.t, b.engine, path, h)
	e := apierr.Envelope{}
	if err := json.Unmarshal([]byte(body), &e); err != nil && resp.StatusCode >= 400 {
		b.t.Fatalf("%s answered %d with a body that is not the contract envelope: %s", path, resp.StatusCode, body)
	}
	return resp.StatusCode, e
}

// TestRoleBoundariesAreWhatTheMatrixSaysTheyAre is B1–B9 and B15: each row is a sentence from the
// matrix, checked in both directions.
func TestRoleBoundariesAreWhatTheMatrixSaysTheyAre(t *testing.T) {
	b := setUp(t)

	stories := []struct {
		story string
		// claim is the matrix's sentence, kept here so a failure reads as the story it broke.
		claim      string
		permission string
		allowed    []string
		denied     []string
	}{
		{"B1", "Admin 2 cannot create, invite, suspend or delete any admin account",
			"identity.admins.manage", []string{"super", "business"}, []string{"operations", "shopperA"}},
		{"B2", "Admin 2 cannot change a role definition or hold identity.roles.manage",
			"identity.roles.manage", []string{"super"}, []string{"business", "operations", "shopperA"}},
		{"B3", "Admin 2 cannot set payment_status directly",
			"payments.status.override", []string{"super", "business"}, []string{"operations", "shopperA"}},
		{"B4", "Admin 2 cannot read revenue aggregates, forecasts, targets or business reports",
			"analytics.dashboards.read.business", []string{"super", "business"}, []string{"operations", "shopperA"}},
		{"B4", "Admin 2 cannot manage targets",
			"analytics.targets.manage", []string{"super", "business"}, []string{"operations", "shopperA"}},
		{"B5", "Admin 2 cannot change global settings",
			"administration.system_settings.manage", []string{"super"}, []string{"operations", "shopperA"}},
		{"B5", "Admin 2 cannot change integration credentials",
			"administration.integrations.manage", []string{"super"}, []string{"operations", "shopperA"}},
		{"B6", "Admin 2 cannot read the audit log",
			"administration.audit_logs.read", []string{"super", "business"}, []string{"operations", "shopperA"}},
		{"B7", "Admin 1 cannot create, suspend or edit a Super Admin",
			"identity.super_admins.manage", []string{"super"}, []string{"business", "operations", "shopperA"}},
		{"B8", "Admin 1 cannot change global role definitions",
			"identity.roles.manage", []string{"super"}, []string{"business", "operations"}},
		{"B8", "Admin 1 cannot change security configuration",
			"administration.integrations.manage", []string{"super"}, []string{"business", "operations"}},
		{"B9", "Admin 1 cannot modify system_settings",
			"administration.system_settings.manage", []string{"super"}, []string{"business", "operations"}},
		{"B15", "Admin 2 may request a refund",
			"payments.refund.request", []string{"super", "business", "operations"}, []string{"shopperA"}},
		{"B15", "Admin 2 may never approve or record a refund's execution",
			"payments.refund", []string{"super", "business"}, []string{"operations", "shopperA"}},
	}

	for _, s := range stories {
		path := probePath(s.permission)

		for _, who := range s.allowed {
			status, e := b.call(who, path)
			if status != http.StatusOK {
				t.Errorf("%s: %s was refused %d (%s) on %q, which the matrix says it holds — %s",
					s.story, who, status, e.Code, s.permission, s.claim)
			}
		}

		for _, who := range s.denied {
			status, e := b.call(who, path)
			if status != http.StatusForbidden {
				t.Errorf("%s: %s reached %q with %d, want 403 — %s",
					s.story, who, s.permission, status, s.claim)
				continue
			}
			// The contract's PermissionDenied exists so a role gap is diagnosable without server
			// logs; a 403 that does not name the key is only half an answer.
			if e.Code != apierr.CodePermissionDenied {
				t.Errorf("%s: %s got code %q, want PERMISSION_DENIED", s.story, who, e.Code)
			}
			if e.MissingPermission != s.permission {
				t.Errorf("%s: missing_permission %q, want %q", s.story, e.MissingPermission, s.permission)
			}
		}
	}
}

// A customer is not an administrator who happens to lack a key: the administrative surface is
// refused to them by the same code path, so this checks the boundary from the outside in.
func TestACustomerHoldsNoneOfTheAdministrativeSurface(t *testing.T) {
	b := setUp(t)

	for _, key := range []string{
		"identity.admins.manage", "identity.roles.manage", "identity.super_admins.manage",
		"payments.status.override", "payments.refund",
		"administration.system_settings.manage", "administration.integrations.manage",
		"administration.audit_logs.read", "analytics.dashboards.read.business",
	} {
		status, e := b.call("shopperA", probePath(key))
		if status != http.StatusForbidden || e.MissingPermission != key {
			t.Errorf("customer on %q: %d/%s, want 403 naming the key", key, status, e.Code)
		}
	}
}

// B10, in the matrix's own words: "even with a valid token and a guessed order reference". The token
// is real and the reference is exact; what is missing is ownership.
func TestOneCustomerCannotReachAnotherCustomersOrder(t *testing.T) {
	b := setUp(t)
	const own, foreign = "/api/v1/probe-scoped/orders/NJ-2026-0001", "/api/v1/probe-scoped/orders/NJ-2026-0002"

	// The control first: A reaches A's own order.
	if status, e := b.call("shopperA", own); status != http.StatusOK {
		t.Fatalf("own order: %d (%s), want 200 — the probe or the grants are wrong", status, e.Code)
	}

	// A reaches for B's order and gets 404, not 403. The contract is explicit that 403 confirms the
	// reference exists, which would turn the endpoint into an order-number enumerator.
	status, e := b.call("shopperA", foreign)
	if status != http.StatusNotFound {
		t.Fatalf("cross-owner read: %d (%s), want 404 — B10 is broken if this is a 403 or a 200",
			status, e.Code)
	}
	if e.Code != apierr.CodeNotFound {
		t.Errorf("cross-owner read code %q, want NOT_FOUND", e.Code)
	}

	// B sees the same reference and gets it. The difference is ownership alone, which is what makes
	// this a statement about the row rather than about the string in the path.
	if status, e := b.call("shopperB", foreign); status != http.StatusOK {
		t.Errorf("owner reading their own order: %d (%s), want 200", status, e.Code)
	}

	// Staff hold ordering.orders.read.all, which carries no condition (ADR-024), so the same
	// reference resolves for them. Without this the suite could not tell narrowing from blocking.
	for _, who := range []string{"super", "business", "operations"} {
		if status, e := b.call(who, foreign); status != http.StatusOK {
			t.Errorf("%s on another customer's order: %d (%s), want 200 via ordering.orders.read.all",
				who, status, e.Code)
		}
	}
}

// B13's own claim, checked rather than asserted. The boundary tests above would still "pass" if a
// cookie silently stopped working, because every actor would then be refused — so the refusal has to
// be distinguishable from the anonymous one, and an anonymous request must never be answered with a
// permission decision it did not earn.
func TestAnAnonymousRequestIsRefusedBeforeAnyBoundaryIsDecided(t *testing.T) {
	b := setUp(t)

	status, e := b.call("", probePath("identity.roles.manage"))
	if status != http.StatusUnauthorized {
		t.Fatalf("anonymous probe: %d, want 401 — the chain is not what the boundary tests assume", status)
	}
	if e.Code != apierr.CodeNotAuthenticated {
		t.Errorf("anonymous probe code %q, want NOT_AUTHENTICATED", e.Code)
	}
	if e.MissingPermission != "" {
		t.Errorf("anonymous probe named missing_permission %q, which invents an authority decision",
			e.MissingPermission)
	}
}

// B11 against the real database rather than the fakes: a suspension written between two requests
// cuts the second one off, with a cookie that was accepted a moment earlier.
func TestSuspensionTakesEffectOnTheNextRequestAgainstPostgres(t *testing.T) {
	b := setUp(t)
	path := probePath("payments.refund.request")

	if status, e := b.call("business", path); status != http.StatusOK {
		t.Fatalf("baseline for admin_1_business: %d (%s), want 200", status, e.Code)
	}

	// Suspend through the same table the per-request read consults, not through a service method
	// that could be the thing under test.
	_, err := b.pool.Exec(context.Background(),
		`UPDATE identity.users SET is_suspended = true WHERE email = $1`, "business.boundary@naoja.test")
	if err != nil {
		t.Fatalf("suspending: %v", err)
	}

	status, e := b.call("business", path)
	if status != http.StatusUnauthorized {
		t.Fatalf("suspended admin: %d (%s), want 401 — B11 requires the next request to fail", status, e.Code)
	}
	if e.Code != apierr.CodeNotAuthenticated {
		t.Errorf("suspended admin code %q, want NOT_AUTHENTICATED", e.Code)
	}

	// And the same session works again the moment the suspension is lifted, with no new login: the
	// account state is read per request, so nothing had to be invalidated or expired.
	if _, err := b.pool.Exec(context.Background(),
		`UPDATE identity.users SET is_suspended = false WHERE email = $1`, "business.boundary@naoja.test"); err != nil {
		t.Fatalf("lifting the suspension: %v", err)
	}
	if status, e := b.call("business", path); status != http.StatusOK {
		t.Errorf("reactivated admin: %d (%s), want 200 without signing in again", status, e.Code)
	}
}

// B12's sentence is about the role, not the person: take a key out of the definition and every
// member loses it on the next request. A re-login would be the bug — it is what a cached grant read
// looks like from the outside, and it is exactly what ADR-019 rules out. So the cookie that was
// accepted a moment earlier is presented again, unchanged, and the answer must already be different.
func TestRemovingAPermissionFromARoleReachesEveryMemberWithoutReLogin(t *testing.T) {
	b := setUp(t)
	path := probePath("administration.audit_logs.read")
	ctx := context.Background()

	// The control: both members of the definition can reach it before anything is removed.
	for _, who := range []string{"super", "business"} {
		if status, e := b.call(who, path); status != http.StatusOK {
			t.Fatalf("%s before the removal: %d (%s), want 200", who, status, e.Code)
		}
	}

	var permissionID int64
	err := b.pool.QueryRow(ctx, `
		DELETE FROM identity.role_permissions rp
		USING identity.roles r, identity.permissions p
		WHERE rp.role_id = r.id AND rp.permission_id = p.id
		  AND r.key = 'admin_1_business' AND p.key = 'administration.audit_logs.read'
		RETURNING rp.permission_id`).Scan(&permissionID)
	if err != nil {
		t.Fatalf("removing the permission from the role: %v", err)
	}
	// The seeded catalogue belongs to the migration, not to this test, so it goes back whether the
	// assertions below pass or fail — a leftover deleted row would make the boundary suite lie next
	// run. Idempotent, because restore is also called inline.
	restore := func() {
		_, err := b.pool.Exec(ctx, `
			INSERT INTO identity.role_permissions (role_id, permission_id)
			SELECT r.id, $1 FROM identity.roles r WHERE r.key = 'admin_1_business'
			ON CONFLICT (role_id, permission_id) DO NOTHING`, permissionID)
		if err != nil {
			t.Errorf("restoring the seeded grant: %v", err)
		}
	}
	t.Cleanup(restore)

	// The member whose definition lost the key is refused, naming it, on the same cookie.
	status, e := b.call("business", path)
	if status != http.StatusForbidden || e.MissingPermission != "administration.audit_logs.read" {
		t.Errorf("admin_1_business after the removal: %d (%s, missing %q), want 403 naming the key",
			status, e.Code, e.MissingPermission)
	}

	// And the other member still holds it: the deletion narrowed one role, not the permission, so a
	// refusal here would mean the test had found a shared-cache bug rather than a role boundary.
	if status, e := b.call("super", path); status != http.StatusOK {
		t.Errorf("super_admin after the removal: %d (%s), want 200 — only one role definition changed",
			status, e.Code)
	}

	// Putting the key back restores the member on the same cookie too, which is the half that proves
	// the 403 above came from a live read rather than from a session that had quietly died.
	restore()
	if status, e := b.call("business", path); status != http.StatusOK {
		t.Errorf("admin_1_business after the grant went back: %d (%s), want 200 without signing in again",
			status, e.Code)
	}
}
