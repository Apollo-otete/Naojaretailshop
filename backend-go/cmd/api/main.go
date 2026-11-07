// Command api runs the Naoja HTTP API.
//
// It is the composition root and the only place that knows every concrete adapter at once:
// configuration, logger, pool, migrator, authorizer, engine. Nothing below this package
// imports anything beside it.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	auditpg "github.com/Apollo-otete/Naojaretailshop/backend-go/internal/audit/postgres"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/authz"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/catalogue"
	cataloguepg "github.com/Apollo-otete/Naojaretailshop/backend-go/internal/catalogue/postgres"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/config"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/database"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpauth"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	identitypg "github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity/postgres"
	inventorypg "github.com/Apollo-otete/Naojaretailshop/backend-go/internal/inventory/postgres"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/observability"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/ordering"
	orderingpg "github.com/Apollo-otete/Naojaretailshop/backend-go/internal/ordering/postgres"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/payments"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/payments/daraja"
	paymentspg "github.com/Apollo-otete/Naojaretailshop/backend-go/internal/payments/postgres"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/server"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/shopping"
	shoppingpg "github.com/Apollo-otete/Naojaretailshop/backend-go/internal/shopping/postgres"
)

// version is replaced at build time: -ldflags "-X main.version=$(git describe --tags)".
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	l := observability.NewLogger(cfg.LogLevel)
	startedAt := time.Now().UTC()

	l.Info("starting naoja api",
		slog.String("version", version),
		slog.String("env", cfg.Env),
		slog.String("addr", cfg.Address),
		slog.String("log_level", cfg.LogLevel.String()),
		slog.Any("allowed_origins", cfg.CORS.AllowedOrigins),
		slog.String("session_cookie", cfg.SessionCookieName),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pg, err := database.New(ctx, cfg.Database, l)
	if err != nil {
		return err
	}
	defer pg.Close()

	if err := migrateIfNeeded(cfg, l); err != nil {
		return err
	}

	// The identity adapters are one Store because they are one schema: users, sessions and the
	// grant tables are read together on every request, and splitting them across types would
	// only offer two places to get the transaction boundary wrong.
	identStore, err := identitypg.New(pg.Pool())
	if err != nil {
		return fmt.Errorf("identity store: %w", err)
	}
	auditWriter, err := auditpg.New(pg.Pool())
	if err != nil {
		return fmt.Errorf("audit writer: %w", err)
	}

	// NewService spends one Argon2id hash here, on purpose: it is the path an unknown email
	// takes at login, and hashing it now means a timing oracle cannot be configured into
	// existence later.
	ident, err := identity.NewService(identity.Deps{
		Users:    identStore,
		Sessions: identStore,
		Grants:   identStore,
		Tx:       identitypg.NewTx(pg.Pool()),
		Audit:    auditWriter,
		Hasher:   identity.NewHasher(),
		Policy: identity.Policy{
			Idle:      cfg.Session.Idle,
			AdminIdle: cfg.Session.AdminIdle,
			Absolute:  cfg.Session.Absolute,
		},
		Login:  identity.LoginPolicy{MaxFailedAttempts: cfg.Login.MaxFailures, Lockout: cfg.Login.Lockout},
		Logger: l,
	})
	if err != nil {
		return fmt.Errorf("identity service: %w", err)
	}

	// Catalogue adapters: one store over the catalogue schema, plus the shared transactor
	// and audit writer already wired for identity.
	catStore, err := cataloguepg.New(pg.Pool())
	if err != nil {
		return fmt.Errorf("catalogue store: %w", err)
	}
	cat, err := catalogue.NewService(catalogue.Deps{
		Categories: catStore,
		Products:   catStore,
		Tx:         cataloguepg.NewTx(pg.Pool()),
		Audit:      auditWriter,
		Clock:      time.Now,
		Logger:     l,
	})
	if err != nil {
		return fmt.Errorf("catalogue service: %w", err)
	}

	// Shopping adapters.
	shopStore, err := shoppingpg.New(pg.Pool())
	if err != nil {
		return fmt.Errorf("shopping store: %w", err)
	}
	shop, err := shopping.NewService(shopping.Deps{
		Carts:      shopStore,
		Products:   catStore,
		Authorizer: authz.RBAC{Logger: l},
		Tx:         shoppingpg.NewTx(pg.Pool()),
		Audit:      auditWriter,
		Clock:      time.Now,
		Logger:     l,
	})
	if err != nil {
		return fmt.Errorf("shopping service: %w", err)
	}

	// Ordering + P0 checkout adapters (quotes, idempotent placement, stock reservation).
	orderStore, err := orderingpg.New(pg.Pool())
	if err != nil {
		return fmt.Errorf("ordering store: %w", err)
	}
	invStore, err := inventorypg.New(pg.Pool())
	if err != nil {
		return fmt.Errorf("inventory store: %w", err)
	}
	ord, err := ordering.NewService(ordering.Deps{
		Orders:     orderStore,
		Products:   catStore,
		Carts:      shopStore,
		Quotes:     orderStore,
		Idem:       orderStore,
		Settings:   orderStore,
		Inventory:  invStore,
		Authorizer: authz.RBAC{Logger: l},
		Tx:         orderingpg.NewTx(pg.Pool()),
		Audit:      auditWriter,
		Clock:      time.Now,
		Logger:     l,
	})
	if err != nil {
		return fmt.Errorf("ordering service: %w", err)
	}

	// Payments (P1): Daraja/M-Pesa. Safe when unconfigured (endpoints report unavailable).
	payStore, err := paymentspg.New(pg.Pool())
	if err != nil {
		return fmt.Errorf("payments store: %w", err)
	}
	paySvc := payments.NewService(payments.Deps{
		Store:      payStore,
		Orders:     payStore,
		OrderState: payStore,
		Inventory:  invStore,
		Tx:         paymentspg.NewTx(pg.Pool()),
		Daraja:     daraja.New(cfg.Mpesa),
		Clock:      time.Now,
		Logger:     l,
	})

	deps := server.Deps{
		Logger:     l,
		Cfg:        cfg,
		Version:    version,
		StartedAt:  startedAt,
		Checkers:   []port.Checker{pg},
		Authorizer: authz.RBAC{Logger: l},
		// The cookie read is a dependency of the service, not of a route: contracts/openapi.yaml
		// declares sessionCookie at the top level, and B11's "fails on the next request" is only
		// true if the next request is the one that reads the row.
		Sessions: ident,

		// The same service behind both: the routes that issue a session and the middleware that
		// reads one must be answering to one implementation, or a login and a request could
		// disagree about what the same cookie means.
		Identity:   ident,
		CartMerger: shop,
		Catalogue:  cat,
		Shopping:   shop,
		Checkout:   ord,
		Ordering:   ord,
		Payments:   paySvc,

		LegacyOrderStore: orderStore,
		LegacyOrderTx:    orderingpg.NewTx(pg.Pool()),

		// ADR-019's per-address rate limit for the public auth operations, on the numbers the
		// operator configured: enough attempts to stop a rotating attacker, few enough that a person
		// who forgot their password is not locked out of their own account for the day.
		AuthRateLimit: httpauth.NewRateLimiter(cfg.Login.RateLimit, cfg.Login.RateLimitWindow),
	}

	handler, err := server.New(deps)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second, // slowloris: ReadTimeout alone does not cover the header phase
		ReadTimeout:       cfg.HTTPTimeouts.Read,
		WriteTimeout:      cfg.HTTPTimeouts.Write,
		IdleTimeout:       cfg.HTTPTimeouts.Idle,
	}

	// Fail fast if the address cannot be bound. Listening inside the goroutine would report a
	// port clash as a crash after the process has already logged "starting".
	ln, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", cfg.Address, err)
	}

	serveErr := make(chan error, 1)
	go func() {
		l.Info("listening", slog.String("address", ln.Addr().String()))
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		if err != nil {
			return fmt.Errorf("serving: %w", err)
		}
		return nil
	case <-ctx.Done():
		l.Info("shutdown signal received", slog.String("signal", "SIGINT/SIGTERM"))
	}

	// Drain: give in-flight requests a window to finish, then close regardless. A shutdown
	// that waits forever is an outage during a rolling deploy.
	drainCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(drainCtx); err != nil {
		return fmt.Errorf("draining: %w", err)
	}
	l.Info("stopped")
	return nil
}
