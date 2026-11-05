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

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/authz"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/config"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/database"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/observability"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/server"
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

	deps := server.Deps{
		Logger:    l,
		Cfg:       cfg,
		Version:   version,
		StartedAt: startedAt,
		Checkers:  []port.Checker{pg},
		// Phase 3 swaps this for the RBAC evaluator. Deny-by-default is not a placeholder to
		// be embarrassed about; it is the state §10 requires when no authorizer is known.
		Authorizer: authz.DenyAll{},
	}

	srv := &http.Server{
		Addr:              cfg.Address,
		Handler:           server.New(deps),
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
