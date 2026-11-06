package identity_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/audit"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
)

const (
	goodPassword = "correct horse battery staple"
	betterNew    = "a different passphrase entirely"
	shopEmail    = "achieng.example@mail.test"
	shopPhone    = "0712345678"
)

func TestRegisterIssuesACustomerSessionAndNothingElse(t *testing.T) {
	h := newHarness(t)
	auth := h.register(shopEmail, shopPhone)

	if got := auth.Principal.User.Account; got != identity.KindCustomer {
		t.Errorf("account kind = %q, want customer", got)
	}
	if auth.Session.Token == "" || auth.Session.Hash == "" || auth.Session.CSRFToken == "" {
		t.Error("a new session must carry a token, its hash and a CSRF token")
	}
	if auth.Session.Hash == auth.Session.Token {
		t.Error("the session hash stored the token instead of hashing it")
	}

	// Registration grants the customer role and nothing else: there is no input to this
	// endpoint that produces an admin, which is the first line of B1 and B2.
	if len(auth.Principal.Roles) != 1 || auth.Principal.Roles[0] != identity.CustomerRoleKey {
		t.Errorf("roles = %v, want exactly [customer]", auth.Principal.Roles)
	}
	for _, perm := range auth.Principal.Permissions {
		if strings.HasPrefix(perm, "identity.") || strings.HasPrefix(perm, "administration.") {
			t.Errorf("a self-registered customer holds %q", perm)
		}
	}
}

func TestRegisterRejectsAWeakPasswordWithoutCreatingAnAccount(t *testing.T) {
	h := newHarness(t)
	_, err := h.service.Register(context.Background(), identity.RegisterInput{
		FullName: "Achieng Otieno", Email: "short@mail.test", Phone: shopPhone, Password: "twotwo",
	})
	if !errors.Is(err, identity.ErrPasswordTooShort) {
		t.Errorf("Register with a 7-character password = %v, want ErrPasswordTooShort", err)
	}
	if n := h.store.countUsers(); n != 0 {
		t.Errorf("a rejected registration left %d account(s) behind", n)
	}
}

func TestRegisterRefusesTheSecondAccountOnOneEmailOrPhone(t *testing.T) {
	h := newHarness(t)
	h.register(shopEmail, shopPhone)

	_, err := h.service.Register(context.Background(), identity.RegisterInput{
		FullName: "Another Person", Email: strings.ToUpper(shopEmail), Phone: "0712345679", Password: goodPassword,
	})
	if !errors.Is(err, identity.ErrEmailTaken) {
		t.Errorf("re-registered email = %v, want ErrEmailTaken (CITEXT makes case irrelevant)", err)
	}

	_, err = h.service.Register(context.Background(), identity.RegisterInput{
		FullName: "Another Person", Email: "someoneelse@mail.test", Phone: "+254" + shopPhone[1:], Password: goodPassword,
	})
	if !errors.Is(err, identity.ErrPhoneTaken) {
		t.Errorf("re-registered phone = %v, want ErrPhoneTaken (0712… and +254712… are one number)", err)
	}
}

// ERD §1 and T-10: a privileged or stateful write and everything that must travel with it are
// one transaction. Here the session write fails after the account row succeeded.
func TestRegisterIsOneUnitOfWork(t *testing.T) {
	h := newHarness(t)
	h.store.failCreateSession = true

	_, err := h.service.Register(context.Background(), identity.RegisterInput{
		FullName: "Achieng Otieno", Email: shopEmail, Phone: shopPhone, Password: goodPassword,
	})
	if err == nil {
		t.Fatal("Register succeeded although CreateSession was set to fail")
	}
	if n := h.store.countUsers(); n != 0 {
		t.Errorf("%d account(s) survive a registration whose session failed; the write is not one unit of work", n)
	}
	if _, err := h.store.UserByEmail(context.Background(), shopEmail); !errors.Is(err, identity.ErrNoUser) {
		t.Errorf("the rolled-back account is still readable: %v", err)
	}
}

func TestLoginClearsTheFailureCounterAndIssuesASession(t *testing.T) {
	h := newHarness(t)
	auth := h.register(shopEmail, shopPhone)
	h.store.mutate(auth.Principal.UserID(), func(u *identity.User) { u.FailedLogins = 4 })

	got, err := h.service.Login(context.Background(), identity.LoginInput{
		Email: shopEmail, Password: goodPassword, IP: "196.189.1.11",
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if after := h.store.user(auth.Principal.UserID()); after.FailedLogins != 0 || after.LockedUntil != nil {
		t.Errorf("a successful login left failed=%d locked=%v", after.FailedLogins, after.LockedUntil)
	}
	if after := h.store.user(auth.Principal.UserID()); after.LastLoginAt == nil {
		t.Error("last_login_at was not recorded")
	}
	if got.Session.Token == auth.Session.Token {
		t.Error("two logins minted the same session token")
	}
	if len(got.Principal.Permissions) != len(auth.Principal.Permissions) {
		t.Error("login and registration disagree about what this account may do")
	}
}

// Matrix C1's rule, applied to login: the answer must not say whether the address exists.
func TestLoginCannotBeUsedToDiscoverAnAccount(t *testing.T) {
	h := newHarness(t)
	h.register(shopEmail, shopPhone)

	_, unknown := h.service.Login(context.Background(), identity.LoginInput{Email: "nobody@mail.test", Password: goodPassword})
	_, wrong := h.service.Login(context.Background(), identity.LoginInput{Email: shopEmail, Password: "not the password"})

	if !errors.Is(unknown, identity.ErrInvalidCredentials) || !errors.Is(wrong, identity.ErrInvalidCredentials) {
		t.Fatalf("unknown=%v wrong=%v, want ErrInvalidCredentials for both", unknown, wrong)
	}
	if unknown.Error() != wrong.Error() {
		t.Errorf("the two answers differ in text: %q vs %q", unknown, wrong)
	}
}

// The 401 text is only half of non-disclosure: if an unknown email returns without touching the
// hash function, timing alone identifies registered accounts.
func TestUnknownEmailStillPaysForHashing(t *testing.T) {
	h := newHarness(t)
	h.register(shopEmail, shopPhone)

	_, err := h.service.Login(context.Background(), identity.LoginInput{Email: "nobody@mail.test", Password: goodPassword})
	if !errors.Is(err, identity.ErrInvalidCredentials) {
		t.Fatalf("Login(unknown) = %v", err)
	}

	hashes, verifies := h.hasher.calls()
	// One hash for the dummy value prepared at construction, one for the registration, and at
	// least one verify for this attempt — which is the point: the expensive path ran.
	if hashes < 2 {
		t.Errorf("hash calls = %d, want the constructed dummy hash plus registration", hashes)
	}
	if verifies < 1 {
		t.Errorf("verify calls = %d for an unknown email; the timing-safe branch was skipped", verifies)
	}
}

func TestFiveFailuresLockTheAccountOnceAndAreAuditedOnce(t *testing.T) {
	h := newHarness(t)
	auth := h.register(shopEmail, shopPhone)

	var lastErr error
	for i := 0; i < h.login.MaxFailedAttempts; i++ {
		_, lastErr = h.service.Login(context.Background(), identity.LoginInput{
			Email: shopEmail, Password: "guess", IP: "196.189.1.12",
		})
		if !errors.Is(lastErr, identity.ErrInvalidCredentials) {
			t.Fatalf("attempt %d = %v, want ErrInvalidCredentials", i+1, lastErr)
		}
	}

	user := h.store.user(auth.Principal.UserID())
	if user.LockedUntil == nil || !user.LockedUntil.After(h.now) {
		t.Fatalf("after %d failures locked_until = %v, want a future instant", h.login.MaxFailedAttempts, user.LockedUntil)
	}
	if user.FailedLogins != 0 {
		t.Errorf("failed count = %d after locking, want the counter reset by the lock", user.FailedLogins)
	}

	// A further attempt cannot re-lock it, so the audit log records one security event and not
	// one per guess.
	for i := 0; i < 3; i++ {
		_, err := h.service.Login(context.Background(), identity.LoginInput{Email: shopEmail, Password: "guess"})
		if !errors.Is(err, identity.ErrLocked) {
			t.Errorf("attempt while locked = %v, want ErrLocked", err)
		}
	}
	entries := h.store.auditEntries()
	if len(entries) != 1 {
		t.Fatalf("audit entries = %d, want exactly the one lockout event", len(entries))
	}
	e := entries[0]
	if e.Action != "identity.account.lockout" || e.ActorKind != audit.KindUser {
		t.Errorf("audit = %+v, want a user-attributed lockout", e)
	}
	if e.IP != "196.189.1.12" {
		t.Errorf("lockout audit ip = %q, want the address that triggered it", e.IP)
	}
	if dumped := fmtEntry(e); strings.Contains(dumped, "guess") {
		t.Errorf("the audit entry recorded a guessed password: %s", dumped)
	}
}

func TestLoginStopsWorkingWhenTheAccountIsSuspended(t *testing.T) {
	h := newHarness(t)
	auth := h.register(shopEmail, shopPhone)
	h.store.suspend(auth.Principal.UserID())

	_, err := h.service.Login(context.Background(), identity.LoginInput{Email: shopEmail, Password: goodPassword})
	// The message is the one for a wrong password: confirming that a suspended address exists
	// is the disclosure C1 forbids.
	if !errors.Is(err, identity.ErrInvalidCredentials) {
		t.Errorf("Login on a suspended account = %v, want the generic credential error", err)
	}
}

func TestLoginSeparatesAWrongPasswordFromABrokenHash(t *testing.T) {
	h := newHarness(t)
	auth := h.register(shopEmail, shopPhone)

	// Corrupt the stored credential the way a bad migration would: not a hash at all. The
	// fake hasher reports that as something other than a mismatch, which is the branch under
	// test.
	h.store.mutate(auth.Principal.UserID(), func(u *identity.User) { u.PasswordHash = "not-a-hash" })

	_, err := h.service.Login(context.Background(), identity.LoginInput{Email: shopEmail, Password: goodPassword})
	if err == nil || errors.Is(err, identity.ErrInvalidCredentials) {
		t.Errorf("a stored hash that cannot be verified answered %v; it must not be reported as a wrong password", err)
	}
	if user := h.store.user(auth.Principal.UserID()); user.FailedLogins != 0 || user.LockedUntil != nil {
		t.Error("an infrastructure failure was charged to the account holder as a failed guess")
	}
}

// B11: "a suspended user, or a user whose permission was revoked, fails on the next request —
// not only on the next login."
func TestSuspensionTakesEffectOnTheNextRequest(t *testing.T) {
	h := newHarness(t)
	auth := h.register(shopEmail, shopPhone)
	token := auth.Session.Token

	if _, err := h.service.Authenticate(context.Background(), token); err != nil {
		t.Fatalf("Authenticate before suspension: %v", err)
	}
	h.store.suspend(auth.Principal.UserID())

	_, err := h.service.Authenticate(context.Background(), token)
	if !errors.Is(err, identity.ErrSuspended) {
		t.Fatalf("Authenticate after suspension = %v, want ErrSuspended", err)
	}

	// And it comes back the moment the suspension is lifted, with no new login.
	h.store.mutate(auth.Principal.UserID(), func(u *identity.User) { u.IsSuspended = false })
	if _, err := h.service.Authenticate(context.Background(), token); err != nil {
		t.Errorf("Authenticate after reactivation = %v, want success", err)
	}
}

// B12: removing a permission from a role removes it from every member without re-login.
func TestRolePermissionChangesApplyWithoutReLogin(t *testing.T) {
	h := newHarness(t)
	auth := h.register(shopEmail, shopPhone)
	token := auth.Session.Token

	if !auth.Principal.Has("ordering.orders.create") {
		t.Fatal("the seeded customer role is missing the permission this test removes")
	}
	h.store.removeRolePermission(identity.CustomerRoleKey, "ordering.orders.create")

	principal, err := h.service.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if principal.Has("ordering.orders.create") {
		t.Error("a permission removed from the role is still held by its member")
	}
	if !principal.Has("shopping.cart.read") {
		t.Error("the rest of the role disappeared with the one permission")
	}
}

func TestEffectivePermissionsAreRolesPlusUnexpiredExceptionalGrants(t *testing.T) {
	h := newHarness(t)
	auth := h.register(shopEmail, shopPhone)
	h.store.addExceptional(auth.Principal.UserID(), "promotions.campaigns.manage")

	principal, err := h.service.Authenticate(context.Background(), auth.Session.Token)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if !principal.Has("promotions.campaigns.manage") {
		t.Error("an exceptional grant is missing from the effective set the contract describes")
	}
	if !principal.Has("shopping.cart.read") {
		t.Error("the exceptional grant replaced the role grants instead of adding to them")
	}
}

func TestExpiredSessionIsRejectedAndAnActiveOneSlides(t *testing.T) {
	h := newHarness(t)
	auth := h.register(shopEmail, shopPhone)

	h.advance(h.policy.Idle)
	_, err := h.service.Authenticate(context.Background(), auth.Session.Token)
	if !errors.Is(err, identity.ErrSessionExpired) {
		t.Fatalf("Authenticate after the idle window = %v, want ErrSessionExpired", err)
	}

	// Sliding, within the cap: activity just inside the window extends it.
	h2 := newHarness(t)
	auth2 := h2.register("slide@mail.test", "0712345680")
	for i := 0; i < 3; i++ {
		h2.advance(h2.policy.Idle - time.Minute)
		if _, err := h2.service.Authenticate(context.Background(), auth2.Session.Token); err != nil {
			t.Fatalf("Authenticate tick %d: %v", i, err)
		}
	}
	stored, err := h2.store.SessionByHash(context.Background(), auth2.Session.Hash)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.ExpiresAt.After(h2.now) {
		t.Errorf("expires_at %v is not ahead of the clock %v after sliding", stored.ExpiresAt, h2.now)
	}
}

// A session that is kept alive must still die at the absolute cap. This is the half of ADR-019's
// expiry rule that a sliding window alone cannot express.
func TestTheAbsoluteCapStopsSlidingForever(t *testing.T) {
	h := newHarness(t)
	auth := h.register(shopEmail, shopPhone)
	deadline := auth.Session.CreatedAt.Add(h.policy.Absolute)

	for h.now.Before(deadline) {
		h.advance(time.Hour)
		if !h.now.Before(deadline) {
			// The cap is inclusive: a session is dead at the instant it turns seven days old,
			// not one tick later. Stepping past it is the last assertion's job.
			break
		}
		if _, err := h.service.Authenticate(context.Background(), auth.Session.Token); err != nil {
			t.Fatalf("a session kept active for %s died: %v", h.now.Sub(auth.Session.CreatedAt), err)
		}
		stored, err := h.store.SessionByHash(context.Background(), auth.Session.Hash)
		if err != nil {
			t.Fatal(err)
		}
		if stored.ExpiresAt.After(deadline) {
			t.Fatalf("sliding pushed expires_at %v past the absolute deadline %v", stored.ExpiresAt, deadline)
		}
	}

	h.now = deadline.Add(time.Minute)
	if _, err := h.service.Authenticate(context.Background(), auth.Session.Token); !errors.Is(err, identity.ErrSessionExpired) {
		t.Fatalf("Authenticate past the absolute deadline = %v, want ErrSessionExpired", err)
	}
}

func TestAdminSessionsExpireSoonerThanCustomerOnes(t *testing.T) {
	h := newHarness(t)
	if h.policy.AdminIdle >= h.policy.Idle {
		t.Fatal("the test policy does not express ADR-019's shorter admin idle window")
	}
	session, err := identity.NewSession(1, true, h.policy, h.now, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if want := h.now.Add(h.policy.AdminIdle); !session.ExpiresAt.Equal(want) {
		t.Errorf("admin session expires at %v, want %v", session.ExpiresAt, want)
	}
}

func TestLogoutIsIdempotentAndRevokesOnlyItsOwnSession(t *testing.T) {
	h := newHarness(t)
	first := h.register(shopEmail, shopPhone)
	principal, err := h.service.Authenticate(context.Background(), first.Session.Token)
	if err != nil {
		t.Fatal(err)
	}
	// A second, unrelated session for the same account: logout must not touch it.
	other, err := h.service.Login(context.Background(), identity.LoginInput{Email: shopEmail, Password: goodPassword})
	if err != nil {
		t.Fatal(err)
	}

	if err := h.service.Logout(context.Background(), principal); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if err := h.service.Logout(context.Background(), principal); err != nil {
		t.Fatalf("second Logout: %v", err)
	}
	if _, err := h.service.Authenticate(context.Background(), first.Session.Token); !errors.Is(err, identity.ErrSessionRevoked) {
		t.Errorf("Authenticate after logout = %v, want ErrSessionRevoked", err)
	}
	if _, err := h.service.Authenticate(context.Background(), other.Session.Token); err != nil {
		t.Errorf("the account's other session was affected by a single logout: %v", err)
	}
}

// ADR-024's second gate reads Principal.Conditions, so the snapshot has to arrive together with
// the permissions it narrows — one read, one moment, and the same values on both paths that
// build a principal. A divergence between the register response and the next request's read is
// a session whose /auth/session body describes a caller the evaluator disagrees about.
func TestThePrincipalCarriesTheConditionsDeclaredForItsPermissions(t *testing.T) {
	h := newHarness(t)
	auth := h.register(shopEmail, shopPhone)

	if got := auth.Principal.Conditions["shopping.cart.read"]; len(got) != 1 || got[0] != conditionOwnerIsCaller {
		t.Fatalf("register issued %v, want the declared owner_is_caller", auth.Principal.Conditions)
	}

	principal, err := h.service.Authenticate(context.Background(), auth.Session.Token)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(principal.Conditions, auth.Principal.Conditions) {
		t.Errorf("the next read reports %v where the login reported %v",
			principal.Conditions, auth.Principal.Conditions)
	}

	// B12's shape, for a condition instead of a permission: changing what the catalogue declares
	// changes the next request, with nobody signing in again.
	h.store.scoped["shopping.cart.read"] = nil
	after, err := h.service.Authenticate(context.Background(), auth.Session.Token)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Conditions["shopping.cart.read"]) != 0 {
		t.Error("a condition removed from the catalogue is still being carried")
	}
	if len(after.Permissions) != len(principal.Permissions) {
		t.Errorf("removing a condition moved a permission too: %v", after.Permissions)
	}
}

func TestAuthenticateRejectsAnUnknownOrEmptyToken(t *testing.T) {
	h := newHarness(t)
	if _, err := h.service.Authenticate(context.Background(), ""); !errors.Is(err, identity.ErrSessionNotFound) {
		t.Errorf("empty token = %v, want ErrSessionNotFound", err)
	}
	if _, err := h.service.Authenticate(context.Background(), "forged-value"); !errors.Is(err, identity.ErrSessionNotFound) {
		t.Errorf("forged token = %v, want ErrSessionNotFound", err)
	}
}

func TestChangePasswordRequiresTheCurrentPasswordAndReplacesIt(t *testing.T) {
	h := newHarness(t)
	auth := h.register(shopEmail, shopPhone)
	principal, err := h.service.Authenticate(context.Background(), auth.Session.Token)
	if err != nil {
		t.Fatal(err)
	}

	err = h.service.ChangePassword(context.Background(), principal, "wrong current", betterNew)
	if !errors.Is(err, identity.ErrInvalidCredentials) {
		t.Errorf("wrong current password = %v, want ErrInvalidCredentials", err)
	}
	if err := h.service.ChangePassword(context.Background(), principal, goodPassword, "short"); !errors.Is(err, identity.ErrPasswordTooShort) {
		t.Errorf("weak new password = %v, want ErrPasswordTooShort", err)
	}
	if err := h.service.ChangePassword(context.Background(), principal, goodPassword, goodPassword); !errors.Is(err, identity.ErrSamePassword) {
		t.Errorf("unchanged password = %v, want ErrSamePassword", err)
	}
	if user := h.store.user(principal.UserID()); user.PasswordHash != fakeSum(goodPassword) {
		t.Error("a rejected change still replaced the password")
	}

	if err := h.service.ChangePassword(context.Background(), principal, goodPassword, betterNew); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if user := h.store.user(principal.UserID()); user.PasswordHash != fakeSum(betterNew) {
		t.Error("the stored hash was not replaced")
	}
	if _, err := h.service.Login(context.Background(), identity.LoginInput{Email: shopEmail, Password: goodPassword}); err == nil {
		t.Error("the old password still works")
	}
	if _, err := h.service.Login(context.Background(), identity.LoginInput{Email: shopEmail, Password: betterNew}); err != nil {
		t.Errorf("the new password does not work: %v", err)
	}
}

// ADR-019: a password change revokes every *other* session immediately, and the one that asked
// for the change keeps working — otherwise the user is logged out mid-action by their own call.
func TestChangePasswordRevokesOtherSessionsButNotThisOne(t *testing.T) {
	h := newHarness(t)
	first := h.register(shopEmail, shopPhone)
	principal, err := h.service.Authenticate(context.Background(), first.Session.Token)
	if err != nil {
		t.Fatal(err)
	}
	elsewhere, err := h.service.Login(context.Background(), identity.LoginInput{Email: shopEmail, Password: goodPassword})
	if err != nil {
		t.Fatal(err)
	}

	if err := h.service.ChangePassword(context.Background(), principal, goodPassword, betterNew); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if _, err := h.service.Authenticate(context.Background(), elsewhere.Session.Token); !errors.Is(err, identity.ErrSessionRevoked) {
		t.Errorf("the other session = %v, want ErrSessionRevoked", err)
	}
	if _, err := h.service.Authenticate(context.Background(), first.Session.Token); err != nil {
		t.Errorf("the requesting session stopped working: %v", err)
	}
}

func TestChangePasswordIsAuditedWithoutPasswordMaterial(t *testing.T) {
	h := newHarness(t)
	auth := h.register(shopEmail, shopPhone)
	principal, err := h.service.Authenticate(context.Background(), auth.Session.Token)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.service.ChangePassword(context.Background(), principal, goodPassword, betterNew); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	entries := h.store.auditEntries()
	if len(entries) != 1 {
		t.Fatalf("audit entries = %d, want exactly one for the password change", len(entries))
	}
	e := entries[0]
	if e.Action != "identity.account.update" || e.ResourceType != "user" {
		t.Errorf("audit action/resource = %q/%q", e.Action, e.ResourceType)
	}
	if e.ActorID == nil || *e.ActorID != principal.UserID() {
		t.Errorf("audit actor = %v, want the account itself", e.ActorID)
	}
	dumped := fmtEntry(e)
	for _, secret := range []string{goodPassword, betterNew, fakeSum(goodPassword), fakeSum(betterNew)} {
		if strings.Contains(dumped, secret) {
			t.Errorf("the audit record contains password material %q", secret)
		}
	}
}

// T-10 in its strongest form: if the audit entry cannot be written, the change does not happen.
// An audit trail that can be lost while the change succeeds is not an audit trail.
func TestChangePasswordRollsBackWhenTheAuditFails(t *testing.T) {
	h := newHarness(t)
	auth := h.register(shopEmail, shopPhone)
	principal, err := h.service.Authenticate(context.Background(), auth.Session.Token)
	if err != nil {
		t.Fatal(err)
	}
	elsewhere, err := h.service.Login(context.Background(), identity.LoginInput{Email: shopEmail, Password: goodPassword})
	if err != nil {
		t.Fatal(err)
	}

	h.store.failAudit = true
	err = h.service.ChangePassword(context.Background(), principal, goodPassword, betterNew)
	if err == nil {
		t.Fatal("the password changed even though the audit write failed")
	}
	if user := h.store.user(principal.UserID()); user.PasswordHash == fakeSum(betterNew) {
		t.Error("the new password survived a rolled-back transaction")
	}
	if _, err := h.service.Authenticate(context.Background(), elsewhere.Session.Token); err != nil {
		t.Errorf("the account's other session was revoked by a change that did not commit: %v", err)
	}
}

func TestNewServiceRefusesAnUnwiredDependencyOrAnUnusablePolicy(t *testing.T) {
	h := newHarness(t)
	base := identity.Deps{
		Users: h.store, Sessions: h.store, Grants: h.store, Audit: h.store, Tx: h.store,
		Hasher: h.hasher, Policy: h.policy,
		Login: identity.LoginPolicy{MaxFailedAttempts: 5, Lockout: time.Minute},
	}

	for _, missing := range []string{"Users", "Sessions", "Grants", "Audit", "Tx", "Hasher"} {
		d := base
		switch missing {
		case "Users":
			d.Users = nil
		case "Sessions":
			d.Sessions = nil
		case "Grants":
			d.Grants = nil
		case "Audit":
			d.Audit = nil
		case "Tx":
			d.Tx = nil
		case "Hasher":
			d.Hasher = nil
		}
		if _, err := identity.NewService(d); !errors.Is(err, identity.ErrMissingDependency) {
			t.Errorf("NewService without %s = %v, want ErrMissingDependency", missing, err)
		}
	}

	d := base
	d.Policy = identity.Policy{Idle: time.Hour, AdminIdle: 0, Absolute: 24 * time.Hour}
	if _, err := identity.NewService(d); err == nil {
		t.Error("NewService accepted a policy with no admin idle window")
	}
	d = base
	d.Policy.Absolute = time.Minute
	if _, err := identity.NewService(d); err == nil {
		t.Error("NewService accepted an absolute cap shorter than the idle window")
	}
	d = base
	d.Login = identity.LoginPolicy{MaxFailedAttempts: 0, Lockout: time.Minute}
	if _, err := identity.NewService(d); err == nil {
		t.Error("NewService accepted a login policy that can never lock anything")
	}
}
