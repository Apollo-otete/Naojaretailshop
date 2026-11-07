// Package server assembles the Gin engine. It is the only package that knows both the
// transport middleware and the dependency graph, which is what makes "Gin must not contain
// business rules" (§8) checkable by eye: if a rule appears here, it is in the wrong place.
package server

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/config"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpauth"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpcatalogue"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpcheckout"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpcookie"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpjson"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpmw"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpordering"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httppayments"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpshopping"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/observability"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/ordering"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
)

// ErrMissingDependency is what New reports for a wiring that would silently degrade the chain.
// It is an error rather than a panic because main.go already turns a startup error into exit 1,
// and a service that comes up without an authorizer is the outage this package should refuse to
// produce.
var ErrMissingDependency = errors.New("server is not fully wired")

type Deps struct {
	Logger     *slog.Logger
	Cfg        config.Config
	Authorizer port.Authorizer

	// Sessions is the per-request session cookie read (ADR-019). It is a dependency of the
	// whole service rather than of one route because the contract declares sessionCookie at
	// the top level: there is no operation whose authorization answer may depend on a cookie
	// nobody looked at.
	Sessions httpmw.Sessions

	// Identity is the use-case surface behind the /auth operations. It is mounted here rather
	// than by the composition root so that the middleware ordering below — authentication and
	// CSRF ahead of every business route — is assembled in one place and cannot be recreated
	// wrongly somewhere else.
	Identity   httpauth.Service
	CartMerger httpauth.CartMerger

	// AuthRateLimit is ADR-019's per-address rate limit for the public auth operations (register,
	// login). It is optional here — the engine-level tests build an engine without the full
	// dependency graph — and the composition root always supplies it, so a deployment cannot run an
	// unthrottled login.
	AuthRateLimit *httpauth.RateLimiter

	// TrackRateLimit rate-limits public verified order tracking (anti-enumeration). Optional; a
	// safe default is applied when nil.
	TrackRateLimit *httpauth.RateLimiter

	// Catalogue is the use-case surface behind the /products and /categories operations.
	Catalogue httpcatalogue.Service

	// Shopping is the use-case surface behind the /cart operations.
	Shopping httpshopping.Service

	// Ordering is the use-case surface behind the /orders operations (contract idempotent placement).
	Ordering httpordering.Service

	// Checkout provides contract quote creation (/checkout/quotes). Optional if Ordering handles it.
	Checkout httpcheckout.Service

	// Payments handles STK initiation and Daraja callbacks (P1). Optional.
	Payments httppayments.Service

	// Legacy compat: order store/tx for translating the existing frontend's checkout payload
	// where the full cart/quote flow is not yet wired end-to-end. Optional.
	LegacyOrderStore ordering.OrderStore
	LegacyOrderTx    ordering.Transactor

	Checkers  []port.Checker
	Version   string
	StartedAt time.Time
}

// New builds the engine without starting a listener, so the whole chain — request id, CORS,
// recovery, authentication, CSRF, error envelope — can be exercised by httptest with no port
// involved.
func New(d Deps) (*gin.Engine, error) {
	if d.Authorizer == nil {
		return nil, fmt.Errorf("%w: Authorizer — with none, every protected operation would either be open or refused by a stub", ErrMissingDependency)
	}
	if d.Sessions == nil {
		return nil, fmt.Errorf("%w: Sessions — without the cookie read nothing reaches a handler with a principal, and the authorizer can only say no", ErrMissingDependency)
	}
	if d.Identity == nil {
		return nil, fmt.Errorf("%w: Identity — the contract's /auth operations would be registered against nothing, and there would be no way to obtain the session every other route requires", ErrMissingDependency)
	}
	// Catalogue is optional at engine construction so the middleware/health tests can build an
	// engine without a full dependency graph; main.go always wires it, and its absence is a
	// deployment error caught by review rather than by a startup panic. When present, its routes
	// are mounted below alongside identity's.
	catalogueSvc := d.Catalogue
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if d.Version == "" {
		d.Version = "dev"
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// gin.New() leaves this off, which would answer PUT /products/42 with a 404 and hide the
	// fact that the resource exists — the contract documents METHOD_NOT_ALLOWED for exactly
	// that case, and a client cannot tell "wrong verb" from "wrong url" from a 404.
	r.HandleMethodNotAllowed = true

	// Gin trusts X-Forwarded-For from every address by default, so a client can choose the IP a
	// future rate limiter or audit entry records. Nothing is trusted until a deployment names the
	// proxy networks it sits behind.
	if err := r.SetTrustedProxies(nil); err != nil {
		d.Logger.Error("could not disable forwarded client IP", slog.String("error", err.Error()))
	}

	// Order matters. Recover is outermost so a panic in any later middleware still produces an
	// envelope with a correlation id; RequestID precedes Access so the access log line carries it.
	r.Use(httpmw.Recover(d.Logger))
	r.Use(httpmw.RequestID(d.Logger, d.Cfg.RequestIDTrust))
	r.Use(httpmw.Access(d.Logger))
	r.Use(httpmw.BodyLimit(10 << 20)) // 10 MiB safety net; handlers may tighten
	r.Use(httpmw.CORS(d.Logger, d.Cfg.CORS.AllowedOrigins))

	// Health is registered *before* the next two Use calls, and that is not incidental: Gin
	// snapshots the middleware list when a route is registered, so these routes are served
	// without a session read. §10's liveness rule — never touch a dependency to answer "am I
	// alive" — would otherwise be broken by a database outage turning /healthz into a 500 and
	// the orchestrator into a restart storm.
	registerHealth(r, d)

	cookie := httpcookie.Session{
		Name: d.Cfg.SessionCookieName,
		// A Secure-only cookie cannot be set over http://localhost, so the development
		// machine would never exercise the cookie path at all. Production is the only
		// environment where cleartext is a real exposure.
		Secure: d.Cfg.IsProduction(),
	}

	// Legacy /api compatibility for the existing frontend (token/Bearer auth, legacy shapes).
	// Mounted before cookie session+CSRF so the frontend's current auth flow keeps working while
	// /api/v1 remains the contract surface. Both are served by this one binary.
	legacy := r.Group("/api")
	if catalogueSvc != nil || d.Identity != nil || d.Shopping != nil || d.Ordering != nil {
		mountLegacy(legacy, d, cookie)
	}

	// Daraja callbacks are server-to-server (no user session/CSRF). Mount before auth middleware.
	if d.Payments != nil {
		httppayments.MountCallback(r, httppayments.Deps{Payments: d.Payments, Authorizer: d.Authorizer, Logger: d.Logger})
	}

	// Everything from here on is the contract surface, authenticated by session cookie and CSRF.
	r.Use(httpmw.Authenticate(d.Logger, d.Sessions, cookie))
	r.Use(httpmw.CSRF(d.Logger))

	// The /api/v1 group is created here, after both Use calls, and that is the same mechanism as
	// the health note above: Group copies the middleware chain as it stands, so a group made
	// earlier would be served without authentication. The version prefix is the contract's
	// `servers` block (§11's "versioned REST under /api/v1"), stated once.
	api := r.Group("/api/v1")
	httpauth.Mount(api, httpauth.Deps{
		Identity:    d.Identity,
		Authorizer:  d.Authorizer,
		Logger:      d.Logger,
		Cookie:      cookie,
		RateLimiter: d.AuthRateLimit,
		CartMerger:  d.CartMerger,
	})

	if catalogueSvc != nil {
		httpcatalogue.Mount(api, httpcatalogue.Deps{
			Catalogue:  catalogueSvc,
			Authorizer: d.Authorizer,
			Logger:     d.Logger,
		})
	}

	if d.Shopping != nil {
		httpshopping.Mount(api, httpshopping.Deps{
			Shopping:   d.Shopping,
			Authorizer: d.Authorizer,
			Logger:     d.Logger,
		})
	}

	checkoutSvc := d.Checkout
	if checkoutSvc == nil && d.Ordering != nil {
		// *ordering.Service satisfies httpcheckout.Service; allow a single dep in simple wiring.
		if cs, ok := any(d.Ordering).(httpcheckout.Service); ok {
			checkoutSvc = cs
		}
	}
	if checkoutSvc != nil {
		httpcheckout.Mount(api, httpcheckout.Deps{
			Ordering:   checkoutSvc,
			Authorizer: d.Authorizer,
			Logger:     d.Logger,
		})
	}

	trackLimiter := d.TrackRateLimit
	if trackLimiter == nil {
		trackLimiter = httpauth.NewRateLimiter(15, time.Minute)
	}
	if d.Ordering != nil {
		httpordering.Mount(api, httpordering.Deps{
			Ordering:     d.Ordering,
			Authorizer:   d.Authorizer,
			Logger:       d.Logger,
			TrackLimiter: trackLimiter,
		})
	}

	if d.Payments != nil {
		httppayments.MountAuthenticated(api, httppayments.Deps{Payments: d.Payments, Authorizer: d.Authorizer, Logger: d.Logger})
	}

	// Unmatched routes get the contract envelope. Gin's default is a plain-text "404 page not
	// found", which any client that parses errors has to special-case — and a body that only
	// exists for half the responses is how a frontend ends up guessing.
	r.NoRoute(func(c *gin.Context) {
		httpjson.Write(c, d.Logger, apierr.NotFound("route"))
	})
	r.NoMethod(func(c *gin.Context) {
		httpjson.Write(c, d.Logger, apierr.New(http.StatusMethodNotAllowed, apierr.CodeMethodNotAllowed,
			"This resource does not accept that method."))
	})

	return r, nil
}

// Require registers a protected operation. The permission keys sit next to the route so the
// contract's x-permissions list and the enforcement stay one edit apart.
func Require(d Deps, permissions ...string) gin.HandlerFunc {
	return httpmw.Authorize(d.Logger, d.Authorizer, permissions...)
}

func logFrom(c *gin.Context, d Deps) *slog.Logger {
	return observability.From(d.Logger, c.Request.Context())
}
