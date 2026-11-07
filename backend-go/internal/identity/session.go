package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

// ADR-019 replaces stateless JWTs with server-side sessions whose revocation is immediate. The
// consequences in this file are all traceable to that: the cookie carries an opaque random token
// and nothing else, the row behind it is read on every request, and expiry slides but is capped
// so that "immediately revocable" does not mean "lives until someone notices".
const (
	// tokenBytes is 256 bits of entropy: the token is the only thing standing between a stolen
	// request and an account, and it is high-entropy precisely so it can be stored hashed.
	tokenBytes = 32

	// Header names and the CSRF header the contract requires on every unsafe method.
	CSRFHeaderName = "X-CSRF-Token"
)

// RevokeReason is a closed set because identity.sessions CHECK-constrains the column. Accepting
// an arbitrary string here would turn an authorisation decision into a database error at
// runtime, which is a worse way to discover a typo than a compile-time one.
type RevokeReason string

const (
	RevokedLogout          RevokeReason = "logout"
	RevokedPasswordChanged RevokeReason = "password_changed"
	RevokedSuspended       RevokeReason = "suspended"
	RevokedRoleRevoked     RevokeReason = "role_revoked"
	RevokedAdmin           RevokeReason = "admin_revoke"
	RevokedExpired         RevokeReason = "expired"
)

var (
	ErrSessionNotFound = errors.New("no such session")
	ErrSessionRevoked  = errors.New("session revoked")
	ErrSessionExpired  = errors.New("session expired")

	ErrUnknownRevoke = errors.New("unknown session revocation reason")
)

// Policy is the session lifetime, and the one place the numbers live. Admin idle is shorter
// than customer idle by ADR-019: an operations console left open in a shared office is a
// different risk from a shopper's phone, and the doc says so.
type Policy struct {
	Idle      time.Duration
	AdminIdle time.Duration
	Absolute  time.Duration
}

// Validate refuses a policy that would make sessions either immortal or unusable, at startup
// rather than at the first login.
func (p Policy) Validate() error {
	switch {
	case p.Idle <= 0:
		return fmt.Errorf("session idle timeout must be positive, got %s", p.Idle)
	case p.AdminIdle <= 0:
		return fmt.Errorf("admin session idle timeout must be positive, got %s", p.AdminIdle)
	case p.Absolute < p.Idle:
		return fmt.Errorf("absolute session cap %s cannot be shorter than the idle timeout %s", p.Absolute, p.Idle)
	}
	return nil
}

// IdleFor picks the idle window for a principal. Roles, not account_kind, decide it: an admin
// who is also a customer is an admin when they are signed in as one.
func (p Policy) IdleFor(isAdmin bool) time.Duration {
	if isAdmin {
		return p.AdminIdle
	}
	return p.Idle
}

// Session is the row behind one cookie. Token and CSRFToken are populated only by NewSession:
// read back from the store, a Session carries their stored forms (Hash, and the plaintext CSRF
// token, which is deliberately not hashed — see csrf_token in migrations/sql/0003_identity.up.sql).
type Session struct {
	ID     int64
	UserID int64

	// Token is the cookie value. It exists between creation and the response and nowhere else:
	// the table holds Hash.
	Token string
	Hash  string

	CSRFToken string

	IP        string
	UserAgent string

	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time

	RevokedAt     *time.Time
	RevokedReason RevokeReason

	// ReplacedByID is rotation lineage: the session that superseded this one, so "which cookie
	// kicked this one out" is answerable during an incident.
	ReplacedByID *int64
}

// NewSession mints a session token and its CSRF token. The caller supplies the idle window,
// because it depends on the roles the user holds, which the service has just loaded.
func NewSession(userID int64, isAdmin bool, p Policy, now time.Time, ip, userAgent string) (Session, error) {
	token, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	csrf, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	idle := p.IdleFor(isAdmin)
	return Session{
		UserID:     userID,
		Token:      token,
		Hash:       HashToken(token),
		CSRFToken:  csrf,
		IP:         ip,
		UserAgent:  userAgent,
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  Expiry(now, now, idle, p.Absolute),
	}, nil
}

// Expiry is the sliding-with-cap rule: a session lives idle long after it was last seen, but
// never past Absolute from its creation. Taking the earlier of the two is the whole rule, and
// getting it wrong in either direction is a finding — a session that only slides is a
// permanent credential, one that only caps cannot be refreshed by an active user.
func Expiry(lastSeen, created time.Time, idle, absolute time.Duration) time.Time {
	sliding := lastSeen.Add(idle)
	capAt := created.Add(absolute)
	if capAt.Before(sliding) {
		return capAt
	}
	return sliding
}

// IsUsable is the per-request check. An expired or revoked row must fail here even though it is
// still in the table, because the table is the authority and nothing caches this answer.
func (s *Session) IsUsable(now time.Time) error {
	if s.RevokedAt != nil {
		return ErrSessionRevoked
	}
	if !now.Before(s.ExpiresAt) {
		return ErrSessionExpired
	}
	return nil
}

// Touch returns the values a request should write back: a new last-seen and a new expiry. It
// does not mutate, because a session read for authorisation must not look changed to the code
// that then rejects it.
func (s *Session) Touch(isAdmin bool, p Policy, now time.Time) (lastSeen, expires time.Time) {
	return now, Expiry(now, s.CreatedAt, p.IdleFor(isAdmin), p.Absolute)
}

func (s *Session) Revoke(reason RevokeReason, now time.Time) error {
	if err := reason.Validate(); err != nil {
		return err
	}
	s.RevokedAt = &now
	s.RevokedReason = reason
	return nil
}

// Validate refuses a reason the sessions table would reject. It belongs to the domain rather than
// to the adapter because the CHECK constraint in 0003 mirrors this set, and the write that fails
// is the same write either way — better to fail it before opening a transaction.
func (r RevokeReason) Validate() error {
	switch r {
	case RevokedLogout, RevokedPasswordChanged, RevokedSuspended,
		RevokedRoleRevoked, RevokedAdmin, RevokedExpired:
		return nil
	}
	return fmt.Errorf("%w: %q", ErrUnknownRevoke, r)
}

// HashToken is what the sessions table stores: SHA-256 of the cookie value, hex. A dump of this
// table must not hand over live credentials (ADR-019). The lookup is an equality test on this
// value and nothing reversible is stored beside it, which is why the row can be indexed and
// still be useless to anyone who reads it.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum)
}

func randomToken() (string, error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating session token: %w", err)
	}
	// URL-safe unpadded base64: cookie values and query strings must not need escaping, and a
	// credential in a query string is the thing ADR-019 forbids outright.
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
