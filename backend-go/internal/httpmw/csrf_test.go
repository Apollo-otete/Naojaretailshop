package httpmw

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/testhelper"
)

// CSRF is one rule with two directions to get wrong: an unsafe method must not proceed without
// the header, and a safe one must not start demanding it. Both are tested here against the
// principal the authentication middleware is contractually obliged to install first, because the
// comparison is against the session's own value and not against anything the client asserts.

const csrfSessionToken = "s3cr3t-session-csrf-token"

var unsafeMethods = []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

type csrfCase struct {
	name    string
	method  string
	caller  *identity.Principal
	header  string // the value of X-CSRF-Token, or "" to send no header at all
	want    int
	reached bool
}

func TestCSRF(t *testing.T) {
	gin.SetMode(gin.TestMode)

	principal := &identity.Principal{CSRFToken: csrfSessionToken}
	unissued := &identity.Principal{CSRFToken: ""}

	cases := []csrfCase{
		{"unsafe with the right header", http.MethodPost, principal, csrfSessionToken, http.StatusNoContent, true},
		{"unsafe with no header", http.MethodPost, principal, "", http.StatusUnauthorized, false},
		{"unsafe with a blank header", http.MethodPost, principal, " ", http.StatusUnauthorized, false},
		{"unsafe with another session's header", http.MethodPut, principal, "a-different-token", http.StatusUnauthorized, false},
		{"unsafe on a session that was never issued a token", http.MethodDelete, unissued, "anything", http.StatusUnauthorized, false},

		{"get needs nothing", http.MethodGet, principal, "", http.StatusNoContent, true},
		{"head needs nothing", http.MethodHead, principal, "", http.StatusNoContent, true},
		// The preflight carries no cookies by design, so demanding the header there would break
		// every cross-site request in the frontend before it started.
		{"options is the cors preflight", http.MethodOptions, principal, "", http.StatusNoContent, true},

		// /auth/register and /auth/login: no session, so there is no ambient credential for a
		// forged page to attach. Rate limiting is their protection (ADR-019 pairs the two).
		{"public unsafe method", http.MethodPost, nil, "", http.StatusNoContent, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l, logs := testhelper.Logger(t, slog.LevelInfo)

			reached := false
			handler := func(c *gin.Context) {
				reached = true
				c.Status(http.StatusNoContent)
			}

			r := gin.New()
			r.Use(authenticate(c.caller))
			r.Use(CSRF(l))
			for _, method := range append([]string{http.MethodGet, http.MethodHead, http.MethodOptions}, unsafeMethods...) {
				r.Handle(method, "/orders/NJ-1", handler)
			}

			req := httptest.NewRequest(c.method, "/orders/NJ-1", nil)
			if c.header != "" {
				req.Header.Set(identity.CSRFHeaderName, c.header)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != c.want {
				t.Fatalf("%s = %d, want %d (%s)", c.method, rec.Code, c.want, rec.Body.String())
			}
			// A middleware that wrote the 401 and then let the handler run would produce a body
			// with two answers in it; the flag is how that is caught.
			if reached != c.reached {
				t.Errorf("handler reached = %t, want %t", reached, c.reached)
			}

			if c.want == http.StatusUnauthorized {
				if got := envelope(t, rec.Body.String()); got.Code != apierr.CodeNotAuthenticated {
					t.Errorf("code = %q, want %q — a CSRF failure is a security-scheme failure, not a permission decision",
						got.Code, apierr.CodeNotAuthenticated)
				}
				// The rejection has to be findable afterwards: a CSRF storm is a security event,
				// and the only witness is this line.
				if logs.Find(t, "request rejected") == nil {
					t.Error("the rejection was not logged")
				} else if got := logs.Field(t, "request rejected", "code"); got != string(apierr.CodeNotAuthenticated) {
					t.Errorf("logged code = %v, want %q", got, apierr.CodeNotAuthenticated)
				}
			}
		})
	}
}

// TestCSRFReadsTheSessionNotTheCaller is the point of the pattern: two callers on the same route,
// each with their own token, and neither may use the other's.
func TestCSRFReadsTheSessionNotTheCaller(t *testing.T) {
	gin.SetMode(gin.TestMode)

	l, _ := testhelper.Logger(t, slog.LevelInfo)
	for i, caller := range []identity.Principal{
		{CSRFToken: "token-of-caller-a"},
		{CSRFToken: "token-of-caller-b"},
	} {
		r := gin.New()
		r.Use(authenticate(&caller))
		r.Use(CSRF(l))
		r.POST("/orders/NJ-1/cancel", func(c *gin.Context) { c.Status(http.StatusNoContent) })

		for _, other := range []string{"token-of-caller-a", "token-of-caller-b"} {
			req := httptest.NewRequest(http.MethodPost, "/orders/NJ-1/cancel", nil)
			req.Header.Set(identity.CSRFHeaderName, other)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			want := http.StatusNoContent
			if other != caller.CSRFToken {
				want = http.StatusUnauthorized
			}
			if rec.Code != want {
				t.Errorf("caller %d with %q = %d, want %d", i, other, rec.Code, want)
			}
		}
	}
}

// TestCSRFRejectsASpoofedPrincipalFromTheHeader guards the ordering the middleware assumes: it
// reads the principal from the request context, which only the authentication middleware writes.
// A client that sends a header shaped like a token cannot become a caller.
func TestCSRFRejectsASpoofedPrincipalFromTheHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	l, _ := testhelper.Logger(t, slog.LevelInfo)
	reached := false
	r := gin.New()
	// No authenticate(): nothing in the chain installed a principal, so this is the anonymous
	// unsafe case, which CSRF must not turn into a session of its own.
	r.Use(CSRF(l))
	r.POST("/orders/NJ-1/cancel", func(c *gin.Context) {
		reached = true
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/orders/NJ-1/cancel", nil)
	req.Header.Set(identity.CSRFHeaderName, csrfSessionToken)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if !reached {
		t.Errorf("the anonymous unsafe request was refused (%d): /auth/login and /auth/register are unsafe methods with no session", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, csrfSessionToken) {
		t.Errorf("the supplied token came back in the body: %s", body)
	}
}

// authenticate stands in for the session middleware that runs ahead of CSRF: it puts a principal
// on the request context exactly as identity.WithPrincipal does in production.
func authenticate(p *identity.Principal) gin.HandlerFunc {
	return func(c *gin.Context) {
		if p != nil {
			ctx := identity.WithPrincipal(c.Request.Context(), *p)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	}
}
