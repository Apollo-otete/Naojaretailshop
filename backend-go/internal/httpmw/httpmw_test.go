package httpmw

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/authz"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/observability"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/testhelper"
)

func init() { gin.SetMode(gin.TestMode) }

// quiet returns a logger whose output is discarded, for tests that exercise behaviour other
// than log content.
func quiet(t *testing.T) *slog.Logger {
	t.Helper()
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	return l
}

type response struct {
	*httptest.ResponseRecorder
	body string
}

func run(t *testing.T, inbound map[string]string, mw []gin.HandlerFunc, routes func(*gin.Engine)) response {
	t.Helper()
	r := gin.New()
	r.Use(mw...)
	routes(r)

	req := httptest.NewRequest(http.MethodGet, "/orders/NJ-2026-417?phone=0712345678", nil)
	for k, v := range inbound {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return response{ResponseRecorder: rec, body: rec.Body.String()}
}

func get(path string, h gin.HandlerFunc) func(*gin.Engine) {
	return func(r *gin.Engine) { r.GET(path, h) }
}

func envelope(t *testing.T, body string) apierr.Envelope {
	t.Helper()
	var e apierr.Envelope
	if err := json.Unmarshal([]byte(body), &e); err != nil {
		t.Fatalf("body is not the Error envelope: %v\n%s", err, body)
	}
	return e
}

// ── Request ID ─────────────────────────────────────────────────────────────────────────────

func TestRequestIDIsGeneratedAndEchoed(t *testing.T) {
	resp := run(t, nil, []gin.HandlerFunc{RequestID(quiet(t), false)},
		get("/orders/:ref", func(c *gin.Context) { c.Status(http.StatusOK) }))

	id := resp.Header().Get("X-Request-Id")
	if len(id) != 36 {
		t.Fatalf("expected a UUID correlation id on the response, got %q", id)
	}
}

// An inbound id is honoured only with trust explicitly enabled — otherwise any client could
// forge the correlation id another customer's incident report quotes.
func TestInboundRequestIDIsIgnoredUnlessTrustIsEnabled(t *testing.T) {
	resp := run(t, map[string]string{"X-Request-Id": "attacker-chosen-id"},
		[]gin.HandlerFunc{RequestID(quiet(t), false)},
		get("/orders/:ref", func(c *gin.Context) { c.Status(http.StatusOK) }))

	if got := resp.Header().Get("X-Request-Id"); got == "attacker-chosen-id" {
		t.Fatal("an untrusted inbound id was adopted")
	}
}

func TestTrustedInboundRequestIDIsAdopted(t *testing.T) {
	l, buf := testhelper.Logger(t, slog.LevelDebug)
	resp := run(t, map[string]string{"X-Request-Id": "trace-123_abc"},
		[]gin.HandlerFunc{RequestID(l, true)},
		get("/orders/:ref", func(c *gin.Context) { c.Status(http.StatusOK) }))

	if got := resp.Header().Get("X-Request-Id"); got != "trace-123_abc" {
		t.Fatalf("X-Request-Id = %q, want the inherited trace id", got)
	}
	if rec := buf.Find(t, "request in"); rec["id_source"] != "inherited" {
		t.Errorf("id_source = %v, want inherited", rec["id_source"])
	}
}

// A caller-controlled string reaches both the logs and a response header. Without this filter,
// one header can forge extra log records or split an HTTP response.
func TestMalformedInboundRequestIDsAreReplaced(t *testing.T) {
	l := quiet(t)
	bad := []string{
		"has space",
		"line1\nline2 fake: record",
		"x\r\nX-Injected: yes",
		strings.Repeat("a", 65),
		`quote"and\backslash`,
		"<script>alert(1)</script>",
		"id/with/path",
	}
	for _, in := range bad {
		t.Run(in[:min(len(in), 14)], func(t *testing.T) {
			resp := run(t, map[string]string{"X-Request-Id": in},
				[]gin.HandlerFunc{RequestID(l, true)},
				get("/orders/:ref", func(c *gin.Context) { c.Status(http.StatusNoContent) }))

			got := resp.Header().Get("X-Request-Id")
			if got == in {
				t.Fatalf("malformed id %q was echoed back verbatim", in)
			}
			if !validRequestID(got) {
				t.Errorf("replacement %q is itself invalid", got)
			}
		})
	}
}

func TestRequestIDSitsOnTheContextForHandlersDownstream(t *testing.T) {
	var fromContext string
	resp := run(t, nil, []gin.HandlerFunc{RequestID(quiet(t), false)},
		get("/orders/:ref", func(c *gin.Context) {
			fromContext = observability.RequestID(c.Request.Context())
			c.Status(http.StatusNoContent)
		}))

	if fromContext == "" {
		t.Fatal("the handler could not read the request id from context")
	}
	if fromContext != resp.Header().Get("X-Request-Id") {
		t.Fatalf("context id %q != header id %q — a log line and a support ticket must agree",
			fromContext, resp.Header().Get("X-Request-Id"))
	}
}

// ── Access log ─────────────────────────────────────────────────────────────────────────────

// The route pattern, never the raw path: /orders/NJ-2026-417 puts a customer identifier into
// every log line, and the query string is where a phone number would land.
func TestAccessLogRecordsTheRoutePatternNotTheRawPath(t *testing.T) {
	l, buf := testhelper.Logger(t, slog.LevelInfo)
	run(t, nil, []gin.HandlerFunc{RequestID(l, false), Access(l)},
		get("/orders/:ref", func(c *gin.Context) { c.Status(http.StatusOK) }))

	line := buf.String()
	if !strings.Contains(line, `"route":"/orders/:ref"`) {
		t.Errorf("expected the matched pattern in: %s", line)
	}
	for _, leak := range []string{"NJ-2026-417", "0712345678", "phone="} {
		if strings.Contains(line, leak) {
			t.Errorf("access log leaked %q", leak)
		}
	}
}

func TestAccessLogCarriesTheCorrelationID(t *testing.T) {
	l, buf := testhelper.Logger(t, slog.LevelInfo)
	run(t, map[string]string{"X-Request-Id": "abc-1"}, []gin.HandlerFunc{RequestID(l, true), Access(l)},
		get("/orders/:ref", func(c *gin.Context) { c.Status(http.StatusTeapot) }))

	rec := buf.Find(t, "request")
	if rec == nil {
		t.Fatalf("no access record; got:\n%s", buf)
	}
	if rec["request_id"] != "abc-1" {
		t.Errorf("request_id = %v", rec["request_id"])
	}
	if rec["status"] != float64(http.StatusTeapot) {
		t.Errorf("status = %v, want the code actually written", rec["status"])
	}
}

// Health polling is the noisiest traffic in the system. It stays visible at debug instead of
// vanishing, so a broken probe can still be traced.
func TestProbesAreLoggedAtDebugAndDoNotFloodInfo(t *testing.T) {
	l, buf := testhelper.Logger(t, slog.LevelInfo)

	r := gin.New()
	r.Use(RequestID(l, false), Access(l))
	r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if strings.TrimSpace(buf.String()) != "" {
		t.Errorf("a probe wrote at Info level; the log becomes unreadable within an hour:\n%s", buf)
	}
}

func TestServerErrorsAreLoggedAsErrorsAndClientErrorsAsWarnings(t *testing.T) {
	cases := []struct {
		status  int
		wantLvl string
	}{
		{http.StatusOK, "INFO"},
		{http.StatusBadRequest, "WARN"},
		{http.StatusForbidden, "WARN"},
		{http.StatusInternalServerError, "ERROR"},
		{http.StatusServiceUnavailable, "ERROR"},
	}
	for _, tc := range cases {
		l, buf := testhelper.Logger(t, slog.LevelDebug)
		r := gin.New()
		r.Use(RequestID(l, false), Access(l))
		r.GET("/x", func(c *gin.Context) { c.Status(tc.status) })
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))

		rec := buf.Find(t, "request")
		if rec == nil {
			t.Fatalf("status %d produced no access record:\n%s", tc.status, buf)
		}
		if rec["level"] != tc.wantLvl {
			t.Errorf("status %d logged at %v, want %s", tc.status, rec["level"], tc.wantLvl)
		}
	}
}

// ── Recover ────────────────────────────────────────────────────────────────────────────────

func TestPanicBecomesTheContractEnvelopeNotATruncatedBody(t *testing.T) {
	l := quiet(t)
	resp := run(t, map[string]string{"X-Request-Id": "boom-1"},
		[]gin.HandlerFunc{Recover(l), RequestID(l, true), Access(l)},
		get("/orders/:ref", func(c *gin.Context) { panic("nil map write") }))

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", resp.Code)
	}
	e := envelope(t, resp.body)
	if e.Code != apierr.CodeInternal {
		t.Errorf("code = %q", e.Code)
	}
	// The whole point of the correlation id: the client saw nothing, but an operator can find
	// the stack from the id printed on their screen.
	if e.CorrelationID != "boom-1" {
		t.Errorf("correlation_id = %q, want the request id", e.CorrelationID)
	}
	if strings.Contains(resp.body, "goroutine") || strings.Contains(resp.body, "httpmw") {
		t.Errorf("a stack trace reached the client: %s", resp.body)
	}
}

func TestRecoveredPanicLogsTheStackAndThePath(t *testing.T) {
	l, buf := testhelper.Logger(t, slog.LevelDebug)

	r := gin.New()
	r.Use(RequestID(l, false), Recover(l))
	r.GET("/orders/:ref", func(c *gin.Context) { panic("nil map write") })
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/orders/NJ-9", nil))

	rec := buf.Find(t, "panic recovered")
	if rec == nil {
		t.Fatalf("no panic record:\n%s", buf)
	}
	for _, field := range []string{"stack", "path", "request_id"} {
		if _, ok := rec[field]; !ok {
			t.Errorf("panic record is missing %q: %v", field, rec)
		}
	}
	if !strings.Contains(asText(rec["stack"]), "goroutine") {
		t.Error("the stack field does not look like a stack")
	}
}

// A panic after the first byte cannot be answered with a 500 body — the status line is gone.
// The handler must still not take the process down with it.
func TestPanicAfterPartialWriteIsRecordedAndDoesNotKillTheProcess(t *testing.T) {
	l, buf := testhelper.Logger(t, slog.LevelDebug)

	r := gin.New()
	r.Use(RequestID(l, false), Recover(l))
	r.GET("/x", func(c *gin.Context) {
		c.Writer.WriteHeader(http.StatusPartialContent)
		_, _ = c.Writer.Write([]byte("first chunk"))
		panic("mid-stream")
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	if buf.Find(t, "panic recovered") == nil {
		t.Error("a mid-stream panic must still be recorded")
	}
	if !strings.HasPrefix(rec.Body.String(), "first chunk") {
		t.Errorf("unexpected body %q", rec.Body.String())
	}
}

// ── CORS ───────────────────────────────────────────────────────────────────────────────────

func TestAllowedOriginGetsCredentialedAccess(t *testing.T) {
	resp := run(t, map[string]string{"Origin": "http://localhost:5173"},
		[]gin.HandlerFunc{CORS(quiet(t), []string{"http://localhost:5173"})},
		get("/orders/:ref", func(c *gin.Context) { c.Status(http.StatusOK) }))

	h := resp.Header()
	if got := h.Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("allow-origin = %q", got)
	}
	if got := h.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("allow-credentials = %q — sessions are cookie-based, so this must be true", got)
	}
	// A cached preflight is per-origin. Without Vary, a shared cache serves the first origin's
	// answer to every origin behind it.
	if got := h.Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("vary = %q", got)
	}
	if got := h.Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Idempotency-Key") {
		t.Errorf("Idempotency-Key must be permitted for order placement, got %q", got)
	}
	if got := h.Get("Access-Control-Expose-Headers"); !strings.Contains(got, "X-Request-Id") {
		t.Errorf("the client cannot read its own correlation id unless it is exposed, got %q", got)
	}
}

func TestUnlistedOriginReceivesNoCORSGrant(t *testing.T) {
	l, buf := testhelper.Logger(t, slog.LevelDebug)
	resp := run(t, map[string]string{"Origin": "http://evil.example"},
		[]gin.HandlerFunc{CORS(l, []string{"http://localhost:5173"})},
		get("/orders/:ref", func(c *gin.Context) { c.Status(http.StatusOK) }))

	if got := resp.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("an unlisted origin was granted %q", got)
	}
	if got := resp.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("credentials granted to an unlisted origin: %q", got)
	}
	// The request still succeeds server-side: CORS is enforced by the browser, and pretending
	// otherwise hides the origin that needs adding to the allow-list.
	if resp.Code != http.StatusOK {
		t.Errorf("status = %d, want the request to be served; got %d", resp.Code, resp.Code)
	}
	if !strings.Contains(buf.String(), "blocked cross-origin request") {
		t.Error("a refused origin should be visible in the log")
	}
}

// A preflight must never reach a handler: an unauthenticated OPTIONS that triggers business
// logic is a real class of bug, and 204 is what browsers expect.
func TestPreflightStopsAtTheMiddleware(t *testing.T) {
	l := quiet(t)

	r := gin.New()
	r.Use(CORS(l, []string{"http://localhost:5173"}))
	reached := false
	r.OPTIONS("/orders", func(c *gin.Context) { reached = true; c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodOptions, "/orders", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if reached {
		t.Error("a preflight reached its handler")
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204", rec.Code)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	if len(body) != 0 {
		t.Errorf("a 204 must have no body, got %q", body)
	}
}

func TestRequestsWithoutAnOriginHeaderGetNoCORSHeaders(t *testing.T) {
	resp := run(t, nil, []gin.HandlerFunc{CORS(quiet(t), []string{"http://localhost:5173"})},
		get("/orders/:ref", func(c *gin.Context) { c.Status(http.StatusOK) }))

	if got := resp.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("no Origin was sent, so no CORS header should be added, got %q", got)
	}
	if resp.Code != http.StatusOK {
		t.Errorf("status = %d, want the request to proceed", resp.Code)
	}
}

// ── Authorize ──────────────────────────────────────────────────────────────────────────────

// Deny-by-default is only a claim until something proves it. Every protected route goes through
// here, so this is the test standing between an unfinished operation and a public endpoint —
// against the real evaluator rather than a stub, because a stub can be agreeable about nothing
// being authenticated.
func TestAuthorizeRefusesAnUnauthenticatedRequest(t *testing.T) {
	l := quiet(t)

	r := gin.New()
	r.Use(RequestID(l, false))
	reached := false
	r.GET("/orders/:ref", Authorize(l, authz.RBAC{Logger: l}, "ordering.orders.read.all"), func(c *gin.Context) {
		reached = true
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/orders/NJ-1", nil))

	if reached {
		t.Fatal("the handler ran without an authorizer saying yes")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s, want 401", rec.Code, rec.Body.String())
	}
	if e := envelope(t, rec.Body.String()); e.Code != apierr.CodeNotAuthenticated {
		t.Errorf("code = %q — nothing was decided about this caller's authority", e.Code)
	}
}

func TestAuthorizeRefusesASignedInCallerLackingThePermission(t *testing.T) {
	l := quiet(t)

	r := gin.New()
	r.Use(RequestID(l, false), authenticateAs(&identity.Principal{
		User:        identity.User{ID: 7},
		Permissions: []string{"ordering.orders.read"},
	}))
	reached := false
	r.GET("/orders/:ref", Authorize(l, authz.RBAC{Logger: l}, "ordering.orders.read.all"), func(c *gin.Context) {
		reached = true
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/orders/NJ-1", nil))

	if reached {
		t.Fatal("a caller without the declared permission reached its handler")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	e := envelope(t, rec.Body.String())
	if e.Code != apierr.CodePermissionDenied {
		t.Errorf("code = %q", e.Code)
	}
	// The client is told which permission is missing, which is what turns "the button is broken"
	// into a one-line support answer.
	if e.MissingPermission != "ordering.orders.read.all" {
		t.Errorf("missing_permission = %q", e.MissingPermission)
	}
}

func TestAuthorizeForwardsEveryDeclaredPermissionOfTheOperation(t *testing.T) {
	l := quiet(t)
	var seen port.Request

	r := gin.New()
	r.Use(RequestID(l, false))
	reached := false
	r.GET("/products", Authorize(l, record{got: &seen}, "catalogue.products.read", "catalogue.products.read.draft"),
		func(c *gin.Context) {
			reached = true
			c.Status(http.StatusOK)
		})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products", nil))

	if !reached {
		t.Fatal("a permitted request was blocked — the middleware is not calling Next")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
	if len(seen.Permissions) != 2 || seen.Permissions[1] != "catalogue.products.read.draft" {
		t.Errorf("the authorizer received %v, want both keys the route declared — forwarding only the "+
			"first would make the second key impossible to hold access through", seen.Permissions)
	}
}

// authenticateAs stands in for the session read in tests that are about what happens *after* a
// principal exists. It installs one directly, which is exactly what Authenticate does after it
// has checked a cookie.
func authenticateAs(p *identity.Principal) gin.HandlerFunc {
	return func(c *gin.Context) {
		if p != nil {
			c.Request = c.Request.WithContext(identity.WithPrincipal(c.Request.Context(), *p))
		}
		c.Next()
	}
}

type record struct{ got *port.Request }

func (r record) Authorize(_ context.Context, req port.Request) error {
	*r.got = req
	return nil
}

func asText(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}
