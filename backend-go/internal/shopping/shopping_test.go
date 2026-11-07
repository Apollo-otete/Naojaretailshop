package shopping

import (
	"strings"
	"testing"
)

// The Shopping context's rules are pure functions, so they are tested without a store, a
// transaction or a database: the quantity ceiling and the guest-token hash are the whole of the
// §5 commerce rules that live here today.

func TestCheckQuantity(t *testing.T) {
	if err := CheckQuantity(1); err != nil {
		t.Errorf("the smallest orderable quantity was refused: %v", err)
	}
	if err := CheckQuantity(999); err != nil {
		t.Errorf("the largest orderable quantity was refused: %v", err)
	}
	for _, bad := range []int{0, -1, 1000} {
		if err := CheckQuantity(bad); err == nil {
			t.Errorf("quantity %d was accepted", bad)
		}
	}
}

func TestHashTokenNeverKeepsTheRawValue(t *testing.T) {
	const tok = "the-guest-token-eyeball"
	h := HashToken(tok)

	// The hash is of the trimmed token, so the same guest presented the same token twice produces
	// the same cart, and the raw value never leaves this function.
	if got := HashToken(strings.TrimSpace(tok)); got != h {
		t.Errorf("the hash is not of the trimmed token: %q vs %q", got, h)
	}
	if h == strings.TrimSpace(tok) {
		t.Error("the " + tok + " was hashed to itself: the token must not be stored in clear text")
	}
	if len(h) != 64 {
		t.Errorf("the hash is %d characters, want the 64 of a SHA-256 hex digest: %q", len(h), h)
	}
	if HashToken("another-guest-token") == h {
		t.Error("two different tokens hashed to the same value")
	}
}
