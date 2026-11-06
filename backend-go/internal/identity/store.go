package identity

import (
	"context"
	"errors"
	"time"
)

// The ports below are the seams §8 asks for: the use cases in service.go depend on these and
// nothing else, so the rules can be tested without a database and the pgx adapter can be tested
// against one without an HTTP server. They are declared where they are used rather than in a
// separate package because every signature here is expressed in this context's types — a port
// that cannot be satisfied without importing the domain is not a port, it is a hint.
var (
	ErrNoUser = errors.New("no such user")

	// ErrNoRole names a role key that is not in the catalogue. Registration assigns `customer`,
	// so reaching this means the seed and the code disagree — a migration failure, not a user
	// error, and it must not be reported as one.
	ErrNoRole = errors.New("no such role")
	// ErrEmailTaken and ErrPhoneTaken are the two 409s /auth/register documents. They are
	// allowed to name the field that collided because registration itself discloses that an
	// address is in use; login deliberately does not (see ErrInvalidCredentials).
	ErrEmailTaken = errors.New("an account already uses this email")
	ErrPhoneTaken = errors.New("an account already uses this phone number")
)

// UserStore is the principals table.
type UserStore interface {
	// CreateUser fills in the new user's ID.
	CreateUser(ctx context.Context, u *User) error
	UserByEmail(ctx context.Context, email string) (User, error)
	UserByID(ctx context.Context, id int64) (User, error)

	// AssignRole attaches a role by key, and reports ErrNoRole when the key is not in the
	// catalogue. grantedBy is nil only for the system — self-service
	// registration assigning `customer` — because every human grant has to name who made it
	// (A-008: "exceptional direct grants must be individually justified and are always
	// audited", and the same attribution rule applies to roles).
	AssignRole(ctx context.Context, userID int64, roleKey string, grantedBy *int64) error

	// UpdateLoginState writes back the failure counter, the lock and last_login_at. It takes
	// the whole user rather than three columns because those three are derived together by
	// User.RegisterFailure / RegisterSuccess, and a caller that writes two of them can produce
	// an account that is locked and has zero failures at the same time.
	UpdateLoginState(ctx context.Context, u User) error

	UpdatePassword(ctx context.Context, userID int64, passwordHash string) error
}

// SessionStore is the cookie's server-side half.
type SessionStore interface {
	CreateSession(ctx context.Context, s *Session) error
	SessionByHash(ctx context.Context, sessionHash string) (Session, error)

	// TouchSession records the sliding window. It is the only write on the authorisation path,
	// and it must not be what decides anything: a request that fails authorization should still
	// look like activity from that session.
	TouchSession(ctx context.Context, id int64, lastSeen, expires time.Time) error

	// RevokeSession reports whether a row was actually revoked, so an idempotent logout can
	// still tell the difference between "revoked" and "already gone" in its logs without
	// telling the caller.
	RevokeSession(ctx context.Context, id int64, reason RevokeReason, at time.Time) (bool, error)

	// RevokeAllExcept is the ADR-019 consequence of a password change: every other session for
	// the account stops working immediately, and this one continues.
	RevokeAllExcept(ctx context.Context, userID, keepSessionID int64, reason RevokeReason, at time.Time) (int64, error)
}

// Grants is the answer to "what may this account do", already resolved: role grants unioned with
// exceptional grants that have not expired and have not been revoked.
//
// It is a value rather than two separate calls because the union is the thing that must not be
// gotten twice, and because this exact shape is what the RBAC evaluator caches (a short-lived
// entry that the grant tables invalidate on change — ADR-019).
type Grants struct {
	Roles       []string
	Permissions []string
}

// GrantReader reads the authority tables. B12 — removing a permission from a role removes it
// from every member without re-login — is a property of this being read per request rather than
// stored in the session, so a session row cannot carry a snapshot of it.
type GrantReader interface {
	GrantsFor(ctx context.Context, userID int64) (Grants, error)
}

// Transactor runs a unit of work in one database transaction. ERD T-10 requires that a
// privileged change and its audit entry commit together; the only honest way to guarantee that
// across two adapters is to hand both of them the same transaction through the context.
type Transactor interface {
	RunInTx(ctx context.Context, work func(ctx context.Context) error) error
}

// Hasher is password.Hash/Verify behind an interface. The production implementation is Argon2id
// at the parameters ADR-019 records; the interface exists so a unit test of the login rules does
// not spend 200 ms per case, not so a weaker function can be configured in production — see
// NewService, which refuses a Deps without a Hasher.
type Hasher interface {
	Hash(plaintext string) (string, error)
	Verify(stored, plaintext string) error
}
