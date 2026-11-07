// Package dbtest is the shared setup for the tests that need a live PostgreSQL.
//
// It exists so that one rule — which connection string may be destroyed — is written once. The
// adapter integration tests truncate real tables and the migration suite drops nine schemas, so
// the rule is a safety property, not a convenience, and a second copy of it is a second copy that
// can be wrong.
//
// This package carries no build tag so that `go vet ./...` covers it; nothing in it does anything
// unless an integration test asks it to.
package dbtest

import (
	"context"
	"log/slog"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/database"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/observability"
)

// Schemas mirrors docs/phase-1-domain-model-erd.md §4: the namespaces this project owns. If a
// migration adds a tenth, the ERD and this list change together.
var Schemas = []string{
	"identity", "catalogue", "commerce", "payments",
	"inventory", "fulfilment", "engagement", "analytics", "administration",
}

// DSN returns a connection string these tests may destroy, or skips the test when there is none.
//
// Resolution order, and why: MIGRATION_TEST_DATABASE_URL exists for CI, which often puts Postgres
// on a service host rather than a loopback address and would be refused by the local-only rule
// below — the flag is the explicit override, not a loophole to widen the heuristic. Failing that,
// a local container is offered on the conventional port, which lets `make test-integration` work
// with no configuration at all.
func DSN(t *testing.T) string {
	t.Helper()

	if raw := os.Getenv("MIGRATION_TEST_DATABASE_URL"); raw != "" {
		// Even an explicitly supplied URL has to name a scratch database.
		requireScratchName(t, raw)
		return raw
	}
	for _, candidate := range []string{
		os.Getenv("DATABASE_URL"),
		"postgres://naoja:naoja_dev_only@localhost:5433/naoja_test?sslmode=disable",
		"postgres://naoja:naoja_dev_only@localhost:5432/naoja_test?sslmode=disable",
		"postgres://postgres:postgres@localhost:5432/naoja_test?sslmode=disable",
		"postgres://postgres@localhost:5432/naoja_test?sslmode=disable",
	} {
		if candidate == "" || !strings.HasSuffix(strings.Split(candidate, "?")[0], "_test") {
			continue
		}
		conn, err := pgx.Connect(context.Background(), candidate)
		if err != nil {
			continue // not up, not reachable, wrong credentials — try the next one
		}
		_ = conn.Close(context.Background())
		return candidate
	}
	t.Skip("no reachable scratch database (one named *_test); run `make db-up` first")
	return ""
}

// requireScratchName refuses a connection string that could point at anything with real data in
// it. The failure is a t.Fatalf rather than an error return because the alternative is a test that
// "passed" while connected to the production shop.
func requireScratchName(t *testing.T, raw string) {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("MIGRATION_TEST_DATABASE_URL does not parse: %v", err)
	}
	if !strings.HasSuffix(strings.Trim(u.Path, "/"), "_test") {
		t.Fatalf("refusing to run destructive migrations against database %q — its name must end in _test", u.Path)
	}
}

// Redact strips credentials from a DSN so a connection failure can be reported without pasting a
// password into the test output, which is a file, which ends up in a CI log.
func Redact(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return "<unparseable>"
	}
	if _, ok := u.User.Password(); ok {
		u.User = url.UserPassword(u.User.Username(), "REDACTED")
	}
	return u.String()
}

// Logger is the quiet logger the setup paths want: a test is not interested in the pool's opinion
// of itself.
func Logger() *slog.Logger { return observability.NewLogger(slog.LevelError) }

// Migrate applies every pending migration and fails if the database is left dirty. It does not roll
// anything back: the suite that proves the down half lives in internal/database, and dropping nine
// schemas between packages would only make the other tests fail for an unrelated reason.
func Migrate(t *testing.T) {
	t.Helper()

	raw := DSN(t)
	version, dirty, err := database.NewMigrator(raw, Logger()).Up()
	if err != nil {
		t.Fatalf("migrating %s up: %v", Redact(raw), err)
	}
	if dirty {
		t.Fatalf("database is dirty at version %d after migration; run `make db-reset`", version)
	}
}

// Pool returns a connected pool with the full schema applied, closed at cleanup.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	raw := DSN(t)
	Migrate(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg, err := pgxpool.ParseConfig(raw)
	if err != nil {
		t.Fatalf("parsing %s: %v", Redact(raw), err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("creating pool for %s: %v", Redact(raw), err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("pinging %s: %v", Redact(raw), err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Conn returns a single connection with the full schema applied, closed at cleanup. It exists for
// assertions that have to see the same server state the adapter wrote — an unreadable session
// token, an audit row that refuses to be updated.
func Conn(t *testing.T) *pgx.Conn {
	t.Helper()

	raw := DSN(t)
	Migrate(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatalf("connecting to %s: %v", Redact(raw), err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}

// qualifiedTable is a schema-qualified name and nothing else. Table names reach the statement as
// text because TRUNCATE takes a list, not a parameter — so the shape is checked rather than
// trusted, even though every caller here is a test passing a literal.
var qualifiedTable = regexp.MustCompile(`^[a-z_]+(\.[a-z_]+)?$`)

// Truncate empties the named tables and everything that references them, leaving the seeded
// catalogue in place. Every adapter test needs a starting point it can count rows against, and
// "counted the rows a previous test left behind" is the most common way an integration test lies.
func Truncate(t *testing.T, pool *pgxpool.Pool, tables ...string) {
	t.Helper()
	if len(tables) == 0 {
		t.Fatal("Truncate called with no tables")
	}
	for _, tbl := range tables {
		if !qualifiedTable.MatchString(tbl) {
			t.Fatalf("refusing to truncate %q: expected schema.table of lowercase letters and underscores", tbl)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// RESTART IDENTITY because the tests assert on generated ids, and CASCADE because
	// identity.users is referenced by sessions, grants and the audit log: truncating the root
	// without it fails rather than cleans.
	if _, err := pool.Exec(ctx,
		"TRUNCATE TABLE "+strings.Join(tables, ", ")+" RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncating %v: %v", tables, err)
	}
}
