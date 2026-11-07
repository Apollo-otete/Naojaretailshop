// Package httpauth is the Identity context's driving adapter: the /auth operations from
// contracts/openapi.yaml, expressed as request reading, response writing and error mapping and
// nothing else.
//
// Every rule lives in internal/identity. §8's "Gin only handles HTTP transport and must not
// contain business rules" is checkable here by eye: the only domain functions this package calls
// are the exported validators — invoked, never reimplemented — and what the package adds on top of
// them is the field name each violation belongs to, which is a transport fact.
package httpauth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpcookie"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpjson"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpmw"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
)

// Service is the use-case surface the /auth routes need. It is the *identity.Service* shape rather
// than the type so the transport can be tested without a database, without a hasher that costs 200
// ms per hash, and without an HTTP server.
type Service interface {
	Register(ctx context.Context, in identity.RegisterInput) (identity.Authenticated, error)
	Login(ctx context.Context, in identity.LoginInput) (identity.Authenticated, error)
	Logout(ctx context.Context, p identity.Principal) error
	ChangePassword(ctx context.Context, p identity.Principal, current, next string) error
}

type CartMerger interface {
	MergeGuestIntoUser(ctx context.Context, userID int64, guestSessionToken string) error
}

type Deps struct {
	Identity   Service
	Authorizer port.Authorizer
	Logger     *slog.Logger
	Cookie     httpcookie.Session

	// RateLimiter throttles the public auth operations per client address (ADR-019). Optional at
	// the transport edge — a nil means the caller did not wire the auth surface — but the
	// composition root always supplies one, so a deployment cannot quietly run an unwalled login.
	RateLimiter *RateLimiter

	// CartMerger merges a guest cart into the authenticated account on login/register (P1).
	// Optional; failures are logged and do not block auth.
	CartMerger CartMerger
}

// Mount registers the five contracted operations. The paths hang off the router the caller passes —
// /api/v1 in the service — so the contract's `servers` block and the routes cannot drift apart in
// two places.
func Mount(r gin.IRouter, d Deps) {
	auth := r.Group("/auth")

	// ADR-019's rate limiting: register and login are throttled per client address with a sliding
	// window, answered 429 with Retry-After. The middleware is transport (which address the request
	// came from) rather than a decision about credentials, so it sits ahead of the use cases and
	// the use cases' own per-account lockout stays the second, finer-grained gate.
	rateLimit := RateLimit(d.Logger, d.RateLimiter)
	auth.POST("/register", rateLimit, register(d))
	auth.POST("/login", rateLimit, login(d))

	// Logout is deliberately not behind the authorizer. Contract: "calling it with an
	// already-revoked cookie succeeds and does nothing, because reporting 'not signed in' here
	// would leak session state to a caller who no longer has it" — and the documented response list
	// is 204 and 500, with no 401 to answer an anonymous request with.
	auth.POST("/logout", logout(d))

	// x-permissions: [] means "any authenticated principal". The evaluator stops at the principal
	// check for an empty key list, so this is that promise and not a bypass.
	auth.GET("/session", httpmw.Authorize(d.Logger, d.Authorizer), currentSession(d))

	// identity.account.update carries owner_is_caller (ADR-024), so this needs the variant that can
	// name a resource: the account being changed is the caller's own.
	auth.POST("/password/change",
		httpmw.AuthorizeOwnAccount(d.Logger, d.Authorizer, "identity.account.update"),
		changePassword(d))
}

// registerRequest mirrors components.schemas.RegisterRequest. The JSON names are the contract's;
// the Go names never reach a client, so they are not what anyone switches on.
type registerRequest struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func guestCartToken(c *gin.Context) string {
	if v, err := c.Cookie("naoja_cart"); err == nil && v != "" {
		return v
	}
	return c.GetHeader("X-Cart-Session")
}

func mergeGuestCart(c *gin.Context, d Deps, userID int64) {
	if d.CartMerger == nil || userID <= 0 {
		return
	}
	tok := guestCartToken(c)
	if tok == "" {
		return
	}
	if err := d.CartMerger.MergeGuestIntoUser(c.Request.Context(), userID, tok); err != nil && d.Logger != nil {
		d.Logger.Warn("guest cart merge failed", slog.Int64("user_id", userID), slog.Any("err", err))
	}
}

func register(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in registerRequest
		if err := decode(c, &in); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		if details := checkRegistration(in); len(details) > 0 {
			httpjson.Write(c, d.Logger, apierr.Validation(details...))
			return
		}

		out, err := d.Identity.Register(c.Request.Context(), identity.RegisterInput{
			FullName:  in.FullName,
			Email:     in.Email,
			Phone:     in.Phone,
			Password:  in.Password,
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		})
		if err != nil {
			httpjson.Fail(c, d.Logger, registrationFailure(err))
			return
		}

		mergeGuestCart(c, d, out.Principal.User.ID)
		d.Cookie.Set(c.Writer, out.Session.Token)
		httpjson.Created(c, sessionViewOf(out.Principal))
	}
}

// checkRegistration runs the domain's own validators, one per field, so a form with three mistakes
// gets three answers in one response. The rules are not restated here: identity.CheckName is the
// same function the use case calls moments later on the same value, which is why checking in the
// transport cannot disagree with the write it precedes.
func checkRegistration(in registerRequest) []apierr.Detail {
	var details []apierr.Detail
	reject := func(field string, err error) {
		if err != nil {
			details = append(details, apierr.Detail{Field: field, Issue: err.Error()})
		}
	}
	reject("full_name", identity.CheckName(in.FullName))
	if _, err := identity.NormaliseEmail(in.Email); err != nil {
		reject("email", err)
	}
	if _, err := identity.NormalisePhone(in.Phone); err != nil {
		reject("phone", err)
	}
	reject("password", identity.CheckPasswordPolicy(in.Password))
	return details
}

// registrationFailure maps the two 409s the contract documents for registration, and only those.
//
// Nothing else needs a case: every way a value can be wrong was already answered by
// checkRegistration, which calls the identical domain functions, so a second validation failure
// from the use case would be a contradiction between two views of one input — and a contradiction
// is worth a 500 and a log line, not a polite 422.
//
// Note the asymmetry matrix C1 requires: registration may name the field that collided, because the
// person filling in the form is the one who owns that address, while login may not.
func registrationFailure(err error) error {
	switch {
	case errors.Is(err, identity.ErrEmailTaken):
		return apierr.Conflict(apierr.CodeConflict,
			"An account already uses this email address.",
			apierr.Detail{Field: "email", Issue: "already registered"})
	case errors.Is(err, identity.ErrPhoneTaken):
		return apierr.Conflict(apierr.CodeConflict,
			"An account already uses this phone number.",
			apierr.Detail{Field: "phone", Issue: "already registered"})
	case errors.Is(err, identity.ErrBreachedPassword):
		return apierr.Validation(apierr.Detail{Field: "password", Issue: err.Error()})
	}
	return err
}

func login(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in loginRequest
		if err := decode(c, &in); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}

		out, err := d.Identity.Login(c.Request.Context(), identity.LoginInput{
			Email:     in.Email,
			Password:  in.Password,
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		})
		if err != nil {
			httpjson.Fail(c, d.Logger, loginFailure(err))
			return
		}

		mergeGuestCart(c, d, out.Principal.User.ID)
		d.Cookie.Set(c.Writer, out.Session.Token)
		httpjson.OK(c, sessionViewOf(out.Principal))
	}
}

// loginFailure keeps matrix C1's non-disclosure rule intact: a suspended or deactivated account is
// answered with the same 401 and the same words as a wrong password, because "your account is
// disabled" confirms that the address is registered. The distinction is not lost, it moves — the
// use case logs which state it refused.
//
// A malformed email is the one failure that *is* disclosed, and the use case returns a different
// error for it on purpose: telling someone their own typo is "email or password is incorrect"
// sends them to reset a password they never entered wrongly.
func loginFailure(err error) error {
	switch {
	case errors.Is(err, identity.ErrInvalidEmail):
		return apierr.Validation(apierr.Detail{Field: "email", Issue: err.Error()})
	case errors.Is(err, identity.ErrLocked):
		return apierr.New(http.StatusLocked, apierr.CodeAccountLocked,
			"This account is temporarily locked after repeated failed sign-ins. Try again later.")
	case errors.Is(err, identity.ErrInvalidCredentials),
		errors.Is(err, identity.ErrSuspended),
		errors.Is(err, identity.ErrInactive):
		return apierr.Unauthorized("Email or password is incorrect.")
	}
	return err
}

func logout(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// No principal means the cookie was absent, expired or already revoked; Authenticate has
		// already cleared it, and answering 204 again is the idempotence the contract asks for.
		if caller, ok := identity.PrincipalFrom(c.Request.Context()); ok {
			if err := d.Identity.Logout(c.Request.Context(), caller); err != nil {
				httpjson.Fail(c, d.Logger, err)
				return
			}
		}
		d.Cookie.Clear(c.Writer)
		httpjson.NoContent(c)
	}
}

func currentSession(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := identity.PrincipalFrom(c.Request.Context())
		if !ok {
			// Authorize refuses this request before the handler runs; reaching here without a
			// principal means the chain was assembled wrongly, and 401 is the honest answer either
			// way.
			httpjson.Write(c, d.Logger, apierr.Unauthorized("Sign in to see your session."))
			return
		}
		httpjson.OK(c, sessionViewOf(caller))
	}
}

func changePassword(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := identity.PrincipalFrom(c.Request.Context())
		if !ok {
			httpjson.Write(c, d.Logger, apierr.Unauthorized("Sign in to change your password."))
			return
		}

		var in changePasswordRequest
		if err := decode(c, &in); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}

		err := d.Identity.ChangePassword(c.Request.Context(), caller, in.CurrentPassword, in.NewPassword)
		if mapped := changePasswordFailure(err); mapped != nil {
			httpjson.Fail(c, d.Logger, mapped)
			return
		}

		// 204, and the cookie stays: this session is the one ChangePassword keeps. Every other
		// session for the account is revoked inside the use case, which is ADR-019's answer to a
		// password that leaked.
		httpjson.NoContent(c)
	}
}

// changePasswordFailure maps the three ways this request is refused after authorisation.
//
// A wrong current password is answered as 422, not as the 401 the operation also documents: this
// contract's Unauthenticated tells the client to "route to login and preserve intent", and throwing
// a login screen at someone who mistyped their existing password inside their own settings form
// interrupts a session that is still perfectly valid. 401 therefore keeps its one meaning
// everywhere — no valid session.
func changePasswordFailure(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, identity.ErrSamePassword):
		return apierr.Validation(apierr.Detail{Field: "new_password", Issue: "must differ from the current password"})
	case errors.Is(err, identity.ErrPasswordTooShort), errors.Is(err, identity.ErrPasswordTooLong), errors.Is(err, identity.ErrBreachedPassword):
		return apierr.Validation(apierr.Detail{Field: "new_password", Issue: err.Error()})
	case errors.Is(err, identity.ErrInvalidCredentials):
		return apierr.Validation(apierr.Detail{Field: "current_password", Issue: "is not your current password"})
	}
	return err
}

// decode reads one JSON object. A body that cannot be parsed is a 400 that quotes nothing from it,
// and the parse error is not logged either: on this endpoint the bytes being parsed are a password,
// and a syntax error can echo part of the input it choked on.
func decode(c *gin.Context, target any) error {
	if err := json.NewDecoder(c.Request.Body).Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return apierr.BadRequest("A JSON request body is required.")
		}
		return apierr.BadRequest("The request body is not valid JSON.")
	}
	return nil
}

// sessionView is the contract's Session schema. permissions is required, and an empty one has to
// arrive as [] rather than null: Go marshals a nil slice to JSON null, and every consumer of that
// field would then have to know the difference between "no permissions" and "no array".
type sessionView struct {
	User        userView  `json:"user"`
	Roles       []string  `json:"roles"`
	Permissions []string  `json:"permissions"`
	Scoped      []string  `json:"scoped_permissions"`
	CSRFToken   string    `json:"csrf_token"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type userView struct {
	ID              int64      `json:"id"`
	FullName        string     `json:"full_name"`
	Email           string     `json:"email"`
	Phone           *string    `json:"phone"`
	AccountKind     string     `json:"account_kind"`
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
	PhoneVerifiedAt *time.Time `json:"phone_verified_at"`
}

func sessionViewOf(p identity.Principal) sessionView {
	return sessionView{
		User: userView{
			ID:              p.User.ID,
			FullName:        p.User.FullName,
			Email:           p.User.Email,
			Phone:           optionalString(p.User.Phone),
			AccountKind:     string(p.User.Account),
			EmailVerifiedAt: p.User.EmailVerifiedAt,
			PhoneVerifiedAt: p.User.PhoneVerifiedAt,
		},
		Roles:       orEmpty(p.Roles),
		Permissions: orEmpty(p.Permissions),
		Scoped:      scopedPermissions(p.Conditions),
		CSRFToken:   p.CSRFToken,
		ExpiresAt:   p.ExpiresAt,
	}
}

// scopedPermissions is Session.scoped_permissions — "keys the backend evaluates against ownership".
// It is rendered from the same snapshot the evaluator used on this request (ADR-024), which is what
// makes it a statement about this caller's authority rather than a copy of the catalogue. Sorted
// because it comes out of a map, and a client that caches the array must not see it reorder.
func scopedPermissions(conditions map[string][]string) []string {
	out := make([]string, 0, len(conditions))
	for permission, names := range conditions {
		if len(names) > 0 {
			out = append(out, permission)
		}
	}
	sort.Strings(out)
	return out
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// The compile-time way to notice that the routes below have started calling a use case this adapter
// does not declare.
var _ Service = (*identity.Service)(nil)
