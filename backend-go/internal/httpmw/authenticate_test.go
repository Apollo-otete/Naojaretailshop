package httpmw

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpcookie"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/testhelper"
)

// Authenticate has one job with three outcomes, and each is a different kind of bug: a request
// that carries a live session must arrive with a principal, a request that carries a dead one
// must stay anonymous *and* keep working if the route is public, and a request whose session
// cannot be read at all must not be mistaken for either.

const testCookieName = "naoja_session"

type stubSessions struct {
	calls     int
	gotToken  string
	principal identity.Principal
	err       error
}

func (s *stubSessions) Authenticate(_ context.Context, token string) (identity.Principal, error) {
	s.calls++
	s.gotToken = token
	return s.principal, s.err
}

func authRoute(t *testing.T, sessions Sessions) (*gin.Engine, *bool) {
	t.Helper()
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	reached := new(bool)

	r := gin.New()
	r.GET("/account", Authenticate(l, sessions, httpcookie.Session{Name: testCookieName, Secure: true}),
		func(c *gin.Context) {
			*reached = true
			if _, ok := identity.PrincipalFrom(c.Request.Context()); !ok {
				c.Header("X-Anonymous", "yes")
			}
			c.Status(http.StatusNoContent)
		})
	return r, reached
}

func requestWithCookie(value string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	if value != "" {
		req.AddCookie(&http.Cookie{Name: testCookieName, Value: value})
	}
	return req
}

func TestNoCookieMeansAnonymousAndNothingIsLookedUp(t *testing.T) {
	sessions := &stubSessions{}
	r, reached := authRoute(t, sessions)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, requestWithCookie(""))

	if !*reached {
		t.Fatal("a request without a cookie was refused — public routes must render for visitors")
	}
	if sessions.calls != 0 {
		t.Errorf("the session store was consulted %d times for a request that carries no credential", sessions.calls)
	}
	if rec.Header().Get("X-Anonymous") != "yes" {
		t.Error("the request should have proceeded with no principal")
	}
	if len(rec.Header().Values("Set-Cookie")) != 0 {
		t.Errorf("nothing was wrong with this cookie, so nothing should have been written: %v", rec.Header().Values("Set-Cookie"))
	}
}

func TestALiveSessionBecomesAPrincipalOnTheContext(t *testing.T) {
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	sessions := &stubSessions{principal: identity.Principal{
		User:        identity.User{ID: 7},
		Permissions: []string{"identity.account.read"},
		CSRFToken:   "csrf-value",
	}}

	reached := new(bool)
	r := gin.New()
	r.GET("/account", Authenticate(l, sessions, httpcookie.Session{Name: testCookieName, Secure: true}),
		func(c *gin.Context) {
			*reached = true
			p, ok := identity.PrincipalFrom(c.Request.Context())
			if !ok {
				t.Error("no principal reached the handler")
				return
			}
			if p.UserID() != 7 || p.CSRFToken != "csrf-value" || !p.Has("identity.account.read") {
				t.Errorf("principal = %+v", p)
			}
			c.Status(http.StatusNoContent)
		})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, requestWithCookie("opaque-session-token"))

	if !*reached {
		t.Fatal("handler did not run")
	}
	// The raw cookie value is what the read is keyed by; hashing is the service's business, and a
	// middleware that hashed first would be duplicating a rule (badly) in the transport.
	if sessions.gotToken != "opaque-session-token" {
		t.Errorf("the session read received %q, want the cookie value verbatim", sessions.gotToken)
	}
}

// Every reason a cookie is worthless produces the same two effects: the request continues as
// anonymous, and the browser is told to stop sending the credential. Both matter — refusing here
// would break the public catalogue for anyone whose session simply aged out, and not clearing
// would leave them aged out forever.
func TestADeadSessionContinuesAnonymousAndClearsTheCookie(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"no such session", identity.ErrSessionNotFound},
		{"revoked", identity.ErrSessionRevoked},
		{"expired", identity.ErrSessionExpired},
		// B11: a suspension bites on this request, and the next one arrives without a cookie.
		{"suspended", identity.ErrSuspended},
		{"deactivated", identity.ErrInactive},
		{"wrapped", errWrappingIdentity},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sessions := &stubSessions{err: tc.err}
			r, reached := authRoute(t, sessions)

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, requestWithCookie("dead-token"))

			if !*reached {
				t.Fatal("a stale cookie turned a public route into an error")
			}
			if rec.Header().Get("X-Anonymous") != "yes" {
				t.Error("the principal of a dead session reached the handler")
			}

			set := rec.Header().Get("Set-Cookie")
			if !strings.HasPrefix(set, testCookieName+"=;") {
				t.Fatalf("Set-Cookie = %q, want the credential emptied", set)
			}
			for _, attr := range []string{"Max-Age=0", "Path=/", "HttpOnly", "SameSite=Lax", "Secure"} {
				if !strings.Contains(set, attr) {
					t.Errorf("the clearing cookie is missing %s, so the browser keeps the live one: %q", attr, set)
				}
			}
		})
	}
}

// A database that will not answer is neither "signed in" nor "not signed in". Continuing
// anonymous would let a public route succeed during an outage and a protected one answer 401 —
// which tells a customer to log in again when nothing is wrong with their credentials.
func TestASessionReadThatFailedIsAServerError(t *testing.T) {
	outage := errors.New("connection reset by peer")
	l, buf := testhelper.Logger(t, slog.LevelInfo)
	reached := new(bool)

	r := gin.New()
	r.Use(RequestID(l, false))
	r.GET("/account", Authenticate(l, &stubSessions{err: outage}, httpcookie.Session{Name: testCookieName}),
		func(c *gin.Context) { *reached = true })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, requestWithCookie("whatever"))

	if *reached {
		t.Fatal("a request whose authority could not be established reached its handler")
	}
	e := envelope(t, rec.Body.String())
	if rec.Code != http.StatusInternalServerError || e.Code != apierr.CodeInternal {
		t.Fatalf("status = %d code = %q, want 500 INTERNAL_ERROR", rec.Code, e.Code)
	}
	if strings.Contains(rec.Body.String(), "connection reset") {
		t.Error("the driver's words reached the client")
	}
	if buf.Find(t, "request failed") == nil {
		t.Errorf("the cause belongs in the log:\n%s", buf)
	}
}

// ADR-019's verification item, at the layer that would be tempted to accommodate it: a session
// credential in a query string is not a second way to authenticate, it is not a way at all.
func TestATokenInTheQueryStringIsNotACredential(t *testing.T) {
	sessions := &stubSessions{principal: identity.Principal{User: identity.User{ID: 7}}}
	l, _ := testhelper.Logger(t, slog.LevelDebug)

	reached := new(bool)
	r := gin.New()
	r.GET("/account", Authenticate(l, sessions, httpcookie.Session{Name: testCookieName}),
		func(c *gin.Context) { *reached = true })

	req := httptest.NewRequest(http.MethodGet, "/account?token=stolen&naoja_session=x", nil)
	req.AddCookie(&http.Cookie{Name: testCookieName, Value: "x"})
	r.ServeHTTP(httptest.NewRecorder(), req)

	// Only the cookie is read; the query value is never even looked at, so there is no code path
	// in which a link or a log line carries a live credential.
	if sessions.gotToken != "x" {
		t.Errorf("the session read was keyed by %q", sessions.gotToken)
	}
	if !*reached {
		t.Fatal("handler did not run")
	}
}

// A store failure that still *means* "revoked" must be treated as revoked: identity.Service's
// adapters wrap sentinels with %w, and an authorisation answer that depended on the exact error
// object would fail open the moment a message was added.
var errWrappingIdentity = fmt.Errorf("reading the session row: %w", identity.ErrSessionRevoked)
