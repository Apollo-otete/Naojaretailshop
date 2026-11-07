// Package identity is the Identity & Access bounded context (§7): the principal, the session
// that carries it, and the use cases behind the /auth operations.
//
// §8 puts business rules here rather than in Gin, so "nothing in this package imports an HTTP
// type" is a structural claim, not a style preference: a rule that needs a *gin.Context* cannot
// be tested without a server, and Phase 3's exit criterion is authorization tests.
package identity

import (
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// AccountKind splits storage concerns, not authority: a customer can be ordered into, an admin
// cannot. Authority is the role. Conflating the two is exactly what the single legacy `admins`
// table does (inventory §6.2), and identity.users CHECK-constrains this to two values.
type AccountKind string

const (
	KindCustomer AccountKind = "customer"
	KindAdmin    AccountKind = "admin"
)

// Password policy from contracts/openapi.yaml RegisterRequest: a length floor and a cap, and
// deliberately no composition rules — "no rules that push people towards write-it-on-a-sticky-note
// behaviour". The cap exists because Argon2 accepts any input and an unbounded field is a
// denial-of-service surface: hashing is the most expensive thing the login path does.
const (
	PasswordMinLength = 10
	PasswordMaxLength = 200
)

const (
	nameMinLength = 2
	nameMaxLength = 120
)

var (
	ErrInvalidEmail     = errors.New("email is not a valid address")
	ErrInvalidPhone     = errors.New("phone must be a Kenyan mobile number")
	ErrInvalidName      = errors.New("full name must be between 2 and 120 characters")
	ErrPasswordTooShort = errors.New("password is too short")
	ErrPasswordTooLong  = errors.New("password is too long")

	// ErrPasswordHashRequired guards the one mistake that would store a plaintext password: a
	// caller that hands NewCustomer the password instead of password.Hash's output.
	ErrPasswordHashRequired = errors.New("a new account needs a password hash, not a password")

	// ErrBreachedPassword indicates the password appears in a known-breach corpus (hook; optional checker).
	ErrBreachedPassword = errors.New("password has appeared in a data breach")

	// ErrInvalidCredentials is the single answer given for "no such account" and "wrong
	// password". Contract /auth/login: "Failures do not disclose whether the email exists."
	ErrInvalidCredentials = errors.New("email or password is incorrect")
)

// phonePattern is the contract's MSISDN pattern: 07XXXXXXXX, 01XXXXXXXX, or the same in +254
// form. Local numbers are normalised to +254 on write so that a phone is comparable across the
// two spellings — the column is UNIQUE, and "0712345678" and "+254712345678" would otherwise be
// two accounts for one person.
var phonePattern = regexp.MustCompile(`^(?:\+254|0)[17]\d{8}$`)

// User is a principal. The zero value is not usable; build one with NewCustomer or NewAdmin and
// let the store fill ID and the timestamps.
type User struct {
	ID       int64
	Email    string
	Phone    string
	FullName string
	Account  AccountKind

	PasswordHash string

	IsActive     bool
	IsSuspended  bool
	LockedUntil  *time.Time
	FailedLogins int

	EmailVerifiedAt *time.Time
	PhoneVerifiedAt *time.Time
	LastLoginAt     *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewCustomer builds the account a storefront registration creates. It validates rather than
// coerces: a caller that wants a normalised value asks for NormaliseEmail/NormalisePhone first,
// so what the client sent and what got stored are never silently different.
func NewCustomer(fullName, email, phone, passwordHash string) (User, error) {
	return newUser(KindCustomer, fullName, email, phone, passwordHash)
}

// NewAdmin is the path an invitation acceptance or a Super Admin creates, never a public POST.
func NewAdmin(fullName, email, phone, passwordHash string) (User, error) {
	return newUser(KindAdmin, fullName, email, phone, passwordHash)
}

func newUser(kind AccountKind, fullName, email, phone, passwordHash string) (User, error) {
	if err := CheckName(fullName); err != nil {
		return User{}, err
	}
	normalizedEmail, err := NormaliseEmail(email)
	if err != nil {
		return User{}, err
	}
	if err := CheckAccountKind(kind); err != nil {
		return User{}, err
	}
	// A customer must be reachable for delivery; an admin may exist without a phone.
	normalizedPhone := ""
	if phone != "" {
		p, err := NormalisePhone(phone)
		if err != nil {
			return User{}, err
		}
		normalizedPhone = p
	}
	if kind == KindCustomer && normalizedPhone == "" {
		return User{}, ErrInvalidPhone
	}
	if passwordHash == "" {
		return User{}, ErrPasswordHashRequired
	}

	return User{
		Email:        normalizedEmail,
		Phone:        normalizedPhone,
		FullName:     strings.TrimSpace(fullName),
		Account:      kind,
		PasswordHash: passwordHash,
		IsActive:     true,
	}, nil
}

// MayAct is the per-request check: does this account still exist and is it still allowed to
// behave. Suspension and deactivation are included; the login lockout is not. A lockout stops
// *new* logins after repeated failed guesses — it is not a revocation, and letting it cut off a
// session that is already open would mean a wrong password typed by a stranger could log someone
// out of their own work. Revoking an existing session is is_suspended's job, or a revoke call's.
//
// This is checked per request rather than per login, which is what makes B11 ("fails on the next
// request, not only on the next login") a property of the code path instead of a cache.
func (u *User) MayAct() bool { return u.IsActive && !u.IsSuspended }

func (u *User) IsLocked(now time.Time) bool {
	return u.LockedUntil != nil && u.LockedUntil.After(now)
}

// RegisterFailure applies one failed attempt. The counter and the lock are set together so a
// caller cannot record an attempt without deciding what it costs: five failures then a
// fifteen-minute lock is ADR-019's per-account half of the login limit, and it is the half the
// current code does not have at all — its lockout is per address only, so an attacker who
// rotates IPs never meets a wall.
func (u *User) RegisterFailure(now time.Time, limit int, window time.Duration) {
	u.FailedLogins++
	if limit > 0 && u.FailedLogins >= limit {
		until := now.Add(window)
		u.LockedUntil = &until
		u.FailedLogins = 0
	}
}

// RegisterSuccess clears the failure state and stamps the login.
func (u *User) RegisterSuccess(now time.Time) {
	u.FailedLogins = 0
	u.LockedUntil = nil
	u.LastLoginAt = &now
}

func CheckName(fullName string) error {
	n := utf8.RuneCountInString(strings.TrimSpace(fullName))
	if n < nameMinLength || n > nameMaxLength {
		return ErrInvalidName
	}
	return nil
}

// CheckPasswordPolicy is the length half of the contract's password rule. It takes the
// plaintext, so it must be called before hashing and its argument must never be logged or
// stored — the same reason the hash column is named password_hash.
func CheckPasswordPolicy(password string) error {
	n := utf8.RuneCountInString(password)
	switch {
	case n < PasswordMinLength:
		return fmt.Errorf("%w: at least %d characters", ErrPasswordTooShort, PasswordMinLength)
	case n > PasswordMaxLength:
		return fmt.Errorf("%w: at most %d characters", ErrPasswordTooLong, PasswordMaxLength)
	}
	return nil
}

func CheckAccountKind(kind AccountKind) error {
	if kind != KindCustomer && kind != KindAdmin {
		return fmt.Errorf("unknown account kind %q", kind)
	}
	return nil
}

// NormaliseEmail validates and lowercases an address. Lowercasing is not cosmetic: the column is
// CITEXT, so "Achieng@mail.test" and "achieng@mail.test" are one address to the database and the
// UNIQUE constraint rejects the second insert — the application has to compare and store the
// same normalised form the database does, or registration fails with an unexplained 409.
func NormaliseEmail(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) > 254 {
		return "", ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(trimmed)
	if err != nil || addr.Address != trimmed {
		return "", ErrInvalidEmail
	}
	// RFC 5322 accepts "a@b". A shop does not: that is what a mistyped domain looks like, and
	// an account created on one is unreachable forever.
	if !strings.Contains(addr.Address, ".") {
		return "", ErrInvalidEmail
	}
	return strings.ToLower(addr.Address), nil
}

func NormalisePhone(raw string) (string, error) {
	// Spaces, hyphens and parentheses are how people write a phone number; +254 712 345 678 and
	// +254712345678 are the same number and must become the same stored value.
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, strings.TrimSpace(raw))

	phone := digits
	switch {
	case strings.HasPrefix(digits, "00254"):
		phone = "+254" + digits[5:]
	case strings.HasPrefix(digits, "254"):
		phone = "+" + digits
	case strings.HasPrefix(digits, "0"):
		phone = "+254" + digits[1:]
	}
	if !phonePattern.MatchString(phone) {
		return "", ErrInvalidPhone
	}
	return phone, nil
}
