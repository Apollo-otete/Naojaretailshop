// Package postgres implements the identity ports against PostgreSQL — the only database in the
// target architecture (ADR-001).
//
// Nothing in here imports Gin, and nothing in here decides anything: the rules are in
// internal/identity, this file answers the questions those rules ask. That split is what §8
// means by a hexagonal adapter, and it is testable against a real server rather than against an
// opinion (see store_integration_test.go).
package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/database"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
)

type Store struct {
	pool *pgxpool.Pool
}

// New builds the adapter. A nil pool is refused here rather than at the first query because the
// alternative is a 500 in production with "invalid memory address" as the only evidence.
func New(pool *pgxpool.Pool) (*Store, error) {
	if pool == nil {
		return nil, database.ErrNoPool
	}
	return &Store{pool: pool}, nil
}

// Compile-time proof that one type satisfies three of the identity ports, so a signature change
// in the domain fails the build instead of surfacing as a missing capability at runtime.
var (
	_ identity.UserStore    = (*Store)(nil)
	_ identity.SessionStore = (*Store)(nil)
	_ identity.GrantReader  = (*Store)(nil)
	_ identity.Transactor   = (*Tx)(nil)
)

// Tx is the transaction port for this context. It is a one-method wrapper over
// database.TxRunner so the use case does not import the connection layer's vocabulary.
type Tx struct{ runner *database.TxRunner }

func NewTx(pool *pgxpool.Pool) *Tx { return &Tx{runner: database.NewTxRunner(pool)} }

func (t *Tx) RunInTx(ctx context.Context, work func(ctx context.Context) error) error {
	return t.runner.RunInTx(ctx, work)
}

const userColumns = `id, email, coalesce(phone, ''), full_name, account_kind, password_hash,
	is_active, is_suspended, failed_login_count, locked_until, email_verified_at,
	phone_verified_at, last_login_at, created_at, updated_at`

func (s *Store) exec(ctx context.Context) database.Execer { return database.DB(ctx, s.pool) }

// ── users ──────────────────────────────────────────────────────────────────────────────────

func (s *Store) CreateUser(ctx context.Context, u *identity.User) error {
	row := s.exec(ctx).QueryRow(ctx, `
		INSERT INTO identity.users
			(email, phone, full_name, password_hash, password_algo, account_kind, is_active, is_suspended)
		VALUES ($1, NULLIF($2, ''), $3, $4, 'argon2id', $5, $6, $7)
		RETURNING id, created_at, updated_at`,
		u.Email, u.Phone, u.FullName, u.PasswordHash, u.Account, u.IsActive, u.IsSuspended,
	)

	var created, updated time.Time
	if err := row.Scan(&u.ID, &created, &updated); err != nil {
		return wrapUnique(err, u.Email)
	}
	u.CreatedAt, u.UpdatedAt = created, updated
	return nil
}

func (s *Store) UserByEmail(ctx context.Context, email string) (identity.User, error) {
	return s.scanOne(ctx, `SELECT `+userColumns+` FROM identity.users WHERE email = $1`, email)
}

func (s *Store) UserByID(ctx context.Context, id int64) (identity.User, error) {
	return s.scanOne(ctx, `SELECT `+userColumns+` FROM identity.users WHERE id = $1`, id)
}

func (s *Store) scanOne(ctx context.Context, query string, arg any) (identity.User, error) {
	var (
		u                          identity.User
		lockedUntil, emailVerified *time.Time
		phoneVerified, lastLoginAt *time.Time
	)
	err := s.exec(ctx).QueryRow(ctx, query, arg).Scan(
		&u.ID, &u.Email, &u.Phone, &u.FullName, &u.Account, &u.PasswordHash,
		&u.IsActive, &u.IsSuspended, &u.FailedLogins, &lockedUntil, &emailVerified,
		&phoneVerified, &lastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.User{}, identity.ErrNoUser
	}
	if err != nil {
		return identity.User{}, fmt.Errorf("reading user: %w", err)
	}
	u.LockedUntil, u.EmailVerifiedAt = lockedUntil, emailVerified
	u.PhoneVerifiedAt, u.LastLoginAt = phoneVerified, lastLoginAt
	return u, nil
}

func (s *Store) UpdateLoginState(ctx context.Context, u identity.User) error {
	tag, err := s.exec(ctx).Exec(ctx, `
		UPDATE identity.users
		SET failed_login_count = $2, locked_until = $3, last_login_at = $4
		WHERE id = $1`,
		u.ID, u.FailedLogins, u.LockedUntil, u.LastLoginAt)
	return affected(err, tag, "identity.users", u.ID)
}

func (s *Store) UpdatePassword(ctx context.Context, userID int64, passwordHash string) error {
	tag, err := s.exec(ctx).Exec(ctx, `
		UPDATE identity.users SET password_hash = $2, password_algo = 'argon2id' WHERE id = $1`,
		userID, passwordHash)
	return affected(err, tag, "identity.users", userID)
}

func (s *Store) AssignRole(ctx context.Context, userID int64, roleKey string, grantedBy *int64) error {
	tag, err := s.exec(ctx).Exec(ctx, `
		INSERT INTO identity.user_roles (user_id, role_id, granted_by)
		SELECT $1, r.id, $2 FROM identity.roles r WHERE r.key = $3
		ON CONFLICT (user_id, role_id) DO NOTHING`,
		userID, grantedBy, roleKey)
	if err != nil {
		return fmt.Errorf("assigning role %q: %w", roleKey, err)
	}
	// Zero rows means either the role key does not exist or the user already held it. The
	// first is a bug in the caller and must not pass silently, so it is looked up rather than
	// guessed at from RowsAffected.
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := s.exec(ctx).QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM identity.roles WHERE key = $1)`, roleKey).Scan(&exists); err != nil {
			return fmt.Errorf("checking role %q: %w", roleKey, err)
		}
		if !exists {
			return fmt.Errorf("assigning %q: %w", roleKey, identity.ErrNoRole)
		}
	}
	return nil
}

// ── sessions ───────────────────────────────────────────────────────────────────────────────

func (s *Store) CreateSession(ctx context.Context, sess *identity.Session) error {
	row := s.exec(ctx).QueryRow(ctx, `
		INSERT INTO identity.sessions
			(session_hash, csrf_token, user_id, ip, user_agent, created_at, last_seen_at, expires_at)
		VALUES ($1, $2, $3, NULLIF($4, '')::inet, $5, $6, $6, $7)
		RETURNING id`,
		sess.Hash, sess.CSRFToken, sess.UserID, sess.IP, sess.UserAgent, sess.CreatedAt, sess.ExpiresAt)
	if err := row.Scan(&sess.ID); err != nil {
		return fmt.Errorf("creating session: %w", err)
	}
	sess.LastSeenAt = sess.CreatedAt
	return nil
}

func (s *Store) SessionByHash(ctx context.Context, sessionHash string) (identity.Session, error) {
	var (
		sess       identity.Session
		revokedAt  *time.Time
		reason     *string
		replacedBy *int64
		ipText     string
	)
	err := s.exec(ctx).QueryRow(ctx, `
		SELECT id, user_id, csrf_token, created_at, last_seen_at, expires_at,
		       revoked_at, revoked_reason, replaced_by_session_id,
		       coalesce(host(ip), ''), coalesce(user_agent, '')
		FROM identity.sessions
		WHERE session_hash = $1`, sessionHash).
		Scan(&sess.ID, &sess.UserID, &sess.CSRFToken, &sess.CreatedAt, &sess.LastSeenAt,
			&sess.ExpiresAt, &revokedAt, &reason, &replacedBy, &ipText, &sess.UserAgent)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.Session{}, identity.ErrSessionNotFound
	}
	if err != nil {
		return identity.Session{}, fmt.Errorf("reading session: %w", err)
	}
	// Hash is the value that was looked up; the caller already has it, and storing it keeps
	// Session self-describing for the transport's cookie handling.
	sess.Hash = sessionHash
	sess.IP = ipText
	sess.RevokedAt, sess.RevokedReason, sess.ReplacedByID = revokedAt, identity.RevokeReason(deref(reason)), replacedBy
	return sess, nil
}

func (s *Store) TouchSession(ctx context.Context, id int64, lastSeen, expires time.Time) error {
	_, err := s.exec(ctx).Exec(ctx, `
		UPDATE identity.sessions SET last_seen_at = $2, expires_at = $3 WHERE id = $1`,
		id, lastSeen, expires)
	if err != nil {
		return fmt.Errorf("touching session %d: %w", id, err)
	}
	return nil
}

func (s *Store) RevokeSession(ctx context.Context, id int64, reason identity.RevokeReason, at time.Time) (bool, error) {
	if err := reason.Validate(); err != nil {
		return false, err
	}
	tag, err := s.exec(ctx).Exec(ctx, `
		UPDATE identity.sessions
		SET revoked_at = $3, revoked_reason = $2
		WHERE id = $1 AND revoked_at IS NULL`,
		id, string(reason), at)
	if err != nil {
		return false, fmt.Errorf("revoking session %d: %w", id, err)
	}
	if tag.RowsAffected() > 0 {
		return true, nil
	}
	// Nothing was revoked. Whether that is "already revoked" (a repeated logout, which must
	// still succeed) or "no such session" is the difference the caller cannot see, so it is
	// looked up rather than assumed.
	var exists bool
	if err := s.exec(ctx).QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM identity.sessions WHERE id = $1)`, id).Scan(&exists); err != nil {
		return false, fmt.Errorf("checking session %d: %w", id, err)
	}
	if !exists {
		return false, identity.ErrSessionNotFound
	}
	return false, nil
}

func (s *Store) RevokeAllExcept(ctx context.Context, userID, keepSessionID int64, reason identity.RevokeReason, at time.Time) (int64, error) {
	if err := reason.Validate(); err != nil {
		return 0, err
	}
	tag, err := s.exec(ctx).Exec(ctx, `
		UPDATE identity.sessions
		SET revoked_at = $4, revoked_reason = $3
		WHERE user_id = $1 AND id <> $2 AND revoked_at IS NULL`,
		userID, keepSessionID, string(reason), at)
	if err != nil {
		return 0, fmt.Errorf("revoking the other sessions of user %d: %w", userID, err)
	}
	return tag.RowsAffected(), nil
}

// ── authority ──────────────────────────────────────────────────────────────────────────────

// GrantsFor is read per request and never cached in a session row, which is what makes B12 true:
// a permission removed from a role is gone from every member on the next authorization check.
func (s *Store) GrantsFor(ctx context.Context, userID int64) (identity.Grants, error) {
	roles, err := s.queryStrings(ctx, `
		SELECT r.key
		FROM identity.user_roles ur
		JOIN identity.roles r ON r.id = ur.role_id
		WHERE ur.user_id = $1
		ORDER BY r.key`, userID)
	if err != nil {
		return identity.Grants{}, fmt.Errorf("reading roles: %w", err)
	}

	// One query, two paths: the role grants and the exceptional grants that have neither expired
	// nor been revoked. Expiry is compared against the database clock so that a grant cannot be
	// live on one machine and expired on another.
	perms, err := s.queryStrings(ctx, `
		SELECT DISTINCT p.key
		FROM identity.permissions p
		WHERE p.id IN (
			SELECT rp.permission_id
			FROM identity.role_permissions rp
			JOIN identity.user_roles ur ON ur.role_id = rp.role_id
			WHERE ur.user_id = $1
		)
		OR p.id IN (
			SELECT up.permission_id
			FROM identity.user_permissions up
			WHERE up.user_id = $1 AND up.revoked_at IS NULL AND up.expires_at > now()
		)
		ORDER BY p.key`, userID)
	if err != nil {
		return identity.Grants{}, fmt.Errorf("reading permissions: %w", err)
	}
	return identity.Grants{Roles: roles, Permissions: perms}, nil
}

func (s *Store) queryStrings(ctx context.Context, query string, args ...any) ([]string, error) {
	rows, err := s.exec(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Sorted, because the effective permission list is returned to the client and an unstable
	// order shows up as a spurious diff in every session response.
	sort.Strings(out)
	return out, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// affected turns "no row changed" into an error naming the row: a use case that ignores it would
// report success for a write that matched nothing, which for a revocation is the worst possible
// answer to give an operator.
func affected(err error, tag pgconn.CommandTag, table string, id int64) error {
	if err != nil {
		return fmt.Errorf("updating %s %d: %w", table, id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s %d: %w", table, id, identity.ErrNoUser)
	}
	return nil
}

// wrapUnique reports the two 409s /auth/register documents and nothing else: a different
// constraint failing is a different problem, and claiming it is a taken email would send the
// reader looking in the wrong table.
//
// The names are `users_email_key`, not `identity_users_email_key`: Postgres generates a
// constraint name from the relation, not from the schema-qualified name. That is asserted against
// a real server in store_integration_test.go rather than assumed here, because a wrong name in
// this switch degrades into an opaque 500 for every duplicate registration.
func wrapUnique(err error, email string) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return fmt.Errorf("creating user: %w", err)
	}
	switch pgErr.ConstraintName {
	case "users_email_key":
		return identity.ErrEmailTaken
	case "users_phone_key":
		return identity.ErrPhoneTaken
	}
	return fmt.Errorf("creating user %s: unique violation on %s: %w", email, pgErr.ConstraintName, err)
}
