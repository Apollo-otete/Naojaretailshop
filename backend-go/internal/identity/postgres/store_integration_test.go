//go:build integration

package postgres

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/dbtest"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
)

// These tests are the other half of internal/identity's unit suite. The unit tests prove the
// rules; these prove the SQL behind the ports is the SQL the rules assume — that a duplicate
// address becomes ErrEmailTaken rather than a 500, that the sessions table holds a hash, that a
// rolled-back transaction takes the row and the grant with it.
//
// They run against a scratch database only (see internal/dbtest), and each starts from an empty
// identity.users, which is also the only table that has to be cleared: the roles, permissions and
// role grants are seeded by migration 0003 and the assertions below read them.

// refHash is the Argon2id reference vector from internal/password/argon2id_test.go — a real PHC
// string produced by a different implementation. The schema CHECK-constrains the column to this
// shape, so a test fixture cannot cheat with "hunter2".
const refHash = "$argon2id$v=19$m=65536,t=3,p=4$MDEyMzQ1Njc4OWFiY2RlZg$2x8WSIXNLdCDSjaiBZgpg8B9cq2w1ir2HdFIAe46OtU"

type fixture struct {
	pool  *pgxpool.Pool
	store *Store
	tx    *Tx
	now   time.Time
}

func setup(t *testing.T) *fixture {
	t.Helper()

	pool := dbtest.Pool(t)
	// Truncating the root cascades to sessions, grants, invitations and the audit log, and
	// RESTART IDENTITY is what makes "the first user has id 1" an assertion rather than a hope.
	dbtest.Truncate(t, pool, "identity.users")

	return &fixture{
		pool:  pool,
		store: mustStore(t, pool),
		tx:    NewTx(pool),
		now:   time.Now().UTC().Truncate(time.Microsecond),
	}
}

func mustStore(t *testing.T, pool *pgxpool.Pool) *Store {
	t.Helper()
	s, err := New(pool)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

// bgCtx is the context used outside a transaction. It is a helper rather than
// context.Background() at every call site, and it is not called ctx because the transaction
// callbacks below take a parameter with that name and shadowing it twice in one file is how a
// test ends up reading the pool when it means the transaction.
func bgCtx() context.Context { return context.Background() }

func (f *fixture) insertUser(t *testing.T, email, phone string) identity.User {
	t.Helper()

	u, err := identity.NewCustomer("Test Subject", email, phone, refHash)
	if err != nil {
		t.Fatalf("NewCustomer(%q, %q): %v", email, phone, err)
	}
	if err := f.store.CreateUser(bgCtx(), &u); err != nil {
		t.Fatalf("CreateUser(%q): %v", email, err)
	}
	return u
}

func (f *fixture) query(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(bgCtx(), sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func (f *fixture) scalarInt(t *testing.T, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := f.pool.QueryRow(bgCtx(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("scalar %q: %v", sql, err)
	}
	return n
}

func (f *fixture) scalarString(t *testing.T, sql string, args ...any) string {
	t.Helper()
	var s string
	if err := f.pool.QueryRow(bgCtx(), sql, args...).Scan(&s); err != nil {
		t.Fatalf("scalar %q: %v", sql, err)
	}
	return s
}

// ── users ──────────────────────────────────────────────────────────────────────────────────

func TestCreateUserRoundTripsEveryFieldTheDomainReads(t *testing.T) {
	f := setup(t)

	want := f.insertUser(t, "Achieng@mail.test", "0712345678")

	// The store fills the identity and the database maintains the timestamps.
	if want.ID != 1 {
		t.Errorf("generated id = %d, want 1 from a restarted sequence", want.ID)
	}
	if want.CreatedAt.IsZero() || want.UpdatedAt.IsZero() {
		t.Error("CreateUser did not read back created_at / updated_at")
	}

	byEmail, err := f.store.UserByEmail(bgCtx(), "achieng@mail.test")
	if err != nil {
		t.Fatalf("UserByEmail: %v", err)
	}
	byID, err := f.store.UserByID(bgCtx(), want.ID)
	if err != nil {
		t.Fatalf("UserByID: %v", err)
	}
	if !reflect.DeepEqual(byEmail, byID) {
		t.Errorf("the two lookups disagree:\n byEmail = %+v\n byID   = %+v", byEmail, byID)
	}

	// Normalisation is the domain's job and storage is the adapter's: what came back is the
	// +254 form and the lowercased address, not what the client typed.
	if byEmail.Email != "achieng@mail.test" {
		t.Errorf("email stored as %q, want the normalised form", byEmail.Email)
	}
	if byEmail.Phone != "+254712345678" {
		t.Errorf("phone stored as %q, want +254712345678", byEmail.Phone)
	}
	if byEmail.PasswordHash != refHash {
		t.Errorf("password hash was altered on the way in")
	}
	if !byEmail.IsActive || byEmail.IsSuspended || byEmail.FailedLogins != 0 {
		t.Errorf("a new account came back as %+v", byEmail)
	}
	if byEmail.LastLoginAt != nil || byEmail.LockedUntil != nil {
		t.Error("a new account has login history it did not earn")
	}
}

func TestTheTwoUniqueColumnsAreTheTwoRegisterConflicts(t *testing.T) {
	f := setup(t)

	f.insertUser(t, "taken@mail.test", "+254712345678")

	_, err := f.store.UserByEmail(bgCtx(), "nope@mail.test")
	if !errors.Is(err, identity.ErrNoUser) {
		t.Errorf("UserByEmail on a missing row = %v, want ErrNoUser", err)
	}

	dupEmail, _ := identity.NewCustomer("Second Person", "TAKEN@mail.test", "0722222222", refHash)
	if err := f.store.CreateUser(bgCtx(), &dupEmail); !errors.Is(err, identity.ErrEmailTaken) {
		t.Errorf("duplicate email = %v, want ErrEmailTaken", err)
	}

	dupPhone, _ := identity.NewCustomer("Third Person", "fresh@mail.test", "254712345678", refHash)
	if err := f.store.CreateUser(bgCtx(), &dupPhone); !errors.Is(err, identity.ErrPhoneTaken) {
		t.Errorf("the same number in a different spelling = %v, want ErrPhoneTaken", err)
	}
	// Both rejections have to be the constraint and nothing else: a wrapped 23505 that names the
	// wrong column sends the reader to the wrong table.
	if got := f.scalarInt(t, `SELECT count(*) FROM identity.users`); got != 1 {
		t.Errorf("%d users after two refused inserts, want 1", got)
	}
}

func TestAnAdminMayExistWithoutAPhoneNumber(t *testing.T) {
	f := setup(t)

	// phone is nullable and UNIQUE permits many NULLs. The adapter turns "" into NULL for
	// exactly this: two staff accounts with no phone are two accounts, not a collision.
	for _, email := range []string{"ops@mail.test", "biz@mail.test"} {
		u, err := identity.NewAdmin("Staff Member", email, "", refHash)
		if err != nil {
			t.Fatalf("NewAdmin: %v", err)
		}
		if err := f.store.CreateUser(bgCtx(), &u); err != nil {
			t.Fatalf("CreateUser(%q): %v", email, err)
		}
	}
	if got := f.scalarInt(t, `SELECT count(*) FROM identity.users WHERE phone IS NULL`); got != 2 {
		t.Errorf("%d phone-less users, want 2 — an empty string would have collided or lied", got)
	}
}

func TestTheSchemaRefusesAHashThatIsNotArgon2id(t *testing.T) {
	f := setup(t)

	for _, bad := range []string{"hunter2", "$2a$10$abcdefghijklmnopqrstuvwxyz01234567890123456789", ""} {
		_, err := f.pool.Exec(bgCtx(), `
			INSERT INTO identity.users (email, full_name, password_hash, account_kind)
			VALUES ($1, 'Test Subject', $2, 'customer')`,
			"bad"+strings.ReplaceAll(bad, "$", "")+"@mail.test", bad)
		if err == nil {
			t.Errorf("the database accepted password_hash %q", bad)
		}
	}
	if got := f.scalarInt(t, `SELECT count(*) FROM identity.users WHERE password_hash <> $1`, refHash); got != 0 {
		t.Errorf("%d rows hold a hash the format CHECK should have refused", got)
	}
}

func TestUpdateLoginStateWritesTheCounterAndTheLockTogether(t *testing.T) {
	f := setup(t)

	u := f.insertUser(t, "lockme@mail.test", "0712345678")

	lockedUntil := f.now.Add(15 * time.Minute)
	u.FailedLogins = 0
	u.LockedUntil = &lockedUntil
	if err := f.store.UpdateLoginState(bgCtx(), u); err != nil {
		t.Fatalf("UpdateLoginState: %v", err)
	}

	back, err := f.store.UserByID(bgCtx(), u.ID)
	if err != nil {
		t.Fatalf("UserByID: %v", err)
	}
	if back.FailedLogins != 0 || back.LockedUntil == nil {
		t.Fatalf("lock state came back as count=%d locked_until=%v", back.FailedLogins, back.LockedUntil)
	}
	if !back.LockedUntil.Equal(lockedUntil) {
		t.Errorf("locked_until = %v, want %v", back.LockedUntil, lockedUntil)
	}
	if back.LastLoginAt != nil {
		t.Error("a failed login stamped last_login_at")
	}

	// A success clears the lock column, not just the counter — a NULL written back has to arrive
	// as NULL, or an account stays locked after it has been signed in.
	if err := f.store.UpdateLoginState(bgCtx(), func() identity.User {
		blessed := back
		blessed.RegisterSuccess(f.now)
		return blessed
	}()); err != nil {
		t.Fatalf("UpdateLoginState after success: %v", err)
	}
	cleared, _ := f.store.UserByID(bgCtx(), u.ID)
	if cleared.LockedUntil != nil {
		t.Errorf("locked_until survived a successful login: %v", cleared.LockedUntil)
	}
	if cleared.LastLoginAt == nil || !cleared.LastLoginAt.Equal(f.now) {
		t.Errorf("last_login_at = %v, want %v", cleared.LastLoginAt, f.now)
	}

	missing := identity.User{ID: 9999}
	if err := f.store.UpdateLoginState(bgCtx(), missing); !errors.Is(err, identity.ErrNoUser) {
		t.Errorf("writing a nonexistent user = %v, want ErrNoUser", err)
	}
}

func TestUpdatePasswordReplacesTheHash(t *testing.T) {
	f := setup(t)
	u := f.insertUser(t, "rotate@mail.test", "0712345678")

	// A second well-formed PHC string with a different salt: only the format matters to the
	// column, and this keeps the assertion about the write rather than about Argon2.
	rotated := "$argon2id$v=19$m=65536,t=3,p=4$Zm9vYmFyYmF6cXV1eA$xTBLBn3GqDZ3cK3cQ9O8kKk1E0F7pQ2m3n4o5p6q7r8"
	if err := f.store.UpdatePassword(bgCtx(), u.ID, rotated); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	back, _ := f.store.UserByID(bgCtx(), u.ID)
	if back.PasswordHash != rotated {
		t.Errorf("hash = %q, want the replacement", back.PasswordHash)
	}
	if err := f.store.UpdatePassword(bgCtx(), 9999, refHash); !errors.Is(err, identity.ErrNoUser) {
		t.Errorf("updating a nonexistent user's password = %v, want ErrNoUser", err)
	}
}

// ── authority ──────────────────────────────────────────────────────────────────────────────

func TestAssignRoleIsIdempotentAndRefusesAnUnknownKey(t *testing.T) {
	f := setup(t)
	u := f.insertUser(t, "roles@mail.test", "0712345678")

	for i := 0; i < 2; i++ {
		if err := f.store.AssignRole(bgCtx(), u.ID, "customer", nil); err != nil {
			t.Fatalf("AssignRole #%d: %v", i+1, err)
		}
	}
	if got := f.scalarInt(t, `SELECT count(*) FROM identity.user_roles WHERE user_id = $1`, u.ID); got != 1 {
		t.Errorf("%d role rows after assigning twice, want 1 — self-service registration is not idempotent", got)
	}
	// The system grant has no grantor.
	if got := f.scalarString(t,
		`SELECT coalesce(granted_by::text, 'null') FROM identity.user_roles WHERE user_id = $1`, u.ID); got != "null" {
		t.Errorf("granted_by = %s, want NULL for a self-service role", got)
	}

	if err := f.store.AssignRole(bgCtx(), u.ID, "root", nil); !errors.Is(err, identity.ErrNoRole) {
		t.Errorf("unknown role key = %v, want ErrNoRole", err)
	}
	if err := f.store.AssignRole(bgCtx(), 9999, "customer", nil); err == nil {
		t.Error("assigning a role to a user that does not exist succeeded")
	}

	// A named grantor is recorded, because every human grant has to say who made it.
	grantor := f.insertUser(t, "issuer@mail.test", "0711111111").ID
	if err := f.store.AssignRole(bgCtx(), u.ID, "admin_2_operations", &grantor); err != nil {
		t.Fatalf("AssignRole with a grantor: %v", err)
	}
	if got := f.scalarInt(t,
		`SELECT count(*) FROM identity.user_roles WHERE user_id = $1 AND granted_by = $2`, u.ID, grantor); got != 1 {
		t.Errorf("the grantor was not recorded (%d rows)", got)
	}
}

func TestGrantsForIsTheSeededRoleUnionedExceptionalGrants(t *testing.T) {
	f := setup(t)
	u := f.insertUser(t, "grants@mail.test", "0712345678")
	if err := f.store.AssignRole(bgCtx(), u.ID, "customer", nil); err != nil {
		t.Fatalf("AssignRole: %v", err)
	}

	grants, err := f.store.GrantsFor(bgCtx(), u.ID)
	if err != nil {
		t.Fatalf("GrantsFor: %v", err)
	}
	if !reflect.DeepEqual(grants.Roles, []string{"customer"}) {
		t.Errorf("roles = %v, want [customer]", grants.Roles)
	}
	// 20 is the customer row of matrix §3.1 as seeded by 0003. If this number moves, the matrix
	// and the seed moved together or they did not — this is the assertion that notices.
	if len(grants.Permissions) != 20 {
		t.Errorf("customer holds %d permissions, want 20: %v", len(grants.Permissions), grants.Permissions)
	}
	if !sorted(grants.Permissions) {
		t.Errorf("permissions came back unsorted: %v", grants.Permissions)
	}
	for _, want := range []string{"shopping.cart.read", "ordering.orders.create", "identity.account.update"} {
		if !has(grants.Permissions, want) {
			t.Errorf("customer is missing %q", want)
		}
	}
	for _, none := range []string{"ordering.orders.read.all", "administration.audit_logs.read", "identity.roles.manage"} {
		if has(grants.Permissions, none) {
			t.Errorf("customer holds %q, which matrix §3 reserves for the admin roles", none)
		}
	}

	perm := func(key string) int64 {
		return f.scalarInt(t, `SELECT id FROM identity.permissions WHERE key = $1`, key)
	}
	grantee := u.ID

	// An exceptional grant that is live appears beside the role's.
	f.query(t, `
		INSERT INTO identity.user_permissions (user_id, permission_id, granted_by, reason, expires_at)
		VALUES ($1, $2, $3, 'incident 2026-11-04: read the audit log to trace a bad refund',
		        now() + interval '1 hour')`,
		grantee, perm("administration.audit_logs.read"), grantee)

	// One that has expired does not, and one that was revoked does not either.
	f.query(t, `
		INSERT INTO identity.user_permissions (user_id, permission_id, granted_by, reason, granted_at, expires_at)
		VALUES ($1, $2, $3, 'backfill that was finished early',
		        now() - interval '2 minutes', now() - interval '1 minute')`,
		grantee, perm("payments.refund"), grantee)
	f.query(t, `
		INSERT INTO identity.user_permissions (user_id, permission_id, granted_by, reason, expires_at, revoked_at, revoked_by)
		VALUES ($1, $2, $3, 'withdrawn stock adjustment cover', now() + interval '1 day',
		        now(), $3)`,
		grantee, perm("inventory.stock.adjust"), grantee)

	grants, err = f.store.GrantsFor(bgCtx(), u.ID)
	if err != nil {
		t.Fatalf("GrantsFor after grants: %v", err)
	}
	if !has(grants.Permissions, "administration.audit_logs.read") {
		t.Error("a live exceptional grant is missing from the resolved permissions")
	}
	for _, gone := range []string{"payments.refund", "inventory.stock.adjust"} {
		if has(grants.Permissions, gone) {
			t.Errorf("%q is granted although it expired or was revoked", gone)
		}
	}
	if len(grants.Permissions) != 21 {
		t.Errorf("resolved permissions = %d, want 20 role grants + 1 live exception: %v",
			len(grants.Permissions), grants.Permissions)
	}
}

// B12 at the database level: authority is read per request, so a permission removed from a role
// disappears from its members without anyone logging out.
func TestRemovingAPermissionFromARoleRemovesItFromItsMembers(t *testing.T) {
	f := setup(t)
	u := f.insertUser(t, "b12@mail.test", "0712345678")
	if err := f.store.AssignRole(bgCtx(), u.ID, "customer", nil); err != nil {
		t.Fatalf("AssignRole: %v", err)
	}

	before, err := f.store.GrantsFor(bgCtx(), u.ID)
	if err != nil {
		t.Fatalf("GrantsFor: %v", err)
	}
	if !has(before.Permissions, "shopping.cart.read") {
		t.Fatal("the seed no longer grants shopping.cart.read to customers; this test is asserting the wrong thing")
	}

	f.query(t, `
		DELETE FROM identity.role_permissions rp
		USING identity.roles r, identity.permissions p
		WHERE rp.role_id = r.id AND rp.permission_id = p.id
		  AND r.key = 'customer' AND p.key = 'shopping.cart.read'`)
	t.Cleanup(func() {
		f.query(t, `
			INSERT INTO identity.role_permissions (role_id, permission_id)
			SELECT r.id, p.id FROM identity.roles r, identity.permissions p
			WHERE r.key = 'customer' AND p.key = 'shopping.cart.read'
			ON CONFLICT DO NOTHING`)
	})

	after, err := f.store.GrantsFor(bgCtx(), u.ID)
	if err != nil {
		t.Fatalf("GrantsFor after removal: %v", err)
	}
	if has(after.Permissions, "shopping.cart.read") {
		t.Error("the removed permission is still resolved for the member")
	}
	if len(after.Permissions) != len(before.Permissions)-1 {
		t.Errorf("removing one permission changed the count %d -> %d", len(before.Permissions), len(after.Permissions))
	}
}

// ── sessions ───────────────────────────────────────────────────────────────────────────────

func (f *fixture) newSession(t *testing.T, userID int64) identity.Session {
	t.Helper()

	sess, err := identity.NewSession(userID, false,
		identity.Policy{Idle: 30 * 24 * time.Hour, AdminIdle: 12 * time.Hour, Absolute: 7 * 24 * time.Hour},
		f.now, "203.0.113.7", "Mozilla/5.0 (test harness)")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if err := f.store.CreateSession(bgCtx(), &sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	return sess
}

func TestSessionsStoreTheHashAndNeverTheToken(t *testing.T) {
	f := setup(t)
	u := f.insertUser(t, "sess@mail.test", "0712345678")
	sess := f.newSession(t, u.ID)

	if sess.ID == 0 {
		t.Fatal("CreateSession did not read back the id")
	}
	// The whole point of storing a hash: a dump of this table must not hand over live
	// credentials. Checked in the column, not in the Go value, because the Go value is where the
	// bug would be invisible.
	if got := f.scalarString(t, `SELECT session_hash FROM identity.sessions WHERE id = $1`, sess.ID); got != sess.Hash {
		t.Errorf("session_hash = %q, want the hash", got)
	}
	if got := f.scalarInt(t, `SELECT count(*) FROM identity.sessions WHERE session_hash = $1`, sess.Token); got != 0 {
		t.Error("the cookie value itself is stored, which makes the table a list of live credentials")
	}
	// The CSRF token is the exception, and it is stored in the clear on purpose: GET
	// /auth/session has to hand it back, which a hash cannot do.
	if got := f.scalarString(t, `SELECT csrf_token FROM identity.sessions WHERE id = $1`, sess.ID); got != sess.CSRFToken {
		t.Errorf("csrf_token = %q, want %q", got, sess.CSRFToken)
	}
	if got := f.scalarString(t, `SELECT host(ip) FROM identity.sessions WHERE id = $1`, sess.ID); got != "203.0.113.7" {
		t.Errorf("ip = %q, want the address it was given", got)
	}

	back, err := f.store.SessionByHash(bgCtx(), sess.Hash)
	if err != nil {
		t.Fatalf("SessionByHash: %v", err)
	}
	if back.ID != sess.ID || back.UserID != u.ID || back.CSRFToken != sess.CSRFToken {
		t.Errorf("session round trip = %+v, want %+v", back, sess)
	}
	if back.RevokedAt != nil || back.RevokedReason != "" {
		t.Error("a live session came back revoked")
	}
	if back.IP != "203.0.113.7" || back.UserAgent != "Mozilla/5.0 (test harness)" {
		t.Errorf("session context lost: ip=%q user_agent=%q", back.IP, back.UserAgent)
	}

	if _, err := f.store.SessionByHash(bgCtx(), strings.Repeat("0", 64)); !errors.Is(err, identity.ErrSessionNotFound) {
		t.Errorf("unknown hash = %v, want ErrSessionNotFound", err)
	}
	// Two cookies for one account are two rows; the uniqueness that matters is on the hash.
	second := f.newSession(t, u.ID)
	if second.Hash == sess.Hash {
		t.Fatal("two sessions were minted with the same token")
	}
	if got := f.scalarInt(t, `SELECT count(*) FROM identity.sessions WHERE user_id = $1`, u.ID); got != 2 {
		t.Errorf("%d sessions for one user, want 2", got)
	}
}

func TestTouchSessionMovesBothEndsOfTheSlidingWindow(t *testing.T) {
	f := setup(t)
	u := f.insertUser(t, "touch@mail.test", "0712345678")
	sess := f.newSession(t, u.ID)

	later := f.now.Add(10 * time.Minute)
	newExpiry := f.now.Add(24 * time.Hour).Add(10 * time.Minute)
	if err := f.store.TouchSession(bgCtx(), sess.ID, later, newExpiry); err != nil {
		t.Fatalf("TouchSession: %v", err)
	}

	back, err := f.store.SessionByHash(bgCtx(), sess.Hash)
	if err != nil {
		t.Fatalf("SessionByHash: %v", err)
	}
	if !back.LastSeenAt.Equal(later) {
		t.Errorf("last_seen_at = %v, want %v", back.LastSeenAt, later)
	}
	if !back.ExpiresAt.Equal(newExpiry) {
		t.Errorf("expires_at = %v, want %v", back.ExpiresAt, newExpiry)
	}
	// The creation time is what the absolute cap is measured from; if a touch moved it, the cap
	// would slide away and a session could live forever.
	if !back.CreatedAt.Equal(sess.CreatedAt) {
		t.Errorf("created_at moved from %v to %v", sess.CreatedAt, back.CreatedAt)
	}

	if err := f.store.TouchSession(bgCtx(), 9999, later, newExpiry); err != nil {
		t.Errorf("touching a nonexistent session failed: %v", err)
	}
}

func TestRevokingIsIdempotentAndNamesItsReason(t *testing.T) {
	f := setup(t)
	u := f.insertUser(t, "revoke@mail.test", "0712345678")
	sess := f.newSession(t, u.ID)
	at := f.now.Add(time.Minute)

	first, err := f.store.RevokeSession(bgCtx(), sess.ID, identity.RevokedLogout, at)
	if err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}
	if !first {
		t.Error("the first revocation reported that it did nothing")
	}

	back, _ := f.store.SessionByHash(bgCtx(), sess.Hash)
	if back.RevokedAt == nil || !back.RevokedAt.Equal(at) {
		t.Errorf("revoked_at = %v, want %v", back.RevokedAt, at)
	}
	if back.RevokedReason != identity.RevokedLogout {
		t.Errorf("revoked_reason = %q, want logout", back.RevokedReason)
	}

	// A repeated logout revokes nothing but must not fail: the user is signed out either way, and
	// the contract calls this 204.
	again, err := f.store.RevokeSession(bgCtx(), sess.ID, identity.RevokedLogout, at)
	if err != nil {
		t.Fatalf("second RevokeSession: %v", err)
	}
	if again {
		t.Error("the second revocation claimed to have revoked something")
	}

	if _, err := f.store.RevokeSession(bgCtx(), 9999, identity.RevokedLogout, at); !errors.Is(err, identity.ErrSessionNotFound) {
		t.Errorf("revoking a missing session = %v, want ErrSessionNotFound", err)
	}

	// An unknown reason is refused by the domain before the database is asked, so the CHECK
	// constraint is a backstop rather than the only defence.
	if _, err := f.store.RevokeSession(bgCtx(), sess.ID, identity.RevokeReason("because"), at); !errors.Is(err, identity.ErrUnknownRevoke) {
		t.Errorf("unknown reason = %v, want ErrUnknownRevoke", err)
	}
	if got := f.scalarString(t,
		`SELECT revoked_reason FROM identity.sessions WHERE id = $1`, sess.ID); got != "logout" {
		t.Errorf("the refused call still rewrote the row: reason = %s", got)
	}
}

func TestRevokeAllExceptLeavesOneSessionLive(t *testing.T) {
	f := setup(t)
	u := f.insertUser(t, "everywhere@mail.test", "0712345678")

	kept := f.newSession(t, u.ID)
	others := []identity.Session{f.newSession(t, u.ID), f.newSession(t, u.ID)}
	// A second account's session must survive: revoking "all other sessions" across users would
	// sign out the whole shop on someone else's password change.
	stranger := f.insertUser(t, "neighbour@mail.test", "0712345679")
	strangerSess := f.newSession(t, stranger.ID)

	n, err := f.store.RevokeAllExcept(bgCtx(), u.ID, kept.ID, identity.RevokedPasswordChanged, f.now)
	if err != nil {
		t.Fatalf("RevokeAllExcept: %v", err)
	}
	if n != 2 {
		t.Errorf("revoked %d sessions, want 2", n)
	}
	for _, s := range others {
		row, err := f.store.SessionByHash(bgCtx(), s.Hash)
		if err != nil {
			t.Fatalf("SessionByHash: %v", err)
		}
		if row.RevokedAt == nil || row.RevokedReason != identity.RevokedPasswordChanged {
			t.Errorf("session %d survived the revocation", s.ID)
		}
	}
	live, _ := f.store.SessionByHash(bgCtx(), kept.Hash)
	if live.RevokedAt != nil {
		t.Error("the current session was revoked by its own password change")
	}
	survivor, _ := f.store.SessionByHash(bgCtx(), strangerSess.Hash)
	if survivor.RevokedAt != nil {
		t.Error("Revoking another account's sessions reached this one")
	}

	if n, err := f.store.RevokeAllExcept(bgCtx(), 9999, 1, identity.RevokedAdmin, f.now); err != nil || n != 0 {
		t.Errorf("revoking for an unknown user = (%d, %v), want (0, nil)", n, err)
	}
}

// ── transactions ───────────────────────────────────────────────────────────────────────────

func TestAFailedUnitOfWorkTakesTheRowAndTheGrantWithIt(t *testing.T) {
	f := setup(t)

	err := f.tx.RunInTx(bgCtx(), func(ctx context.Context) error {
		u, err := identity.NewCustomer("Half Done", "half@mail.test", "0712345678", refHash)
		if err != nil {
			return err
		}
		if err := f.store.CreateUser(ctx, &u); err != nil {
			return err
		}
		if err := f.store.AssignRole(ctx, u.ID, "customer", nil); err != nil {
			return err
		}
		sess, err := identity.NewSession(u.ID, false,
			identity.Policy{Idle: time.Hour, AdminIdle: time.Hour, Absolute: 24 * time.Hour},
			f.now, "203.0.113.7", "harness")
		if err != nil {
			return err
		}
		if err := f.store.CreateSession(ctx, &sess); err != nil {
			return err
		}
		// The row is visible here, so the disappearance asserted below is the rollback's doing
		// and not a write that quietly failed.
		if _, err := f.store.UserByEmail(ctx, "half@mail.test"); err != nil {
			return err
		}
		// The failure is the one Register is written to survive: the audit entry that comes
		// after the change, and does not arrive.
		return errors.New("simulated failure after every write succeeded")
	})
	if err == nil {
		t.Fatal("a transaction that returned an error committed")
	}

	if got := f.scalarInt(t, `SELECT count(*) FROM identity.users WHERE email = 'half@mail.test'`); got != 0 {
		t.Errorf("%d users survived a rolled-back transaction", got)
	}
	// Both dependent rows go with it — this is what the adapter joining the transaction buys, and
	// the difference between T-10 holding and T-10 being a comment.
	if got := f.scalarInt(t, `SELECT count(*) FROM identity.user_roles`); got != 0 {
		t.Errorf("%d role grants survived", got)
	}
	if got := f.scalarInt(t, `SELECT count(*) FROM identity.sessions`); got != 0 {
		t.Errorf("%d sessions survived", got)
	}
	if _, err := f.store.UserByEmail(bgCtx(), "half@mail.test"); !errors.Is(err, identity.ErrNoUser) {
		t.Errorf("reading the rolled-back user outside any transaction = %v, want ErrNoUser", err)
	}
}

func TestWritesInsideATransactionAreVisibleToItselfOnly(t *testing.T) {
	f := setup(t)

	var id int64
	if err := f.tx.RunInTx(bgCtx(), func(ctx context.Context) error {
		u, err := identity.NewCustomer("Inside Out", "inside@mail.test", "0712345678", refHash)
		if err != nil {
			return err
		}
		if err := f.store.CreateUser(ctx, &u); err != nil {
			return err
		}
		id = u.ID
		// Reading through the pool here would raise "no such user" and that would be correct.
		if _, err := f.store.UserByID(ctx, id); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("RunInTx: %v", err)
	}

	back, err := f.store.UserByID(bgCtx(), id)
	if err != nil {
		t.Fatalf("the committed user is gone: %v", err)
	}
	if back.Email != "inside@mail.test" {
		t.Errorf("committed user = %q", back.Email)
	}
}

func TestRunInTxWithoutAPoolRefusesRatherThanPanics(t *testing.T) {
	if err := NewTx(nil).RunInTx(bgCtx(), func(context.Context) error { return nil }); err == nil {
		t.Error("a transaction runner with no pool reported success")
	}
}

// ── helpers ────────────────────────────────────────────────────────────────────────────────

func sorted(xs []string) bool {
	for i := 1; i < len(xs); i++ {
		if xs[i-1] > xs[i] {
			return false
		}
	}
	return true
}

func has(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
