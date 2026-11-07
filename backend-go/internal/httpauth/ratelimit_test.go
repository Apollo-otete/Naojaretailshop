package httpauth

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
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

// ADR-019's rate limiting, on the transport edge: the ceiling is per client address inside a
// sliding window, the refusal is the contract's 429 with Retry-After, and the per-account lockout
// the use case already runs stays the finer-grained second gate.

func TestRateLimiterAdmitsUpToTheLimitAndDeniesAfter(t *testing.T) {
	l := NewRateLimiter(2, 15*time.Minute)

	if !l.Allow("203.0.113.9", time.Unix(1_000_000, 0)) {
		t.Error("the first attempt inside the window must be admitted")
	}
	if !l.Allow("203.0.113.9", time.Unix(1_000_000, 0).Add(time.Minute)) {
		t.Error("the second attempt inside the window must be admitted")
	}
	if l.Allow("203.0.113.9", time.Unix(1_000_000, 0).Add(2*time.Minute)) {
		t.Error("a third attempt inside the window must be refused")
	}
	if got := l.RetryAfter("203.0.113.9", time.Unix(1_000_000, 0).Add(2*time.Minute)); got < time.Minute {
		t.Errorf("RetryAfter = %s, want just under the window", got)
	}

	// A different address is a different conversation, and the counter for one must not reach the
	// other.
	if !l.Allow("198.51.100.7", time.Unix(1_000_000, 0)) {
		t.Error("the counter for another address must be independent")
	}
}

func TestRateLimiterWindowExpires(t *testing.T) {
	l := NewRateLimiter(1, 10*time.Millisecond)

	if !l.Allow("203.0.113.9", time.Unix(2_000_000, 0)) {
		t.Error("the first attempt must be admitted")
	}
	if !l.Allow("203.0.113.9", time.Unix(2_000_000, 0).Add(20*time.Millisecond)) {
		t.Error("the attempt after the window must be admitted again")
	}
}

// Three attempts, ceiling two: the third is refused with the contract's rate-limit refusal rather
// than with the use case's own answer, and the refusal carries Retry-After.
func TestLoginAnswers429WhenThePerAddressLimitIsExceeded(t *testing.T) {
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	limiter := NewRateLimiter(2, time.Minute)
	svc := &fakeIdentity{loginEr: identity.ErrInvalidCredentials}
	r := routerWithDeps(t, svc, Deps{
		Identity:    svc,
		Authorizer:  authz.RBAC{Logger: l},
		Logger:      l,
		Cookie:      httpcookie.Session{Name: cookieName},
		RateLimiter: limiter,
	})

	for i := 1; i <= 2; i++ {
		rec := post(t, r, "/api/v1/auth/login", `{"email":"achieng.example@mail.test","password":"wrong but plausible"}`)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status %d, want 401", i, rec.Code)
		}
	}

	rec := post(t, r, "/api/v1/auth/login", `{"email":"achieng.example@mail.test","password":"wrong but plausible"}`)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429", rec.Code)
	}
	e := decodeEnvelope(t, rec)
	if e.Code != apierr.CodeRateLimited {
		t.Errorf("code %q, want %q", e.Code, apierr.CodeRateLimited)
	}
	if ra := rec.Header().Get("Retry-After"); ra == "" {
		t.Error("the 429 carries no Retry-After")
	}
	if n := len(svc.calls); n != 2 {
		t.Errorf("the third attempt reached the use case %d times: %v", n, svc.calls)
	}
}

// Register is throttled by the same per-address ceiling, on the same numbers.
func TestRegisterIsThrottledPerAddress(t *testing.T) {
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	limiter := NewRateLimiter(1, time.Minute)
	svc := &fakeIdentity{register: signedUp()}
	r := routerWithDeps(t, svc, Deps{
		Identity:    svc,
		Authorizer:  authz.RBAC{Logger: l},
		Logger:      l,
		Cookie:      httpcookie.Session{Name: cookieName},
		RateLimiter: limiter,
	})

	rec := post(t, r, "/api/v1/auth/register", goodRegistration)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, want 201: %s", rec.Code, rec.Body.String())
	}

	rec = post(t, r, "/api/v1/auth/register", goodRegistration)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429: %s", rec.Code, rec.Body.String())
	}
	if e := decodeEnvelope(t, rec); e.Code != apierr.CodeRateLimited {
		t.Errorf("code %q, want %q", e.Code, apierr.CodeRateLimited)
	}
}

// The ceiling is per address: the same client identity may still be attempted from a different
// address while one address is throttled.
func TestRateLimitIsPerAddress(t *testing.T) {
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	limiter := NewRateLimiter(1, time.Minute)
	svc := &fakeIdentity{loginEr: identity.ErrInvalidCredentials}
	r := routerWithDeps(t, svc, Deps{
		Identity:    svc,
		Authorizer:  authz.RBAC{Logger: l},
		Logger:      l,
		Cookie:      httpcookie.Session{Name: cookieName},
		RateLimiter: limiter,
	})

	if rec := post(t, r, "/api/v1/auth/login", `{"email":"achieng.example@mail.test","password":"wrong but plausible"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("first attempt: %d, want 401", rec.Code)
	}
	if rec := postFrom(t, r, "198.51.100.7", "/api/v1/auth/login", `{"email":"achieng.example@mail.test","password":"wrong but plausible"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a different address: %d, want 401 — the limit is per address", rec.Code)
	}
}

func postFrom(t *testing.T, r *gin.Engine, ip, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = ip + ":5678"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}
