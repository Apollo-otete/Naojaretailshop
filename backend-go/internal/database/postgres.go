// Package database owns the PostgreSQL connection pool and the migration pipeline. Postgres
// is the only database in the target architecture (ADR-001); there is deliberately no second
// engine to keep in sync.
package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/config"
)

// Postgres wraps the pool so the health port can be satisfied without every caller knowing
// what a pool is.
type Postgres struct {
	pool       *pgxpool.Pool
	pingBudget time.Duration
}

// New builds the pool. It does not connect: pgx dials lazily, so the process can start and
// serve /healthz while the database is still coming up — /readyz is what reports the truth.
func New(ctx context.Context, cfg config.Database, l *slog.Logger) (*Postgres, error) {
	pcfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parsing DATABASE_URL: %w", err)
	}
	pcfg.MaxConns = cfg.MaxConns
	pcfg.MinConns = cfg.MinConns
	// A dead connection discovered mid-request is worse than a slightly cold pool: this is a
	// shop with bursty traffic and long idle stretches.
	pcfg.HealthCheckPeriod = time.Minute
	pcfg.MaxConnLifetime = time.Hour
	pcfg.MaxConnIdleTime = 15 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("creating pool: %w", err)
	}

	l.Info("postgres pool configured",
		slog.String("dsn", cfg.RedactedURL()),
		slog.Int("max_conns", int(cfg.MaxConns)),
		slog.Int("min_conns", int(cfg.MinConns)),
	)

	return &Postgres{pool: pool, pingBudget: cfg.PingBudget}, nil
}

func (p *Postgres) Pool() *pgxpool.Pool { return p.pool }

func (p *Postgres) Close() { p.pool.Close() }

// Name identifies this dependency in the readiness payload.
func (p *Postgres) Name() string { return "postgresql" }

// Check satisfies port.Checker. The budget is the pool's ping plus a timeout, because a
// readiness probe that hangs is indistinguishable from one that failed — and it holds the
// whole request open while it does nothing.
func (p *Postgres) Check(ctx context.Context) error {
	if p == nil || p.pool == nil {
		return errors.New("pool is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, p.pingBudget)
	defer cancel()

	start := time.Now()
	if err := p.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping failed after %s: %w", time.Since(start).Round(time.Millisecond), err)
	}
	return nil
}
