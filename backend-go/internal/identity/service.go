package identity

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/audit"
)

// ErrLocked and the two account-state errors are internal distinctions. The contract's
// /auth/login documents only 401 and 423, and matrix C1's non-disclosure rule means "disabled
// account" cannot be answered with a message that confirms the address exists — so the transport
// collapses these into the same 401 while the log keeps the truth.
var (
	ErrLocked       = errors.New("account temporarily locked")
	ErrSuspended    = errors.New("account suspended")
	ErrInactive     = errors.New("account is not active")
	ErrSamePassword = errors.New("the new password must differ from the current one")

	// ErrCredentialMismatch is what the Argon2id adapter returns for a wrong password, as
	// opposed to a stored hash that cannot be verified at all. The difference decides whether
	// the failure counter moves, so it cannot be one error.
	ErrCredentialMismatch = errors.New("credential mismatch")

	ErrMissingDependency = errors.New("identity service is not fully wired")
)

// LoginPolicy is the per-account half of ADR-019's login limiting. The per-address half is
// middleware, and both are needed: one stops an attacker hammering a single account, the other
// stops the same attacker being moved between accounts for free — which is exactly what the
// legacy service's address-only limiter allowed.
type LoginPolicy struct {
	MaxFailedAttempts int
	Lockout           time.Duration
}

func (p LoginPolicy) Validate() error {
	switch {
	case p.MaxFailedAttempts < 1:
		return fmt.Errorf("LOGIN_MAX_FAILURES must be at least 1, got %d", p.MaxFailedAttempts)
	case p.Lockout <= 0:
		return fmt.Errorf("LOGIN_LOCKOUT must be positive, got %s", p.Lockout)
	}
	return nil
}

// BreachedPasswordChecker reports whether a candidate password is known-compromised.
// Optional; when nil, no breach check is performed (hook for HIBP/k-anonymity integration).
type BreachedPasswordChecker interface {
	IsBreached(ctx context.Context, password string) (bool, error)
}

// Deps is the service's whole world. A struct rather than eight positional arguments means
// adding a port cannot silently reorder two time.Duration values.
type Deps struct {
	Users    UserStore
	Sessions SessionStore
	Grants   GrantReader
	Audit    audit.Writer
	Tx       Transactor
	Hasher   Hasher
	Policy   Policy
	Login    LoginPolicy
	Clock    func() time.Time
	Logger   *slog.Logger

	BreachCheck BreachedPasswordChecker
}

// Service is the application layer of the Identity context: the operations the contract
// documents, expressed against the ports above. It imports no HTTP types, so every rule here is
// reachable from a test without a server — which is how Phase 3's "authorization tests pass" is
// satisfiable at all.
type Service struct {
	d Deps

	// dummyHash is verified when an unknown email is presented, so that "no such account" costs
	// the same as "wrong password". Without it, response time alone tells an attacker which
	// addresses are registered and the non-disclosing 401 message becomes a polite lie.
	dummyHash string
}

func NewService(d Deps) (*Service, error) {
	if d.Users == nil || d.Sessions == nil || d.Grants == nil || d.Audit == nil || d.Tx == nil || d.Hasher == nil {
		return nil, fmt.Errorf("%w: Users, Sessions, Grants, Audit, Tx and Hasher are all required", ErrMissingDependency)
	}
	if err := d.Policy.Validate(); err != nil {
		return nil, err
	}
	if err := d.Login.Validate(); err != nil {
		return nil, err
	}
	if d.Clock == nil {
		d.Clock = time.Now
	}
	if d.Logger == nil {
		// Silent by default rather than stdout by default: a service that logs without being
		// asked to cannot be silenced from the wiring layer, and this one logs user ids.
		d.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	s := &Service{d: d}
	hash, err := d.Hasher.Hash("this string is never a password; it exists to spend time")
	if err != nil {
		return nil, fmt.Errorf("preparing the unknown-account hash path: %w", err)
	}
	s.dummyHash = hash
	return s, nil
}

// Authenticated is what the transport needs after a successful Register or Login: the session
// carrying the cookie value — which exists only in this response and is never stored — and the
// principal to render as the Session body.
type Authenticated struct {
	Session   Session
	Principal Principal
}

type RegisterInput struct {
	FullName  string
	Email     string
	Phone     string
	Password  string
	IP        string
	UserAgent string
}

type LoginInput struct {
	Email     string
	Password  string
	IP        string
	UserAgent string
}

// CustomerRoleKey is the role every self-registered storefront account holds. Registration
// cannot grant anything else, which is the first line of B1 and B2: there is no input to this
// endpoint that produces an admin.
const CustomerRoleKey = "customer"

// Register creates a Customer account and signs it in (contract 201: account created *and*
// Set-Cookie). The account row, the customer role and the session are one transaction: a
// half-created account with no role is a support ticket, and one with a session but no role is
// worse.
func (s *Service) checkBreached(ctx context.Context, password string) error {
	if s.d.BreachCheck == nil {
		return nil
	}
	breached, err := s.d.BreachCheck.IsBreached(ctx, password)
	if err != nil {
		// Fail open on checker errors? Safer to fail closed for security-sensitive paths; but
		// without a configured reliable checker we must not block users due to infra issues.
		if s.d.Logger != nil {
			s.d.Logger.Warn("breached-password check failed; allowing by default", "error", err)
		}
		return nil
	}
	if breached {
		return ErrBreachedPassword
	}
	return nil
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (Authenticated, error) {
	if err := CheckPasswordPolicy(in.Password); err != nil {
		return Authenticated{}, err
	}
	if err := s.checkBreached(ctx, in.Password); err != nil {
		return Authenticated{}, err
	}
	hash, err := s.d.Hasher.Hash(in.Password)
	if err != nil {
		return Authenticated{}, err
	}
	user, err := NewCustomer(in.FullName, in.Email, in.Phone, hash)
	if err != nil {
		return Authenticated{}, err
	}

	now := s.d.Clock()
	var session Session
	err = s.d.Tx.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.d.Users.CreateUser(ctx, &user); err != nil {
			return err
		}
		if err := s.d.Users.AssignRole(ctx, user.ID, CustomerRoleKey, nil); err != nil {
			return fmt.Errorf("assigning the customer role: %w", err)
		}
		var err error
		if session, err = NewSession(user.ID, false, s.d.Policy, now, in.IP, in.UserAgent); err != nil {
			return err
		}
		return s.d.Sessions.CreateSession(ctx, &session)
	})
	if err != nil {
		return Authenticated{}, err
	}

	grants, err := s.d.Grants.GrantsFor(ctx, user.ID)
	if err != nil {
		return Authenticated{}, err
	}
	return s.authenticated(user, session, grants), nil
}

// Login is the only path to a session for every actor (§3.1's one-endpoint rule, which retires
// the hardcoded admin credential and the env-var backdoor recorded in the inventory).
func (s *Service) Login(ctx context.Context, in LoginInput) (Authenticated, error) {
	normalized, err := NormaliseEmail(in.Email)
	if err != nil {
		// Not ErrInvalidCredentials: a malformed address is a client bug, and answering it with
		// "email or password is incorrect" would be the one case where the non-disclosure rule
		// hides the truth from the person using their own keyboard.
		return Authenticated{}, err
	}

	user, err := s.d.Users.UserByEmail(ctx, normalized)
	switch {
	case errors.Is(err, ErrNoUser):
		_ = s.d.Hasher.Verify(s.dummyHash, in.Password)
		return Authenticated{}, ErrInvalidCredentials
	case err != nil:
		return Authenticated{}, err
	}

	now := s.d.Clock()
	if user.IsLocked(now) {
		return Authenticated{}, ErrLocked
	}
	if !user.MayAct() {
		return Authenticated{}, ErrInvalidCredentials
	}

	switch err := s.d.Hasher.Verify(user.PasswordHash, in.Password); {
	case errors.Is(err, ErrCredentialMismatch):
		return s.failedLogin(ctx, user, in.IP, now)
	case err != nil:
		// A stored hash that cannot be verified is a broken row, not a wrong guess: say so to
		// the log and to the client, and leave the failure counter alone.
		return Authenticated{}, fmt.Errorf("verifying the stored credential for %d: %w", user.ID, err)
	}

	grants, err := s.d.Grants.GrantsFor(ctx, user.ID)
	if err != nil {
		return Authenticated{}, err
	}
	session, err := NewSession(user.ID, user.Account == KindAdmin, s.d.Policy, now, in.IP, in.UserAgent)
	if err != nil {
		return Authenticated{}, err
	}

	// One transaction: a session must not exist for an account whose counter still says "five
	// wrong guesses", and a lost last_login_at is a lost audit answer.
	if err := s.d.Tx.RunInTx(ctx, func(ctx context.Context) error {
		user.RegisterSuccess(now)
		if err := s.d.Users.UpdateLoginState(ctx, user); err != nil {
			return err
		}
		return s.d.Sessions.CreateSession(ctx, &session)
	}); err != nil {
		return Authenticated{}, err
	}

	return s.authenticated(user, session, grants), nil
}

// Authenticate is the per-request read, and the reason revocation is immediate: nothing here is
// derived from a claim inside a token. The session row, the account row and the grant tables are
// each consulted again, so B11 and B12 hold without a cache-invalidation message.
func (s *Service) Authenticate(ctx context.Context, token string) (Principal, error) {
	if token == "" {
		return Principal{}, ErrSessionNotFound
	}
	session, err := s.d.Sessions.SessionByHash(ctx, HashToken(token))
	if err != nil {
		return Principal{}, err
	}

	now := s.d.Clock()
	if err := session.IsUsable(now); err != nil {
		return Principal{}, err
	}

	user, err := s.d.Users.UserByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, ErrNoUser) {
			// The account was deleted under a live session. Report an invalid session rather
			// than a missing user: the first is true from the caller's side of the cookie.
			return Principal{}, ErrSessionNotFound
		}
		return Principal{}, err
	}
	if !user.MayAct() {
		if user.IsSuspended {
			return Principal{}, ErrSuspended
		}
		return Principal{}, ErrInactive
	}

	// Activity is recorded before the grants are read, so a session used to probe a permission
	// it does not have still leaves a trace. A failed touch is logged and not fatal: it must not
	// be able to decide an authorization answer in either direction.
	lastSeen, expires := session.Touch(user.Account == KindAdmin, s.d.Policy, now)
	if err := s.d.Sessions.TouchSession(ctx, session.ID, lastSeen, expires); err != nil {
		s.d.Logger.Warn("could not record session activity",
			slog.Int64("session_id", session.ID), slog.String("error", err.Error()))
	}
	// Touch does not mutate, and here the row has just been written, so this copy now describes
	// the session as the store holds it. GET /auth/session reports expires_at from it; a
	// principal carrying the pre-request expiry would answer "when does this cookie stop working"
	// with a value the request itself just moved.
	session.ExpiresAt = expires

	grants, err := s.d.Grants.GrantsFor(ctx, user.ID)
	if err != nil {
		return Principal{}, err
	}
	return principalOf(user, session, grants), nil
}

// Logout revokes exactly the presenting session and succeeds either way. Contract: calling it
// with an already-revoked cookie must not report "not signed in", because that would leak
// session state to whoever no longer has it.
func (s *Service) Logout(ctx context.Context, p Principal) error {
	revoked, err := s.d.Sessions.RevokeSession(ctx, p.SessionID, RevokedLogout, s.d.Clock())
	if err != nil {
		return err
	}
	if revoked {
		s.d.Logger.Info("session revoked",
			slog.Int64("user_id", p.UserID()), slog.Int64("session_id", p.SessionID))
	}
	return nil
}

// ChangePassword is `[own]` by construction: the actor comes from the session, and there is no
// parameter through which another account's password can be reached. That is the hole the
// inventory records at auth.js:122, where an environment value worked as a permanent master key.
func (s *Service) ChangePassword(ctx context.Context, p Principal, current, next string) error {
	if err := CheckPasswordPolicy(next); err != nil {
		return err
	}
	if err := s.checkBreached(ctx, next); err != nil {
		return err
	}
	if current == next {
		return ErrSamePassword
	}

	// Re-read rather than trust the principal's copy: a password changed a moment ago must not
	// still be verifiable against a hash loaded before the change.
	user, err := s.d.Users.UserByID(ctx, p.UserID())
	if err != nil {
		return err
	}
	switch err := s.d.Hasher.Verify(user.PasswordHash, current); {
	case errors.Is(err, ErrCredentialMismatch):
		return ErrInvalidCredentials
	case err != nil:
		return fmt.Errorf("verifying the current credential for %d: %w", user.ID, err)
	}

	hash, err := s.d.Hasher.Hash(next)
	if err != nil {
		return err
	}
	now := s.d.Clock()
	userID := user.ID

	return s.d.Tx.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.d.Users.UpdatePassword(ctx, userID, hash); err != nil {
			return err
		}
		revoked, err := s.d.Sessions.RevokeAllExcept(ctx, userID, p.SessionID, RevokedPasswordChanged, now)
		if err != nil {
			return err
		}
		if revoked > 0 {
			s.d.Logger.Info("other sessions revoked by a password change",
				slog.Int64("user_id", userID), slog.Int64("sessions_revoked", revoked))
		}
		// previous_value and new_value stay empty on purpose: the contract requires the event
		// audited and forbids password material in the record, and there is nothing else here to
		// diff. The row itself — actor, action, resource, ip, correlation id — is the evidence.
		return s.d.Audit.Record(ctx, audit.ByUser(userID, "identity.account.update", "user", &userID))
	})
}

// failedLogin spends the counter and, when that counter trips the lock, records the event. The
// write and the answer have to agree, so they happen in one transaction.
func (s *Service) failedLogin(ctx context.Context, user User, ip string, now time.Time) (Authenticated, error) {
	wasLocked := user.IsLocked(now)
	user.RegisterFailure(now, s.d.Login.MaxFailedAttempts, s.d.Login.Lockout)
	justLocked := !wasLocked && user.IsLocked(now)

	err := s.d.Tx.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.d.Users.UpdateLoginState(ctx, user); err != nil {
			return err
		}
		if !justLocked {
			return nil
		}
		// A lockout is a security event, and the matrix describes administration.audit_logs.read
		// as the audit *and security event* log. The recorded values are the count and the lock
		// expiry — never the password that produced them.
		actor := user.ID
		e := audit.ByUser(actor, "identity.account.lockout", "user", &actor)
		e.IP = ip
		e.OccurredAt = now
		e.NewValue = map[string]any{"failed_login_count": user.FailedLogins, "locked_until": user.LockedUntil}
		return s.d.Audit.Record(ctx, e)
	})
	if err != nil {
		return Authenticated{}, err
	}
	if justLocked {
		s.d.Logger.Warn("account locked after repeated failures",
			slog.Int64("user_id", user.ID), slog.String("ip", ip))
	}
	return Authenticated{}, ErrInvalidCredentials
}

func (s *Service) authenticated(user User, session Session, grants Grants) Authenticated {
	return Authenticated{Session: session, Principal: principalOf(user, session, grants)}
}

// principal is the only place a Principal is assembled, and the reason it is one function is
// ADR-019's: the register-login path, the login path and the per-request read must not be able to
// disagree about what a caller holds. A divergence there is a session that authorises differently
// from the /auth/session body describing it.
func principalOf(user User, session Session, grants Grants) Principal {
	return Principal{
		User:        user,
		SessionID:   session.ID,
		ExpiresAt:   session.ExpiresAt,
		CSRFToken:   session.CSRFToken,
		Roles:       grants.Roles,
		Permissions: grants.Permissions,
		Conditions:  grants.Conditions,
	}
}
