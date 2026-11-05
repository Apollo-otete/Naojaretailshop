// Package server assembles the Gin engine. It is the only package that knows both the
// transport middleware and the dependency graph, which is what makes "Gin must not contain
// business rules" (§8) checkable by eye: if a rule appears here, it is in the wrong place.
package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/config"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpjson"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpmw"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/observability"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
)

type Deps struct {
	Logger     *slog.Logger
	Cfg        config.Config
	Authorizer port.Authorizer
	Checkers   []port.Checker
	Version    string
	StartedAt  time.Time
}

// New builds the engine without starting a listener, so the whole chain — request id, CORS,
// recovery, authorizer, error envelope — can be exercised by httptest with no port involved.
func New(d Deps) *gin.Engine {
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
	r.Use(httpmw.CORS(d.Logger, d.Cfg.CORS.AllowedOrigins))

	registerHealth(r, d)

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

	return r
}

// Require registers a protected operation. The permission keys sit next to the route so the
// contract's x-permissions list and the enforcement stay one edit apart.
func Require(d Deps, permissions ...string) gin.HandlerFunc {
	return httpmw.Authorize(d.Logger, d.Authorizer, permissions...)
}

func logFrom(c *gin.Context, d Deps) *slog.Logger {
	return observability.From(d.Logger, c.Request.Context())
}
