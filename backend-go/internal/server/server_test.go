package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/authz"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/config"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/observability"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/testhelper"
)

func init() { gin.SetMode(gin.TestMode) }

func testCfg(env string) config.Config {
	return config.Config{
		Env:               env,
		Address:           ":0",
		LogLevel:          slog.LevelDebug,
		Database:          config.Database{URL: "postgres://naoja:x@localhost:5433/naoja", PingBudget: time.Second},
		CORS:              config.CORS{AllowedOrigins: []string{"http://localhost:5173"}},
		SessionCookieName: "naoja_session",
	}
}

func get(t *testing.T, r *gin.Engine, path string, header http.Header) (*http.Response, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	resp := rec.Result()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp, string(body)
}

// ── Liveness ───────────────────────────────────────────────────────────────────────────────

// Liveness must never consult a dependency: an orchestrator restarts a process for a live
// failure, and restarting the API because the database is slow turns an outage into a stampede.
func TestHealthzAnswersWithoutTouchingAnyDependency(t *testing.T) {
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	probe := &countingChecker{name: "postgresql"}

	r := New(Deps{
		Logger:     l,
		Cfg:        testCfg("development"),
		Authorizer: authz.DenyAll{},
		Checkers:   []port.Checker{probe},
		Version:    "v0.2.0-test",
		StartedAt:  time.Now().Add(-90 * time.Second),
	})

	resp, body := get(t, r, "/healthz", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body=%s", resp.StatusCode, body)
	}
	if probe.calls.Load() != 0 {
		t.Fatalf("/healthz called %q %d times — it must be dependency-free", probe.name, probe.calls.Load())
	}

	var out struct {
		Status        string `json:"status"`
		Version       string `json:"version"`
		UptimeSeconds int64  `json:"uptime_seconds"`
	}
	decode(t, body, &out)
	if out.Status != "live" || out.Version != "v0.2.0-test" {
		t.Errorf("payload = %+v", out)
	}
	// The deploy artifact identity is what makes "which build is this?" answerable at 2am.
	if out.UptimeSeconds < 89 || out.UptimeSeconds > 95 {
		t.Errorf("uptime_seconds = %d, want ~90", out.UptimeSeconds)
	}
}

// ── Readiness ──────────────────────────────────────────────────────────────────────────────

type readyResponse struct {
	Status       string `json:"status"`
	Version      string `json:"version"`
	Dependencies []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Detail string `json:"detail"`
	} `json:"dependencies"`
}

func (rr readyResponse) byName(t *testing.T, name string) (string, string) {
	t.Helper()
	for _, d := range rr.Dependencies {
		if d.Name == name {
			return d.Status, d.Detail
		}
	}
	t.Fatalf("dependency %q was not reported; got %+v", name, rr.Dependencies)
	return "", ""
}

func TestReadyzPassesWhenEveryDependencyPasses(t *testing.T) {
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	r := New(Deps{Logger: l, Cfg: testCfg("development"), Authorizer: authz.DenyAll{},
		Checkers: []port.Checker{&fakeChecker{name: "postgresql"}, &fakeChecker{name: "daraja"}}, Version: "t"})

	resp, body := get(t, r, "/readyz", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body=%s", resp.StatusCode, body)
	}
	var out readyResponse
	decode(t, body, &out)
	if out.Status != "ready" || len(out.Dependencies) != 2 {
		t.Fatalf("payload = %+v", out)
	}
	for _, name := range []string{"postgresql", "daraja"} {
		if st, _ := out.byName(t, name); st != "ok" {
			t.Errorf("%s = %q", name, st)
		}
	}
}

// "unhealthy" tells an operator nothing. Which thing is down is the entire value of the
// endpoint, so every dependency is reported individually rather than folded into a boolean.
func TestReadyzNamesTheFailingDependencyAndAnswers503(t *testing.T) {
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	dbDown := &fakeChecker{name: "postgresql", err: errors.New("ping failed after 1s: driver: bad connection")}

	r := New(Deps{Logger: l, Cfg: testCfg("development"), Authorizer: authz.DenyAll{},
		Checkers: []port.Checker{dbDown, &fakeChecker{name: "smtp"}}, Version: "t"})

	resp, body := get(t, r, "/readyz", nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 — a pod that cannot serve must leave the load balancer", resp.StatusCode)
	}
	var out readyResponse
	decode(t, body, &out)
	if out.Status != "unready" {
		t.Errorf("status = %q", out.Status)
	}

	st, detail := out.byName(t, "postgresql")
	if st != "error" {
		t.Errorf("postgresql = %q", st)
	}
	if !strings.Contains(detail, "ping failed") {
		t.Errorf("development should report the cause so it can be fixed, got %q", detail)
	}
	// One dependency being down must not hide the state of the others.
	if st, _ := out.byName(t, "smtp"); st != "ok" {
		t.Errorf("smtp = %q, want ok", st)
	}
}

// A driver error can quote a host, a port and a username. That is a debugging aid internally
// and reconnaissance externally, so production reports the name and the verdict only.
func TestReadyzSuppressesFailureDetailInProduction(t *testing.T) {
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	secret := `dial tcp 10.20.0.7:5432: connect: connection refused (host "db-primary.internal" user "naoja_owner")`

	r := New(Deps{Logger: l, Cfg: testCfg("production"), Authorizer: authz.DenyAll{},
		Checkers: []port.Checker{&fakeChecker{name: "postgresql", err: errors.New(secret)}}, Version: "t"})

	resp, body := get(t, r, "/readyz", nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	for _, leak := range []string{"10.20.0.7", "naoja_owner", "db-primary", "connection refused"} {
		if strings.Contains(body, leak) {
			t.Errorf("production readiness leaked %q: %s", leak, body)
		}
	}
	var out readyResponse
	decode(t, body, &out)
	if st, _ := out.byName(t, "postgresql"); st != "error" {
		t.Errorf("the verdict must survive redaction: %q", st)
	}
}

// Readiness has a budget. Checked sequentially, five dependencies at three seconds each answer
// 503 seventeen seconds after the load balancer gave up on the first one.
func TestReadyzChecksDependenciesConcurrently(t *testing.T) {
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	slow := func() port.Checker { return &fakeChecker{name: "x", delay: 150 * time.Millisecond} }

	r := New(Deps{Logger: l, Cfg: testCfg("development"), Authorizer: authz.DenyAll{}, Version: "t",
		Checkers: []port.Checker{slow(), slow(), slow(), slow()}})

	start := time.Now()
	resp, _ := get(t, r, "/readyz", nil)
	elapsed := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if elapsed > 400*time.Millisecond {
		t.Errorf("four 150ms checks took %s — they are not running concurrently", elapsed)
	}
}

// A checker that cannot see the request context cannot bound its own work, so readiness would
// keep hanging a probe long after the orchestrator gave up on it.
func TestReadyzHandsTheRequestContextToEveryChecker(t *testing.T) {
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	sawID := &ctxChecker{}

	r := New(Deps{Logger: l, Cfg: testCfg("development"), Authorizer: authz.DenyAll{},
		Checkers: []port.Checker{sawID}, Version: "t"})

	resp, body := get(t, r, "/readyz", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body=%s", resp.StatusCode, body)
	}
	if sawID.requestID == "" {
		t.Fatal("the checker received no request id on its context")
	}
	if got := resp.Header.Get("X-Request-Id"); got != sawID.requestID {
		t.Errorf("checker saw %q but the client was told %q — a timeout log and a support ticket "+
			"would point at different requests", sawID.requestID, got)
	}
}

// ── Protected routes ───────────────────────────────────────────────────────────────────────

// Require is how every operation in Phase 4 onwards will be guarded. If the wiring is wrong
// here — a middleware list that forgets to abort — every endpoint is public.
func TestRequireRefusesThroughTheDenyAllAuthorizer(t *testing.T) {
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	d := Deps{Logger: l, Cfg: testCfg("development"), Authorizer: authz.DenyAll{}, Version: "t"}

	r := New(d)
	var reached atomic.Bool
	r.POST("/api/v1/products", Require(d, "catalogue.products.write"), func(c *gin.Context) {
		reached.Store(true)
	})

	resp, body := do(t, r, http.MethodPost, "/api/v1/products", nil)
	if reached.Load() {
		t.Fatal("a protected handler executed without authorisation")
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", resp.StatusCode, body)
	}
	if e := envelope(t, body); e.MissingPermission != "catalogue.products.write" {
		t.Errorf("missing_permission = %q", e.MissingPermission)
	}
}

func TestRequireNamesEveryPermissionTheOperationDeclared(t *testing.T) {
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	spy := &spyAuthorizer{}
	d := Deps{Logger: l, Cfg: testCfg("development"), Authorizer: spy, Version: "t"}

	r := New(d)
	// The contract declares either key for the all-vs-own pair; the middleware must forward the
	// whole list rather than only the first, or the second key can never grant access.
	r.GET("/api/v1/orders", Require(d, "ordering.orders.read.all", "ordering.orders.read.own"),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"reached": true}) })

	if _, body := do(t, r, http.MethodGet, "/api/v1/orders", nil); !strings.Contains(body, `"reached":true`) {
		t.Fatalf("handler did not run: %s", body)
	}
	if len(spy.last) != 2 || spy.last[1] != "ordering.orders.read.own" {
		t.Errorf("authorizer saw %v, want both declared keys", spy.last)
	}
}

// ── The chain as a whole ───────────────────────────────────────────────────────────────────

// Unmatched routes get the contract envelope. Gin's default is plain text "404 page not
// found", and a body that only exists for half the responses is how a frontend ends up guessing.
func TestUnknownRouteRendersTheErrorEnvelope(t *testing.T) {
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	r := New(Deps{Logger: l, Cfg: testCfg("development"), Authorizer: authz.DenyAll{}, Version: "t"})

	resp, body := do(t, r, http.MethodGet, "/api/v1/does-not-exist", http.Header{"Origin": {"http://localhost:5173"}})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	e := envelope(t, body)
	if e.Code != apierr.CodeNotFound {
		t.Errorf("code = %q", e.Code)
	}
	if e.CorrelationID == "" {
		t.Error("even a 404 needs a correlation id — it is the id a user reports")
	}
	if id := resp.Header.Get("X-Request-Id"); id == "" {
		t.Error("the request id must be on the response even for an unmatched route")
	}
	// The pattern does not exist for an unmatched route; recording the raw path would be the
	// only option, and that is exactly when PII discipline matters.
	if strings.Contains(body, "does-not-exist") {
		t.Errorf("the 404 body echoes the requested path back: %s", body)
	}
}

func TestWrongMethodRendersTheErrorEnvelope(t *testing.T) {
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	d := Deps{Logger: l, Cfg: testCfg("development"), Authorizer: authz.DenyAll{}, Version: "t"}
	r := New(d)
	r.DELETE("/api/v1/products/:id", Require(d, "catalogue.products.delete"), func(c *gin.Context) {})

	resp, body := do(t, r, http.MethodPut, "/api/v1/products/42", nil)
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d body=%s", resp.StatusCode, body)
	}
	if e := envelope(t, body); e.Code != apierr.CodeMethodNotAllowed {
		t.Errorf("code = %q", e.Code)
	}
}

// Gin trusts X-Forwarded-For from every address by default, which lets a client pick the IP a
// rate limiter throttles or an audit row records. Nothing is trusted until a deployment names
// its proxy networks.
func TestForwardedForCannotChooseTheClientIP(t *testing.T) {
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	r := New(Deps{Logger: l, Cfg: testCfg("development"), Authorizer: authz.DenyAll{}, Version: "t"})

	var seen string
	// Registered on POST because that is the verb the contract gives this operation; a GET here
	// would be answered 405 and the assertion would pass for the wrong reason.
	r.POST("/api/v1/checkout/quotes", func(c *gin.Context) {
		seen = c.ClientIP()
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/quotes", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	r.ServeHTTP(httptest.NewRecorder(), req)

	if seen == "1.2.3.4" {
		t.Fatal("an untrusted X-Forwarded-For decided the client IP")
	}
	if seen != "192.0.2.1" {
		t.Errorf("client IP = %q, want the transport peer", seen)
	}
}

// One line per request, from the outermost middleware to the last byte written, with the id
// that ties them together.
func TestChainLogsOneAccessRecordPerRequestWithItsCorrelationID(t *testing.T) {
	l, buf := testhelper.Logger(t, slog.LevelInfo)
	cfg := testCfg("development")
	cfg.RequestIDTrust = true // only this test asserts an inbound trace id survives the chain
	// A granting authorizer is a test double, not a production type: an exported AllowAll in the
	// authz package is one careless import away from an open endpoint.
	d := Deps{Logger: l, Cfg: cfg, Authorizer: &spyAuthorizer{}, Version: "t"}
	r := New(d)
	r.GET("/api/v1/products", Require(d, "catalogue.products.read"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"reached": true})
	})

	resp, _ := do(t, r, http.MethodGet, "/api/v1/products", http.Header{"X-Request-Id": {"trace-77"}})

	rec := buf.Find(t, "request")
	if rec == nil {
		t.Fatalf("no access record:\n%s", buf)
	}
	if len(buf.FindAll(t, "request")) != 1 {
		t.Errorf("the access log wrote more than once for one request:\n%s", buf)
	}
	if rec["route"] != "/api/v1/products" {
		t.Errorf("route = %v", rec["route"])
	}
	if rec["request_id"] != "trace-77" {
		t.Errorf("request_id = %v — the chain must inherit before it logs", rec["request_id"])
	}
	if rec["status"] != float64(200) {
		t.Errorf("status = %v, want the code the client received", rec["status"])
	}
	if id := resp.Header.Get("X-Request-Id"); id != "trace-77" {
		t.Errorf("response X-Request-Id = %q", id)
	}
}

// ── helpers ────────────────────────────────────────────────────────────────────────────────

func do(t *testing.T, r *gin.Engine, method, path string, header http.Header) (*http.Response, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec.Result(), rec.Body.String()
}

func decode(t *testing.T, body string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(body), v); err != nil {
		t.Fatalf("decoding %s: %v", body, err)
	}
}

func envelope(t *testing.T, body string) apierr.Envelope {
	t.Helper()
	var e apierr.Envelope
	decode(t, body, &e)
	return e
}

type fakeChecker struct {
	name  string
	err   error
	delay time.Duration
}

func (f *fakeChecker) Name() string { return f.name }

func (f *fakeChecker) Check(ctx context.Context) error {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
		}
	}
	return f.err
}

// ctxChecker records what the request context carried when it was asked.
type ctxChecker struct{ requestID string }

func (c *ctxChecker) Name() string { return "postgresql" }
func (c *ctxChecker) Check(ctx context.Context) error {
	c.requestID = observability.RequestID(ctx)
	return nil
}

type countingChecker struct {
	name  string
	calls atomic.Int64
}

func (c *countingChecker) Name() string { return c.name }
func (c *countingChecker) Check(context.Context) error {
	c.calls.Add(1)
	return nil
}

type spyAuthorizer struct{ last []string }

func (s *spyAuthorizer) Authorize(_ context.Context, required []string) error {
	s.last = append([]string(nil), required...)
	return nil
}
