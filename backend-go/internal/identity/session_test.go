package identity_test

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
)

var (
	hexRe   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	tokenRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

func TestNewSessionStoresOnlyAHashOfTheCookieValue(t *testing.T) {
	now := time.Now()
	s1, err := identity.NewSession(7, false, identity.Policy{Idle: time.Hour, AdminIdle: time.Minute, Absolute: 2 * time.Hour}, now, "196.189.1.1", "unittest")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	s2, err := identity.NewSession(7, false, identity.Policy{Idle: time.Hour, AdminIdle: time.Minute, Absolute: 2 * time.Hour}, now, "196.189.1.1", "unittest")
	if err != nil {
		t.Fatal(err)
	}

	// The token is the cookie: URL-safe, unpadded, and not the value the table stores.
	if !tokenRe.MatchString(s1.Token) || strings.ContainsAny(s1.Token, "=/+") {
		t.Errorf("token %q needs escaping in a cookie or a header", s1.Token)
	}
	if !hexRe.MatchString(s1.Hash) {
		t.Errorf("hash %q is not a fixed-width hex digest", s1.Hash)
	}
	if s1.Hash == s1.Token {
		t.Error("the stored session is the credential itself")
	}
	if s1.CSRFToken == "" || s1.CSRFToken == s1.Token {
		t.Error("the CSRF token is missing or is the session token under a different name")
	}
	if s1.Token == s2.Token || s1.CSRFToken == s2.CSRFToken {
		t.Error("two sessions minted at the same instant share a secret")
	}
	if got := identity.HashToken(s1.Token); got != s1.Hash {
		t.Errorf("HashToken(token) = %q, want the stored %q", got, s1.Hash)
	}
	if s1.UserID != 7 || s1.IP != "196.189.1.1" || s1.UserAgent != "unittest" {
		t.Errorf("session context not recorded: %+v", s1)
	}
}

func TestExpirySlidesButNeverPastTheCap(t *testing.T) {
	created := time.Now()
	idle := time.Hour
	absolute := 3 * time.Hour

	lastSeen := created.Add(2*time.Hour + 50*time.Minute)
	got := identity.Expiry(lastSeen, created, idle, absolute)
	if want := created.Add(absolute); !got.Equal(want) {
		t.Errorf("Expiry near the cap = %v, want the cap %v", got, want)
	}

	lastSeen = created.Add(30 * time.Minute)
	if got := identity.Expiry(lastSeen, created, idle, absolute); !got.Equal(lastSeen.Add(idle)) {
		t.Errorf("Expiry inside the cap = %v, want %v", got, lastSeen.Add(idle))
	}
}

func TestSessionIsUsableBoundaries(t *testing.T) {
	now := time.Now()
	s := identity.Session{CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Hour)}

	if err := s.IsUsable(now); err != nil {
		t.Errorf("a fresh session is unusable: %v", err)
	}
	if err := s.IsUsable(now.Add(time.Hour)); !errors.Is(err, identity.ErrSessionExpired) {
		t.Errorf("exactly at the expiry instant = %v, want ErrSessionExpired", err)
	}
	if err := s.IsUsable(now.Add(time.Hour - time.Millisecond)); err != nil {
		t.Errorf("one millisecond before expiry = %v, want usable", err)
	}

	at := now.Add(time.Minute)
	s.RevokedAt, s.RevokedReason = &at, identity.RevokedLogout
	if err := s.IsUsable(now.Add(time.Second)); !errors.Is(err, identity.ErrSessionRevoked) {
		t.Errorf("a revoked session inside its window = %v, want ErrSessionRevoked", err)
	}
}

// identity.sessions CHECK-constrains revoked_reason to six values. Discovering that at runtime
// would mean a revocation that did not happen — for a suspension, that is a revoked session that
// is still live.
func TestRevokeAcceptsOnlyTheDocumentedReasons(t *testing.T) {
	now := time.Now()
	for _, reason := range []identity.RevokeReason{
		identity.RevokedLogout, identity.RevokedPasswordChanged, identity.RevokedSuspended,
		identity.RevokedRoleRevoked, identity.RevokedAdmin, identity.RevokedExpired,
	} {
		s := identity.Session{}
		if err := s.Revoke(reason, now); err != nil {
			t.Errorf("Revoke(%q): %v", reason, err)
		}
		if s.RevokedReason != reason || s.RevokedAt == nil {
			t.Errorf("Revoke(%q) left %+v", reason, s)
		}
	}
	s := identity.Session{}
	if err := s.Revoke("because_i_said_so", now); !errors.Is(err, identity.ErrUnknownRevoke) {
		t.Errorf("Revoke with an invented reason = %v, want ErrUnknownRevoke", err)
	}
	if s.RevokedAt != nil {
		t.Error("a rejected Revoke still marked the session revoked")
	}
}

func TestPolicyValidate(t *testing.T) {
	cases := []struct {
		name   string
		policy identity.Policy
		ok     bool
	}{
		{"sane", identity.Policy{Idle: 24 * time.Hour, AdminIdle: time.Hour, Absolute: 7 * 24 * time.Hour}, true},
		{"no idle", identity.Policy{Idle: 0, AdminIdle: time.Hour, Absolute: 7 * 24 * time.Hour}, false},
		{"admin idle unset", identity.Policy{Idle: time.Hour, AdminIdle: 0, Absolute: 7 * 24 * time.Hour}, false},
		{"admin longer than customer", identity.Policy{Idle: time.Hour, AdminIdle: 2 * time.Hour, Absolute: 7 * 24 * time.Hour}, true},
		{"cap below idle", identity.Policy{Idle: 24 * time.Hour, AdminIdle: time.Hour, Absolute: time.Hour}, false},
		{"negative", identity.Policy{Idle: -time.Hour, AdminIdle: time.Hour, Absolute: 7 * 24 * time.Hour}, false},
	}
	for _, c := range cases {
		err := c.policy.Validate()
		if c.ok && err != nil {
			t.Errorf("%s: Validate = %v", c.name, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: Validate accepted %v", c.name, c.policy)
		}
	}
}

func TestIdleForUsesTheAdminWindow(t *testing.T) {
	p := identity.Policy{Idle: 24 * time.Hour, AdminIdle: time.Hour, Absolute: 7 * 24 * time.Hour}
	if got := p.IdleFor(true); got != time.Hour {
		t.Errorf("IdleFor(admin) = %s, want the admin window", got)
	}
	if got := p.IdleFor(false); got != 24*time.Hour {
		t.Errorf("IdleFor(customer) = %s, want the customer window", got)
	}
}

// Touch must not mutate: the caller may be about to *refuse* this session, and a session that
// looks refreshed to the code that just rejected it is how an expired cookie starts working.
func TestTouchDoesNotMutateTheSession(t *testing.T) {
	now := time.Now()
	s, err := identity.NewSession(1, false, identity.Policy{Idle: time.Hour, AdminIdle: time.Hour, Absolute: 4 * time.Hour}, now, "", "")
	if err != nil {
		t.Fatal(err)
	}
	before := s

	lastSeen, expires := s.Touch(false, identity.Policy{Idle: time.Hour, AdminIdle: time.Hour, Absolute: 4 * time.Hour}, now.Add(30*time.Minute))
	if s.CreatedAt != before.CreatedAt || s.ExpiresAt != before.ExpiresAt || s.LastSeenAt != before.LastSeenAt {
		t.Error("Touch mutated the session it was given")
	}
	if !lastSeen.Equal(now.Add(30 * time.Minute)) {
		t.Errorf("lastSeen = %v", lastSeen)
	}
	if !expires.Equal(now.Add(90 * time.Minute)) {
		t.Errorf("expires = %v, want 30 minutes slid onto the idle window", expires)
	}
}
