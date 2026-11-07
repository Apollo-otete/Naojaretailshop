package httpauth

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpjson"
)

// RateLimiter is the per-client-address attempt counter behind ADR-019's rate limiting: lockout
// is per account, but an attacker rotating between accounts is only buying time, so the same
// window and ceiling are applied to the address the attempts arrived from.
//
// The window is sliding, which is what makes the ceiling honest: an attempt counts against the
// window that contained it, so the limit is "no more than N attempts inside any window", not
// "N attempts per fixed wall-clock bucket that can be lapped by arriving early".
type RateLimiter struct {
	limit  int
	window time.Duration
	mu     sync.Mutex
	hits   map[string][]time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit < 1 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RateLimiter{
		limit:  limit,
		window: window,
		hits:   map[string][]time.Time{},
	}
}

// Allow records one attempt from key and reports whether it was inside the limit.
func (l *RateLimiter) Allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-l.window)
	kept := make([]time.Time, 0, len(l.hits[key]))
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	l.hits[key] = kept

	if len(kept) >= l.limit {
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}

// RetryAfter returns how long the caller must wait before the same address may attempt again —
// the gap between the oldest attempt still inside the window and the window's end. A caller that
// was just admitted has nothing to wait for, so it gets the full window.
func (l *RateLimiter) RetryAfter(key string, now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-l.window)
	var oldest time.Time
	found := false
	for _, t := range l.hits[key] {
		if !t.After(cutoff) {
			continue
		}
		if !found || t.Before(oldest) {
			oldest = t
			found = true
		}
	}
	if !found {
		return l.window
	}
	return oldest.Add(l.window).Sub(now)
}

func (l *RateLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.hits, key)
}

// RateLimit returns the gin middleware answering the contract's rate-limit behaviour on the
// operations that carry it. An absent limiter denies nobody: the composition root always wires
// one, and a nil here means the caller did not wire the auth surface at all, which is the caller's
// defect rather than something this package guesses at.
func RateLimit(l *slog.Logger, limiter *RateLimiter) gin.HandlerFunc {
	if limiter == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		if limiter.Allow(c.ClientIP(), time.Now()) {
			c.Next()
			return
		}
		f := apierr.New(http.StatusTooManyRequests, apierr.CodeRateLimited,
			"Too many requests have been made from this address. Try again later.")
		f.RetryIn = int(limiter.RetryAfter(c.ClientIP(), time.Now()).Seconds())
		if f.RetryIn < 1 {
			f.RetryIn = 1
		}
		httpjson.Write(c, l, f)
	}
}
