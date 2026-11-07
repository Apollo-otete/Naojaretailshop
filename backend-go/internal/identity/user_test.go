package identity_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
)

func TestNormalisePhone(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "0712345678", want: "+254712345678"},
		{in: "0112345678", want: "+254112345678"},
		{in: "+254712345678", want: "+254712345678"},
		// The same number written the way a person writes it is the same stored value, because
		// the column is UNIQUE and an order can be tracked by it.
		{in: "+254 712 345 678", want: "+254712345678"},
		{in: "0712-345-678", want: "+254712345678"},
		{in: "(0712) 345678", want: "+254712345678"},
		{in: "00254712345678", want: "+254712345678"},

		{in: "", wantErr: true},
		{in: "0612345678", wantErr: true},  // [17] only: 06… is not a Kenyan mobile prefix
		{in: "071234567", wantErr: true},   // too short
		{in: "07123456789", wantErr: true}, // too long
		{in: "+255712345678", wantErr: true},
		{in: "+25411234567", wantErr: true},
		{in: "0712345abc", wantErr: true},
	}
	for _, c := range cases {
		got, err := identity.NormalisePhone(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("NormalisePhone(%q) = %q, want an error", c.in, got)
			} else if !errors.Is(err, identity.ErrInvalidPhone) {
				t.Errorf("NormalisePhone(%q) error = %v, want ErrInvalidPhone", c.in, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalisePhone(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("NormalisePhone(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormaliseEmail(t *testing.T) {
	for _, ok := range []string{"Achieng@Mail.TEST ", "achieng.example@mail.test"} {
		got, err := identity.NormaliseEmail(ok)
		if err != nil {
			t.Errorf("NormaliseEmail(%q): %v", ok, err)
			continue
		}
		if want := strings.ToLower(strings.TrimSpace(ok)); got != want {
			t.Errorf("NormaliseEmail(%q) = %q, want %q", ok, got, want)
		}
	}
	for _, bad := range []string{"", "   ", "achieng", "@mail.test", "achieng@", "a@b", "A <a@mail.test>", "a b@mail.test"} {
		if got, err := identity.NormaliseEmail(bad); err == nil {
			t.Errorf("NormaliseEmail(%q) = %q, want an error", bad, got)
		} else if !errors.Is(err, identity.ErrInvalidEmail) {
			t.Errorf("NormaliseEmail(%q) error = %v, want ErrInvalidEmail", bad, err)
		}
	}
}

func TestCheckPasswordPolicyIsLengthOnly(t *testing.T) {
	// The contract's rule, and the reason it is only length: composition rules push people to
	// write passwords down, which is weaker than a long passphrase typed from memory.
	if err := identity.CheckPasswordPolicy(strings.Repeat("a", identity.PasswordMinLength-1)); !errors.Is(err, identity.ErrPasswordTooShort) {
		t.Errorf("9 characters = %v, want ErrPasswordTooShort", err)
	}
	if err := identity.CheckPasswordPolicy(strings.Repeat("a", identity.PasswordMinLength)); err != nil {
		t.Errorf("the minimum length was refused: %v", err)
	}
	if err := identity.CheckPasswordPolicy(strings.Repeat("a", identity.PasswordMaxLength)); err != nil {
		t.Errorf("the maximum length was refused: %v", err)
	}
	if err := identity.CheckPasswordPolicy(strings.Repeat("a", identity.PasswordMaxLength+1)); !errors.Is(err, identity.ErrPasswordTooLong) {
		t.Errorf("201 characters = %v, want ErrPasswordTooLong", err)
	}
	// No digit, no symbol, no uppercase — and it must still be accepted.
	if err := identity.CheckPasswordPolicy("correct horse battery staple"); err != nil {
		t.Errorf("a lowercase passphrase was refused: %v", err)
	}
	// Length is counted in characters, not bytes: a user typing ten characters of a multi-byte
	// script has met a ten-character rule, and failing them because strlen said thirty is how a
	// password policy turns into a language policy.
	tenRunes := strings.Repeat("आ", identity.PasswordMinLength)
	if bytes := len(tenRunes); bytes == identity.PasswordMinLength {
		t.Fatalf("test input is %d bytes; the point needs a rune that is more than one byte", bytes)
	}
	if err := identity.CheckPasswordPolicy(tenRunes); err != nil {
		t.Errorf("a 10-character unicode password was refused: %v", err)
	}
	if err := identity.CheckPasswordPolicy(strings.Repeat("आ", identity.PasswordMinLength-1)); !errors.Is(err, identity.ErrPasswordTooShort) {
		t.Errorf("9 unicode characters = %v, want ErrPasswordTooShort", err)
	}
}

func TestNewCustomerAndNewAdmin(t *testing.T) {
	customer, err := identity.NewCustomer("Achieng Otieno", "a@mail.test", "0712345678", "fake$deadbeef")
	if err != nil {
		t.Fatalf("NewCustomer: %v", err)
	}
	if customer.Account != identity.KindCustomer || !customer.IsActive || customer.IsSuspended {
		t.Errorf("new customer = %+v", customer)
	}
	if customer.Phone != "+254712345678" {
		t.Errorf("phone = %q, want the normalised form", customer.Phone)
	}
	if customer.PasswordHash != "fake$deadbeef" {
		t.Error("the hash was not stored verbatim")
	}

	// An admin may exist without a phone; a customer may not, because delivery needs one.
	if _, err := identity.NewAdmin("Admin Person", "admin@mail.test", "", "fake$deadbeef"); err != nil {
		t.Errorf("NewAdmin without a phone: %v", err)
	}
	if _, err := identity.NewCustomer("No Phone", "nop@mail.test", "", "fake$deadbeef"); !errors.Is(err, identity.ErrInvalidPhone) {
		t.Errorf("customer without a phone = %v, want ErrInvalidPhone", err)
	}

	if _, err := identity.NewCustomer("A", "a@mail.test", "0712345678", "fake$x"); !errors.Is(err, identity.ErrInvalidName) {
		t.Errorf("one-character name = %v, want ErrInvalidName", err)
	}
	if _, err := identity.NewCustomer("Ok Name", "a@mail.test", "0712345678", ""); !errors.Is(err, identity.ErrPasswordHashRequired) {
		t.Errorf("empty hash = %v, want ErrPasswordHashRequired", err)
	}
}

// The point of RegisterFailure is that the counter and the lock move together, and that reaching
// the limit costs the *attacker* time rather than costing the account nothing.
func TestRegisterFailureLocksAtTheLimit(t *testing.T) {
	now := time.Now()
	u := identity.User{IsActive: true}
	for i := 1; i < 5; i++ {
		u.RegisterFailure(now, 5, 15*time.Minute)
		if u.IsLocked(now) {
			t.Fatalf("the account locked after %d failures, want 5", i)
		}
	}
	u.RegisterFailure(now, 5, 15*time.Minute)
	if !u.IsLocked(now) {
		t.Fatal("five failures did not lock the account")
	}
	if u.FailedLogins != 0 {
		t.Errorf("failed count = %d after locking, want the counter reset with the lock", u.FailedLogins)
	}
	if got, want := *u.LockedUntil, now.Add(15*time.Minute); !got.Equal(want) {
		t.Errorf("locked until %v, want %v", got, want)
	}
	if u.IsLocked(now.Add(16 * time.Minute)) {
		t.Error("the lock outlived its window")
	}

	u.RegisterSuccess(now)
	if u.LockedUntil != nil || u.FailedLogins != 0 || u.LastLoginAt == nil {
		t.Errorf("RegisterSuccess left %+v", u)
	}
}

// A lockout is not a revocation: it stops new logins, and MayAct — the per-request question —
// must not answer it, or someone guessing wrong passwords could log an operator out of a
// session they are using.
func TestLockoutDoesNotRevokeAnOpenSession(t *testing.T) {
	now := time.Now()
	u, err := identity.NewAdmin("Locked Admin", "locked@mail.test", "", "fake$x")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		u.RegisterFailure(now, 5, 15*time.Minute)
	}
	if !u.IsLocked(now) {
		t.Fatal("test setup: the account is not locked")
	}
	if !u.MayAct() {
		t.Error("a login lockout took away the right to act on an existing session")
	}

	u.IsSuspended = true
	if u.MayAct() {
		t.Error("a suspension did not take away the right to act")
	}
	u.IsSuspended, u.IsActive = false, false
	if u.MayAct() {
		t.Error("a deactivated account may still act")
	}
}
