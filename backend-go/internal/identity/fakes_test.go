package identity_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/audit"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
)

// The fakes are the proof that identity.Service depends on nothing but its ports: each one
// implements an exported interface of the package under test, so a port that grew a hidden
// requirement would stop compiling here rather than in production.
//
// The store also implements real rollback. A fake that ignored a failed write would let the
// service pass a test it should fail — "the change and its audit entry commit together" (ERD
// T-10) is only checked if dropping one drops the other.
type store struct {
	mu sync.Mutex

	users    map[int64]identity.User
	byEmail  map[string]int64
	sessions map[string]identity.Session

	// roles is the role catalogue: key → permissions. GrantsFor unions it with a user's
	// exceptional list, which is what makes B12 testable by editing one slice.
	roles       map[string][]string
	userRoles   map[int64][]string
	exceptional map[int64][]string

	auditLog []audit.Entry

	nextUser int64
	nextRow  int64

	txActive bool
	journal  []func()

	// One switch per write, so an atomicity test can break exactly one thing.
	failCreateUser     bool
	failCreateSession  bool
	failUpdatePassword bool
	failAudit          bool
}

func newStore() *store {
	return &store{
		users:       map[int64]identity.User{},
		byEmail:     map[string]int64{},
		sessions:    map[string]identity.Session{},
		roles:       map[string][]string{},
		userRoles:   map[int64][]string{},
		exceptional: map[int64][]string{},
	}
}

// write applies an undo closure only while a transaction is open; outside one, the store has
// already committed by the time the method returns.
func (s *store) write(undo func()) {
	if s.txActive {
		s.journal = append(s.journal, undo)
	}
}

func (s *store) RunInTx(ctx context.Context, work func(context.Context) error) error {
	s.mu.Lock()
	if s.txActive {
		// Nested: the same transaction, not a savepoint. Nothing in the service nests today, and
		// the pgx adapter behaves the same way, so the fake must not invent different semantics.
		s.mu.Unlock()
		err := work(ctx)
		s.mu.Lock()
		defer s.mu.Unlock()
		return err
	}
	s.txActive = true
	mark := len(s.journal)
	s.mu.Unlock()

	err := work(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.txActive = false
	if err != nil {
		for i := len(s.journal) - 1; i >= mark; i-- {
			s.journal[i]()
		}
	}
	s.journal = s.journal[:mark]
	return err
}

// ── UserStore ──────────────────────────────────────────────────────────────────────────────

func (s *store) CreateUser(_ context.Context, u *identity.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failCreateUser {
		return errors.New("fake: createUser failed")
	}
	if _, taken := s.byEmail[u.Email]; taken {
		return identity.ErrEmailTaken
	}
	for _, other := range s.users {
		if other.Phone != "" && other.Phone == u.Phone {
			return identity.ErrPhoneTaken
		}
	}
	s.nextUser++
	id := s.nextUser
	u.ID = id
	s.users[id] = *u
	s.byEmail[u.Email] = id
	s.write(func() {
		delete(s.users, id)
		delete(s.byEmail, u.Email)
		delete(s.userRoles, id)
	})
	return nil
}

func (s *store) UserByEmail(_ context.Context, email string) (identity.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.byEmail[strings.ToLower(email)]
	if !ok {
		return identity.User{}, identity.ErrNoUser
	}
	return s.users[id], nil
}

func (s *store) UserByID(_ context.Context, id int64) (identity.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return identity.User{}, identity.ErrNoUser
	}
	return u, nil
}

func (s *store) AssignRole(_ context.Context, userID int64, roleKey string, grantedBy *int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.roles[roleKey]; !ok {
		return fmt.Errorf("%w: %q", identity.ErrNoRole, roleKey)
	}
	if _, ok := s.users[userID]; !ok {
		return identity.ErrNoUser
	}
	if grantedBy != nil {
		if _, ok := s.users[*grantedBy]; !ok {
			return fmt.Errorf("fake: grantor %d does not exist", *grantedBy)
		}
	}
	before := len(s.userRoles[userID])
	s.userRoles[userID] = append(s.userRoles[userID], roleKey)
	s.write(func() {
		if before == 0 {
			delete(s.userRoles, userID)
			return
		}
		s.userRoles[userID] = s.userRoles[userID][:before]
	})
	return nil
}

func (s *store) UpdateLoginState(_ context.Context, u identity.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.users[u.ID]
	if !ok {
		return identity.ErrNoUser
	}
	before := stored
	stored.FailedLogins = u.FailedLogins
	stored.LockedUntil = u.LockedUntil
	stored.LastLoginAt = u.LastLoginAt
	s.users[u.ID] = stored
	s.write(func() { s.users[u.ID] = before })
	return nil
}

func (s *store) UpdatePassword(_ context.Context, userID int64, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failUpdatePassword {
		return errors.New("fake: updatePassword failed")
	}
	stored, ok := s.users[userID]
	if !ok {
		return identity.ErrNoUser
	}
	before := stored
	stored.PasswordHash = hash
	s.users[userID] = stored
	s.write(func() { s.users[userID] = before })
	return nil
}

// ── SessionStore ───────────────────────────────────────────────────────────────────────────

func (s *store) CreateSession(_ context.Context, sess *identity.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failCreateSession {
		return errors.New("fake: createSession failed")
	}
	s.nextRow++
	id := s.nextRow
	sess.ID = id
	s.sessions[sess.Hash] = *sess
	s.write(func() { delete(s.sessions, sess.Hash) })
	return nil
}

func (s *store) SessionByHash(_ context.Context, hash string) (identity.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[hash]
	if !ok {
		return identity.Session{}, identity.ErrSessionNotFound
	}
	return sess, nil
}

func (s *store) TouchSession(_ context.Context, id int64, lastSeen, expires time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for hash, sess := range s.sessions {
		if sess.ID == id {
			before := sess
			sess.LastSeenAt, sess.ExpiresAt = lastSeen, expires
			s.sessions[hash] = sess
			s.write(func() { s.sessions[hash] = before })
			return nil
		}
	}
	return identity.ErrSessionNotFound
}

func (s *store) RevokeSession(_ context.Context, id int64, reason identity.RevokeReason, at time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for hash, sess := range s.sessions {
		if sess.ID != id {
			continue
		}
		if sess.RevokedAt != nil {
			return false, nil
		}
		before := sess
		sess.RevokedAt, sess.RevokedReason = &at, reason
		s.sessions[hash] = sess
		s.write(func() { s.sessions[hash] = before })
		return true, nil
	}
	return false, identity.ErrSessionNotFound
}

func (s *store) RevokeAllExcept(_ context.Context, userID, keep int64, reason identity.RevokeReason, at time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int64
	for hash, sess := range s.sessions {
		if sess.UserID != userID || sess.ID == keep || sess.RevokedAt != nil {
			continue
		}
		before := sess
		sess.RevokedAt, sess.RevokedReason = &at, reason
		s.sessions[hash] = sess
		s.write(func() { s.sessions[hash] = before })
		count++
	}
	return count, nil
}

// ── GrantReader ────────────────────────────────────────────────────────────────────────────

func (s *store) GrantsFor(_ context.Context, userID int64) (identity.Grants, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	seen := map[string]bool{}
	for _, role := range s.userRoles[userID] {
		seen[role] = true
	}
	roles := make([]string, 0, len(seen))
	for role := range seen {
		roles = append(roles, role)
	}
	sort.Strings(roles)

	// The union the contract promises in Session.permissions: role grants first, then any
	// exceptional grant that has not expired. Expiry is the adapter's SQL condition, so the
	// fake models it by simply not listing an expired grant.
	var perms []string
	add := func(keys []string) {
		for _, k := range keys {
			if !contains(perms, k) {
				perms = append(perms, k)
			}
		}
	}
	for _, role := range roles {
		add(s.roles[role])
	}
	add(s.exceptional[userID])
	sort.Strings(perms)
	return identity.Grants{Roles: roles, Permissions: perms}, nil
}

func contains(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

// ── audit.Writer ───────────────────────────────────────────────────────────────────────────

func (s *store) Record(_ context.Context, e audit.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failAudit {
		return errors.New("fake: audit insert failed")
	}
	if err := e.Validate(); err != nil {
		return err
	}
	s.auditLog = append(s.auditLog, e)
	return nil
}

func (s *store) auditEntries() []audit.Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]audit.Entry(nil), s.auditLog...)
}

// ── the catalogue seeded for tests ─────────────────────────────────────────────────────────

const (
	customerPerms  = "shopping.cart.read,ordering.orders.create"
	adminTwoPerms  = "ordering.orders.read.all,ordering.orders.process"
	adminOnePerms  = "ordering.orders.status.override,payments.refund"
	superUserPerms = "identity.roles.manage,administration.system_settings.manage"
)

func (s *store) seedRoles() {
	s.roles["customer"] = split(customerPerms)
	s.roles["admin_2_operations"] = split(adminTwoPerms)
	s.roles["admin_1_business"] = split(adminOnePerms)
	s.roles["super_admin"] = split(superUserPerms)
}

func split(csv string) []string { return strings.Split(csv, ",") }

// ── Hasher ─────────────────────────────────────────────────────────────────────────────────

// fastHasher stands in for Argon2id so a unit test of login *rules* is not a test of 64 MiB of
// memory allocation. It is one-way, it distinguishes a mismatch from an unusable stored value,
// and it satisfies the same two-method contract the production adapter does (hasher.go, which
// the integration test exercises with the real thing).
type fastHasher struct {
	mu          sync.Mutex
	hashCalls   int
	verifyCalls int
}

func fakeSum(plaintext string) string {
	sum := sha256.Sum256([]byte("pepper:" + plaintext))
	return "fake$" + hex.EncodeToString(sum[:])
}

func (h *fastHasher) Hash(plaintext string) (string, error) {
	h.mu.Lock()
	h.hashCalls++
	h.mu.Unlock()
	return fakeSum(plaintext), nil
}

func (h *fastHasher) Verify(stored, plaintext string) error {
	h.mu.Lock()
	h.verifyCalls++
	h.mu.Unlock()
	if !strings.HasPrefix(stored, "fake$") {
		return errors.New("fake: stored value is not a hash this fake wrote")
	}
	if stored != fakeSum(plaintext) {
		return identity.ErrCredentialMismatch
	}
	return nil
}

func (h *fastHasher) calls() (hash, verify int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.hashCalls, h.verifyCalls
}

// ── wiring ─────────────────────────────────────────────────────────────────────────────────

type harness struct {
	t       *testing.T
	store   *store
	hasher  *fastHasher
	service *identity.Service
	now     time.Time
	policy  identity.Policy
	login   identity.LoginPolicy
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{t: t, store: newStore(), hasher: &fastHasher{}, now: time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)}
	h.store.seedRoles()
	h.policy = identity.Policy{Idle: 24 * time.Hour, AdminIdle: 2 * time.Hour, Absolute: 7 * 24 * time.Hour}
	h.login = identity.LoginPolicy{MaxFailedAttempts: 5, Lockout: 15 * time.Minute}

	svc, err := identity.NewService(identity.Deps{
		Users: h.store, Sessions: h.store, Grants: h.store, Audit: h.store, Tx: h.store,
		Hasher: h.hasher, Policy: h.policy, Login: h.login,
		Clock: func() time.Time { return h.now },
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	h.service = svc
	return h
}

// clockSteps forwards the fake clock, because "the next request" in B11 has to mean a later
// instant than the one that issued the session.
func (h *harness) advance(d time.Duration) {
	h.now = h.now.Add(d)
}

func (h *harness) register(email, phone string) identity.Authenticated {
	h.t.Helper()
	auth, err := h.service.Register(context.Background(), identity.RegisterInput{
		FullName: "Achieng Otieno", Email: email, Phone: phone,
		Password: "correct horse battery staple", IP: "196.189.1.10", UserAgent: "unittest",
	})
	if err != nil {
		h.t.Fatalf("Register(%s): %v", email, err)
	}
	return auth
}

// ── test setup helpers ────────────────────────────────────────────────────────────────────

func (s *store) user(id int64) identity.User {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.users[id]
}

func (s *store) mutate(id int64, f func(*identity.User)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.users[id]
	f(&u)
	s.users[id] = u
}

func (s *store) suspend(id int64)    { s.mutate(id, func(u *identity.User) { u.IsSuspended = true }) }
func (s *store) deactivate(id int64) { s.mutate(id, func(u *identity.User) { u.IsActive = false }) }

func (s *store) addExceptional(id int64, permission string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exceptional[id] = append(s.exceptional[id], permission)
}

func (s *store) removeRolePermission(roleKey, permission string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.roles[roleKey][:0:cap(s.roles[roleKey])]
	for _, p := range s.roles[roleKey] {
		if p != permission {
			kept = append(kept, p)
		}
	}
	s.roles[roleKey] = kept
}

func (s *store) countUsers() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.users)
}

func (s *store) liveSessions(userID int64) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, sess := range s.sessions {
		if sess.UserID == userID && sess.RevokedAt == nil {
			n++
		}
	}
	return n
}

// fmtEntry renders an audit entry the way the adapter would send it to JSONB, so a test can ask
// whether the record leaked something it should not contain.
func fmtEntry(e audit.Entry) string {
	body, err := json.Marshal(e)
	if err != nil {
		panic(err)
	}
	return string(body)
}

// withHasher rebuilds the service with a different Hasher over the same store, which is how a
// test asks what happens when the stored credential cannot be verified at all.
func (h *harness) withHasher(hasher identity.Hasher) *identity.Service {
	h.t.Helper()
	svc, err := identity.NewService(identity.Deps{
		Users: h.store, Sessions: h.store, Grants: h.store, Audit: h.store, Tx: h.store,
		Hasher: hasher, Policy: h.policy, Login: h.login,
		Clock: func() time.Time { return h.now },
	})
	if err != nil {
		h.t.Fatalf("NewService: %v", err)
	}
	return svc
}
