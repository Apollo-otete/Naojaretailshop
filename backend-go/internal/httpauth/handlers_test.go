package httpauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/authz"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpcookie"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/testhelper"
)

// These tests hold the transport to contracts/openapi.yaml's /auth section: the status each
// operation may answer with, the body each status carries, and the three rules the section exists
// to enforce — that a password never comes back, that a failed login says nothing about whether the
// account exists, and that logging out twice is not an error.

const cookieName = "naoja_session"

type fakeIdentity struct {
	calls []string

	registerIn identity.RegisterInput
	register   identity.Authenticated
	registerEr error

	loginIn  identity.LoginInput
	login    identity.Authenticated
	loginEr  error
	logoutAs identity.Principal
	logoutEr error

	changeAs   identity.Principal
	changeCur  string
	changeNext string
	changeEr   error
}

func (f *fakeIdentity) call(name string) { f.calls = append(f.calls, name) }

func (f *fakeIdentity) Register(_ context.Context, in identity.RegisterInput) (identity.Authenticated, error) {
	f.call("register")
	f.registerIn = in
	return f.register, f.registerEr
}

func (f *fakeIdentity) Login(_ context.Context, in identity.LoginInput) (identity.Authenticated, error) {
	f.call("login")
	f.loginIn = in
	return f.login, f.loginEr
}

func (f *fakeIdentity) Logout(_ context.Context, p identity.Principal) error {
	f.call("logout")
	f.logoutAs = p
	return f.logoutEr
}

func (f *fakeIdentity) ChangePassword(_ context.Context, p identity.Principal, current, next string) error {
	f.call("change_password")
	f.changeAs, f.changeCur, f.changeNext = p, current, next
	return f.changeEr
}

func signedUp() identity.Authenticated {
	return identity.Authenticated{
		Session:   identity.Session{ID: 7, Token: "opaque-session-token", ExpiresAt: expiry()},
		Principal: withExpiry(customer()),
	}
}

func expiry() time.Time {
	return time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
}

func customer() identity.Principal {
	return identity.Principal{
		User: identity.User{
			ID:       42,
			FullName: "Achieng' Otieno",
			Email:    "achieng.example@mail.test",
			Phone:    "+254712345678",
			Account:  identity.KindCustomer,
		},
		SessionID: 7,
		CSRFToken: "csrf-for-this-session",
		Roles:     []string{"customer"},
		// The keys the seeded customer role actually holds (migrations/sql/0003), each of which
		// carries owner_is_caller, so this fixture is a caller the catalogue really describes.
		Permissions: []string{"identity.account.update", "ordering.orders.create", "shopping.cart.read"},
		Conditions: map[string][]string{
			"identity.account.update": {authz.ConditionOwnerIsCaller},
			"ordering.orders.create":  {authz.ConditionOwnerIsCaller},
			"shopping.cart.read":      {authz.ConditionOwnerIsCaller},
		},
	}
}

func withExpiry(p identity.Principal) identity.Principal {
	p.ExpiresAt = expiry()
	return p
}

// router mounts the routes under the same /api/v1 prefix the service uses, so a test that asks for
// a path asks for the path a client would. The optional middlewares stand in for the Authenticate
// step, which internal/httpmw tests on its own.
func router(t *testing.T, svc Service, extra ...gin.HandlerFunc) *gin.Engine {
	t.Helper()
	l, _ := testhelper.Logger(t, slog.LevelDebug)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(extra...)

	api := r.Group("/api/v1")
	Mount(api, Deps{
		Identity:   svc,
		Authorizer: authz.RBAC{Logger: l},
		Logger:     l,
		Cookie:     httpcookie.Session{Name: cookieName},
	})
	return r
}

func signIn(p identity.Principal) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(identity.WithPrincipal(c.Request.Context(), p))
		c.Next()
	}
}

func post(t *testing.T, r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// The session row records the user agent that opened it, and httptest's request has none, so
	// without this the audit answer "which browser stole this account" would be untestable.
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) NaojaTestHarness/1.0")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func get(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) apierr.Envelope {
	t.Helper()
	var e apierr.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("body is not the error envelope: %v — %s", err, rec.Body)
	}
	return e
}

func decodeSession(t *testing.T, rec *httptest.ResponseRecorder) sessionView {
	t.Helper()
	var s sessionView
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("body is not a Session: %v — %s", err, rec.Body)
	}
	return s
}

// cookieAttrs returns the Set-Cookie attributes as the browser will see them, keyed lowercase, with
// the name=value pair under "pair".
func cookieAttrs(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	set := rec.Header().Values("Set-Cookie")
	if len(set) != 1 {
		t.Fatalf("expected exactly one Set-Cookie, got %d: %v", len(set), set)
	}
	attrs := map[string]string{}
	for _, part := range strings.Split(set[0], ";") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, "=") {
			attrs[strings.ToLower(part)] = ""
			continue
		}
		name, value, _ := strings.Cut(part, "=")
		attrs[strings.ToLower(name)] = value
	}
	return attrs
}

const goodRegistration = `{"full_name":"Achieng' Otieno","email":"achieng.example@mail.test","phone":"0712345678","password":"correct horse battery staple"}`

func TestRegisterCreatesTheAccountIssuesTheCookieAndDescribesTheSession(t *testing.T) {
	svc := &fakeIdentity{register: signedUp()}
	rec := post(t, router(t, svc), "/api/v1/auth/register", goodRegistration)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, want 201: %s", rec.Code, rec.Body)
	}

	attrs := cookieAttrs(t, rec)
	if attrs[cookieName] != "opaque-session-token" {
		t.Errorf("cookie carries %q, want the session token", attrs[cookieName])
	}
	if _, ok := attrs["httponly"]; !ok {
		t.Error("the session cookie is readable by scripts: HttpOnly is absent")
	}
	if attrs["samesite"] != "Lax" || attrs["path"] != "/" {
		t.Errorf("cookie attributes %v, want SameSite=Lax and Path=/ as the contract fixes them", attrs)
	}
	// A browser-session cookie: no Max-Age, because the server is the authority on when a session
	// dies and a cookie that outlived its own row only produces a confusing 401.
	if _, ok := attrs["max-age"]; ok {
		t.Errorf("a new session cookie carries Max-Age %q, which outlives the server's say", attrs["max-age"])
	}
	// Contract 201 is "account created *and* signed in": the body is the Session the new cookie
	// already authenticates, not a receipt for the account.
	body := decodeSession(t, rec)
	if body.User.ID != 42 || body.CSRFToken == "" {
		t.Errorf("session body %+v", body)
	}

	if got := svc.registerIn.Phone; got != "0712345678" {
		t.Errorf("the use case got phone %q: normalising is the domain's job, not the transport's", got)
	}
	if svc.registerIn.IP == "" || svc.registerIn.UserAgent == "" {
		t.Errorf("the session row needs the client address and agent, got %q %q",
			svc.registerIn.IP, svc.registerIn.UserAgent)
	}
}

// The response body is where a password leaks. This is the assertion that says it cannot: the
// submitted value must not appear anywhere in what came back.
func TestThePasswordNeverComesBack(t *testing.T) {
	svc := &fakeIdentity{register: signedUp()}
	rec := post(t, router(t, svc), "/api/v1/auth/register", goodRegistration)

	if body := rec.Body.String(); strings.Contains(body, "correct horse battery staple") {
		t.Errorf("the response body echoes the password: %s", body)
	}
	if body := rec.Header().Get("Set-Cookie"); strings.Contains(body, "correct horse") {
		t.Errorf("the cookie carries the password: %s", body)
	}
}

// A signup form with four problems should be corrected in one attempt, which is what the
// contract's "every violation is listed in details, keyed by field" is for.
func TestRegisterAnswersEveryFieldProblemAtOnce(t *testing.T) {
	svc := &fakeIdentity{}
	rec := post(t, router(t, svc), "/api/v1/auth/register",
		`{"full_name":"A","email":"not-an-address","phone":"07 1234","password":"short"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422: %s", rec.Code, rec.Body)
	}
	if len(svc.calls) != 0 {
		t.Errorf("the use case ran despite an invalid request: %v", svc.calls)
	}

	e := decodeEnvelope(t, rec)
	if e.Code != apierr.CodeValidationFailed {
		t.Errorf("code %q, want VALIDATION_FAILED", e.Code)
	}
	fields := make([]string, 0, len(e.Details))
	for _, d := range e.Details {
		fields = append(fields, d.Field)
	}
	want := []string{"full_name", "email", "phone", "password"}
	slices.Sort(fields)
	slices.Sort(want)
	if !slices.Equal(fields, want) {
		t.Errorf("details cover %v, want every bad field %v: %+v", fields, want, e.Details)
	}
}

// A parse error is answered without quoting what failed to parse: the bytes being parsed are the
// caller's password, and a syntax error can repeat part of its input.
func TestRegisterRefusesAJunkBodyWithoutEchoingIt(t *testing.T) {
	rec := post(t, router(t, &fakeIdentity{}), "/api/v1/auth/register",
		`{"full_name":"Achieng","email":"a@mail.test","phone":"0712345678","password":"hunter2 secret"`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400: %s", rec.Code, rec.Body)
	}
	if e := decodeEnvelope(t, rec); e.Code != apierr.CodeBadRequest {
		t.Errorf("code %q, want BAD_REQUEST", e.Code)
	}
	if strings.Contains(rec.Body.String(), "hunter2") {
		t.Errorf("the refusal repeats the submitted secret: %s", rec.Body)
	}
}

func TestRegisterNamesTheFieldThatCollided(t *testing.T) {
	for _, tc := range []struct {
		taken  error
		field  string
		body   string
		inBody string
	}{
		{identity.ErrEmailTaken, "email", goodRegistration, "achieng"},
		{identity.ErrPhoneTaken, "phone", goodRegistration, "0712345678"},
	} {
		svc := &fakeIdentity{registerEr: tc.taken}
		rec := post(t, router(t, svc), "/api/v1/auth/register", tc.body)

		if rec.Code != http.StatusConflict {
			t.Fatalf("%v: status %d, want 409: %s", tc.taken, rec.Code, rec.Body)
		}
		e := decodeEnvelope(t, rec)
		if len(e.Details) != 1 || e.Details[0].Field != tc.field {
			t.Errorf("%v: details %+v, want the colliding field %q", tc.taken, e.Details, tc.field)
		}
	}
}

// Matrix C1, applied to login: "no such account" and "wrong password" and "your account is
// disabled" have to be indistinguishable from outside, or the endpoint is an account enumerator.
// Suspension is the case that tempts a friendly message into existence.
func TestLoginFailuresDoNotDiscloseTheAccountState(t *testing.T) {
	filler := `{"email":"achieng.example@mail.test","password":"wrong but plausible"}`

	var reference *httptest.ResponseRecorder
	for _, refused := range []error{
		identity.ErrInvalidCredentials,
		identity.ErrSuspended,
		identity.ErrInactive,
	} {
		svc := &fakeIdentity{loginEr: refused}
		rec := post(t, router(t, svc), "/api/v1/auth/login", filler)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%v: status %d, want 401: %s", refused, rec.Code, rec.Body)
		}
		if e := decodeEnvelope(t, rec); e.Code != apierr.CodeNotAuthenticated {
			t.Errorf("%v: code %q, want NOT_AUTHENTICATED", refused, e.Code)
		}
		if reference == nil {
			reference = rec
			continue
		}
		if rec.Body.String() != reference.Body.String() {
			t.Errorf("%v answers %q where the plain refusal says %q — the difference is the disclosure",
				refused, rec.Body, reference.Body)
		}
	}
}

func TestLoginIssuesTheSessionAndDescribesIt(t *testing.T) {
	svc := &fakeIdentity{login: signedUp()}
	rec := post(t, router(t, svc), "/api/v1/auth/login",
		`{"email":"ACHIENG.EXAMPLE@MAIL.TEST","password":"correct horse battery staple"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body)
	}
	if cookieAttrs(t, rec)[cookieName] != "opaque-session-token" {
		t.Errorf("no session cookie: %v", rec.Header())
	}
	// The use case normalises; the transport must not have tried, or the two would disagree about
	// which address was presented.
	if svc.loginIn.Email != "ACHIENG.EXAMPLE@MAIL.TEST" {
		t.Errorf("login was handed %q — lowercasing is the domain's rule", svc.loginIn.Email)
	}
}

// A lockout is not a wrong password, and a client cannot tell them apart from a 401: the front end
// has to stop retrying, and the person has to know why. 423 is the contract's answer.
func TestALockedAccountIsRefusedWithItsOwnStatus(t *testing.T) {
	svc := &fakeIdentity{loginEr: identity.ErrLocked}
	rec := post(t, router(t, svc), "/api/v1/auth/login",
		`{"email":"achieng.example@mail.test","password":"guess"}`)

	if rec.Code != http.StatusLocked {
		t.Fatalf("status %d, want 423: %s", rec.Code, rec.Body)
	}
	if e := decodeEnvelope(t, rec); e.Code != apierr.CodeAccountLocked {
		t.Errorf("code %q, want ACCOUNT_LOCKED", e.Code)
	}
}

// The one login failure that is disclosed is a malformed address, and the contract's own
// non-disclosure rule is about accounts, not about a typo the caller can see.
func TestAMalformedLoginAddressIsNamed(t *testing.T) {
	svc := &fakeIdentity{loginEr: identity.ErrInvalidEmail}
	rec := post(t, router(t, svc), "/api/v1/auth/login", `{"email":"nope","password":"whatever"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422: %s", rec.Code, rec.Body)
	}
	if e := decodeEnvelope(t, rec); len(e.Details) != 1 || e.Details[0].Field != "email" {
		t.Errorf("details %+v, want the field the caller mistyped", e.Details)
	}
}

// The contract documents 204 and 500 for logout and nothing else. A revoked or absent cookie is
// therefore not an error here: answering 401 would tell an attacker that the session was already
// gone, which is session state, and the whole point of idempotence is that the caller ends up
// signed out either way.
func TestLogoutWithADeadOrAbsentCookieStillSucceeds(t *testing.T) {
	svc := &fakeIdentity{}
	rec := post(t, router(t, svc), "/api/v1/auth/logout", "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d, want 204: %s", rec.Code, rec.Body)
	}
	if len(svc.calls) != 0 {
		t.Errorf("a request with no session reached the use case: %v", svc.calls)
	}
	if attrs := cookieAttrs(t, rec); attrs["max-age"] != "0" {
		t.Errorf("the cookie was not cleared: %v", attrs)
	}
}

func TestLogoutRevokesExactlyThePresentingSession(t *testing.T) {
	svc := &fakeIdentity{}
	other := customer()
	other.SessionID = 99

	rec := post(t, router(t, svc, signIn(other)), "/api/v1/auth/logout", "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d, want 204: %s", rec.Code, rec.Body)
	}
	if svc.logoutAs.SessionID != 99 {
		t.Errorf("logout was handed session %d, want the one that presented itself (99)",
			svc.logoutAs.SessionID)
	}
}

func TestSessionDescribesTheCallerWithoutAskingAnything(t *testing.T) {
	svc := &fakeIdentity{}
	rec := get(t, router(t, svc, signIn(withExpiry(customer()))), "/api/v1/auth/session")

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body)
	}
	if len(svc.calls) != 0 {
		t.Errorf("GET /auth/session called %v; the principal is already on the request", svc.calls)
	}

	// Asserted on the raw JSON, because these are the three things the frontend switches on and
	// json.Unmarshal into a struct would silently accept nulls where a client expects arrays.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"user", "permissions", "csrf_token"} {
		if string(raw[required]) == "" || string(raw[required]) == "null" {
			t.Errorf("Session.%s is missing or null: %s", required, rec.Body)
		}
	}
	if got := string(raw["permissions"]); !strings.HasPrefix(got, "[") {
		t.Errorf("permissions is %s, want an array", got)
	}

	body := decodeSession(t, rec)
	if body.CSRFToken != "csrf-for-this-session" {
		t.Errorf("csrf_token %q — the client cannot build X-CSRF-Token without it", body.CSRFToken)
	}
	if !body.ExpiresAt.Equal(expiry()) {
		t.Errorf("expires_at %s, want the session's own deadline", body.ExpiresAt)
	}
	// scoped_permissions is rendered from the conditions that were used to authorise this very
	// request, so a client can explain why an order is visible to one customer and not another.
	want := []string{"identity.account.update", "ordering.orders.create", "shopping.cart.read"}
	if !slices.Equal(body.Scoped, want) {
		t.Errorf("scoped_permissions %v, want %v sorted", body.Scoped, want)
	}
}

func TestAnEmptyGrantListIsAnEmptyArrayNotAnAbsentField(t *testing.T) {
	bare := identity.Principal{User: identity.User{ID: 1, Account: identity.KindCustomer}, CSRFToken: "c"}
	rec := get(t, router(t, &fakeIdentity{}, signIn(bare)), "/api/v1/auth/session")

	if !strings.Contains(rec.Body.String(), `"permissions":[]`) {
		t.Errorf("a customer with no grants should get an empty array: %s", rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"phone":null`) {
		t.Errorf("an absent phone should be null, the contract's [string, 'null']: %s", rec.Body)
	}
}

func TestSessionRefusesSomeoneWithNoSession(t *testing.T) {
	rec := get(t, router(t, &fakeIdentity{}), "/api/v1/auth/session")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401: %s", rec.Code, rec.Body)
	}
}

const goodChange = `{"current_password":"correct horse battery staple","new_password":"a longer new one"}`

func TestPasswordChangeIsRefusedWithoutThePermissionAndSaysWhich(t *testing.T) {
	noGrant := customer()
	noGrant.Permissions = []string{"shopping.cart.read"}
	svc := &fakeIdentity{}

	rec := post(t, router(t, svc, signIn(noGrant)), "/api/v1/auth/password/change", goodChange)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403: %s", rec.Code, rec.Body)
	}
	if e := decodeEnvelope(t, rec); e.MissingPermission != "identity.account.update" {
		t.Errorf("missing_permission %q, want the key the contract declares for this operation",
			e.MissingPermission)
	}
	if len(svc.calls) != 0 {
		t.Errorf("the use case ran for a refused caller: %v", svc.calls)
	}
}

// identity.account.update is narrowed by owner_is_caller (ADR-024). This is the case the
// own-account middleware exists for: the resource has no id in the request, because it is the
// caller, and the evaluation still has to happen through the evaluator rather than by assumption.
func TestPasswordChangeEvaluatesTheOwnershipConditionAgainstTheCaller(t *testing.T) {
	svc := &fakeIdentity{}
	rec := post(t, router(t, svc, signIn(customer())), "/api/v1/auth/password/change", goodChange)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d, want 204 — the caller owns the account it is signing in with: %s",
			rec.Code, rec.Body)
	}
	if svc.changeAs.SessionID != 7 || svc.changeCur != "correct horse battery staple" {
		t.Errorf("use case got principal %+v current %q", svc.changeAs, svc.changeCur)
	}
	if svc.changeNext != "a longer new one" {
		t.Errorf("the new password did not reach the use case: %q", svc.changeNext)
	}
	// This session survives a password change by design (RevokeAllExcept keeps it), so the cookie
	// must not be touched: clearing it would log out the person who just proved they own the
	// account.
	if rec.Header().Get("Set-Cookie") != "" {
		t.Errorf("a successful password change replaced the session cookie: %v", rec.Header().Values("Set-Cookie"))
	}
}

// A wrong current password is a 422, not the 401 this operation also documents: the contract's
// Unauthenticated tells the client to route to login, and interrupting a valid session because the
// user mistyped their *old* password is the wrong remedy. The response list for this operation
// includes 422, so the choice stays inside the contract.
func TestAWrongCurrentPasswordDoesNotEndTheSession(t *testing.T) {
	svc := &fakeIdentity{changeEr: identity.ErrInvalidCredentials}
	rec := post(t, router(t, svc, signIn(customer())), "/api/v1/auth/password/change", goodChange)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422: %s", rec.Code, rec.Body)
	}
	e := decodeEnvelope(t, rec)
	if len(e.Details) != 1 || e.Details[0].Field != "current_password" {
		t.Errorf("details %+v, want current_password named", e.Details)
	}
}

func TestAWeakNewPasswordIsRefusedOnTheRightField(t *testing.T) {
	svc := &fakeIdentity{changeEr: fmt.Errorf("%w: at least 10 characters", identity.ErrPasswordTooShort)}
	rec := post(t, router(t, svc, signIn(customer())), "/api/v1/auth/password/change", goodChange)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422: %s", rec.Code, rec.Body)
	}
	if e := decodeEnvelope(t, rec); len(e.Details) != 1 || e.Details[0].Field != "new_password" {
		t.Errorf("details %+v, want new_password named", e.Details)
	}
}

func TestReusingTheSamePasswordIsRefused(t *testing.T) {
	svc := &fakeIdentity{changeEr: identity.ErrSamePassword}
	rec := post(t, router(t, svc, signIn(customer())), "/api/v1/auth/password/change", goodChange)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422: %s", rec.Code, rec.Body)
	}
	if e := decodeEnvelope(t, rec); e.Details[0].Field != "new_password" {
		t.Errorf("details %+v, want new_password named", e.Details)
	}
}

// A store failure is not the caller's fault and must not be dressed as one. It renders as the 500
// every operation documents, with the cause kept out of the body.
func TestAStoreFailureIsAFiftyHundredAndSaysNothing(t *testing.T) {
	cause := errors.New("dial tcp: connection refused")
	rec := post(t, router(t, &fakeIdentity{loginEr: fmt.Errorf("reading the users row: %w", cause)}),
		"/api/v1/auth/login", `{"email":"achieng.example@mail.test","password":"guess"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500: %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "connection refused") {
		t.Errorf("the internal cause reached the client: %s", rec.Body)
	}
}

// The contract's paths, methods and response status codes are what the generated TypeScript is
// built from, so an endpoint added here that the contract does not describe — or one it describes
// that was never mounted — is a client that cannot be written.
func TestTheMountedOperationsAreExactlyTheContractedOnes(t *testing.T) {
	r := router(t, &fakeIdentity{})

	want := []string{
		"GET /api/v1/auth/session",
		"POST /api/v1/auth/login",
		"POST /api/v1/auth/logout",
		"POST /api/v1/auth/password/change",
		"POST /api/v1/auth/register",
	}
	got := make([]string, 0, len(r.Routes()))
	for _, rt := range r.Routes() {
		got = append(got, rt.Method+" "+rt.Path)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("mounted %v, want exactly the five /auth operations %v", got, want)
	}
}

// Nothing here may answer with a body that is not the contract's envelope. A client that parses
// errors switches on `code` and iterates `details`, so a refusal with an empty code or a null
// details array is a client crash on someone else's typo.
func TestEveryRefusalCarriesTheErrorEnvelope(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		body   string
		extra  gin.HandlerFunc
		svc    Service
	}{
		{"junk register", http.MethodPost, "/api/v1/auth/register", "not json at all", nil, &fakeIdentity{}},
		{"junk login body", http.MethodPost, "/api/v1/auth/login", `{"email":`, nil, &fakeIdentity{}},
		{"anonymous session read", http.MethodGet, "/api/v1/auth/session", "", nil, &fakeIdentity{}},
		{"anonymous password change", http.MethodPost, "/api/v1/auth/password/change", "{}", nil, &fakeIdentity{}},
		{"unpermitted password change", http.MethodPost, "/api/v1/auth/password/change", "{}",
			signIn(identity.Principal{SessionID: 7, CSRFToken: "csrf-for-this-session"}), &fakeIdentity{}},
		{"failing use case", http.MethodPost, "/api/v1/auth/login", `{"email":"a@mail.test","password":"b"}`,
			nil, &fakeIdentity{loginEr: errors.New("the pool is closed")}},
	}

	for _, tc := range cases {
		var extra []gin.HandlerFunc
		if tc.extra != nil {
			extra = append(extra, tc.extra)
		}
		r := router(t, tc.svc, extra...)

		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code < 400 {
			t.Errorf("%s: expected a refusal, got %d — the case no longer tests anything", tc.name, rec.Code)
			continue
		}
		var e apierr.Envelope
		if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
			t.Errorf("%s: %d answered without an envelope: %v — %s", tc.name, rec.Code, err, rec.Body)
			continue
		}
		if e.Code == "" {
			t.Errorf("%s: refusal has no code to switch on: %s", tc.name, rec.Body)
		}
		if !strings.Contains(rec.Body.String(), `"details":`) {
			t.Errorf("%s: refusal omits details, which a client iterates: %s", tc.name, rec.Body)
		}
	}
}

// A 204 must be empty: gin will happily write a body if a handler does, and a client that reads
// `response.json()` after logout then throws on nothing.
func TestTheNoContentAnswersReallyCarryNoBody(t *testing.T) {
	for _, tc := range []struct {
		path string
		body string
		svc  Service
	}{
		{"/api/v1/auth/logout", "", &fakeIdentity{}},
		{"/api/v1/auth/password/change", goodChange, &fakeIdentity{}},
	} {
		rec := post(t, router(t, tc.svc, signIn(customer())), tc.path, tc.body)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("%s: status %d, want 204: %s", tc.path, rec.Code, rec.Body)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("%s: 204 carried a body: %q", tc.path, rec.Body)
		}
	}
}
