//go:build integration

package database

import (
	"context"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/observability"
)

// The migration pipeline is only worth having if the up and the down are both proven against a
// real server. Everything else in this repository's test suite runs without a database; this
// file is the reason `make test-integration` exists as a separate target.
//
// It is destructive by design — `DownAll` drops nine schemas — so it refuses any connection
// string that does not look like a scratch database.

// schemas mirrors docs/phase-1-domain-model-erd.md §4. If a migration adds a tenth, the ERD and
// this list change together.
var schemas = []string{
	"identity", "catalogue", "commerce", "payments",
	"inventory", "fulfilment", "engagement", "analytics", "administration",
}

// scratchDSN returns a connection string these tests may destroy, or "" when there is none.
//
// Resolution order, and why: MIGRATION_TEST_DATABASE_URL exists for CI, which often puts
// Postgres on a service host rather than a loopback address and would be refused by the
// local-only rule below — the flag is the explicit override, not a loophole to widen the
// heuristic. Failing that, a local container is offered on the conventional port, which lets
// `make test-integration` work with no configuration at all.
func scratchDSN(t *testing.T) string {
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
	return ""
}

// These tests drop every schema they create, so a connection string that could point at
// anything with real data in it is refused here rather than discovered at the worst moment.
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

var dsnOnce struct {
	value string
	done  bool
}

func dsn(t *testing.T) string {
	t.Helper()
	if !dsnOnce.done {
		dsnOnce.value = scratchDSN(t)
		dsnOnce.done = true
	}
	if dsnOnce.value == "" {
		t.Skip("no reachable scratch database (one named *_test); run `make db-up` first")
	}
	return dsnOnce.value
}

func connect(t *testing.T) *pgx.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn(t))
	if err != nil {
		t.Fatalf("connecting to %s: %v", redact(dsn(t)), err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}

func redact(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return "<unparseable>"
	}
	if _, ok := u.User.Password(); ok {
		u.User = url.UserPassword(u.User.Username(), "REDACTED")
	}
	return u.String()
}

func TestMigrationsRunUpAndBackDownAgainstPostgres(t *testing.T) {
	conn := connect(t)
	m := NewMigrator(dsn(t), testLog())

	ctx := context.Background()

	// Start from a clean slate so the assertions below are about these migrations and not
	// about whatever the last run left behind.
	reset(t, m)

	pending, err := m.Pending()
	if err != nil {
		t.Fatalf("pending before up: %v", err)
	}
	if len(pending) == 0 {
		t.Fatal("an empty database should report every migration as pending")
	}

	version, dirty, err := m.Up()
	if err != nil {
		t.Fatalf("up: %v", err)
	}
	if dirty {
		t.Fatal("database is dirty after a clean up — a migration failed part-way")
	}
	if want := pending[len(pending)-1]; version != want {
		t.Fatalf("version = %d after applying %v, want %d", version, pending, want)
	}

	assertSchemas(t, conn, true)

	// system_settings exists and is seeded: this is the proof that the SQL embedded in the
	// binary is the SQL that ran, which is the whole reason for //go:embed in the first place.
	var seeded int
	if err := conn.QueryRow(ctx,
		`SELECT count(*) FROM administration.system_settings WHERE ratified IS NOT NULL`).Scan(&seeded); err != nil {
		t.Fatalf("reading the seed data: %v", err)
	}
	if seeded == 0 {
		t.Error("system_settings was created but not seeded")
	}

	// The unratified delivery terms must arrive marked as unratified, or a Phase 5 checkout
	// reads KSh 150 as an approved price.
	var ratified bool
	err = conn.QueryRow(ctx,
		`SELECT ratified FROM administration.system_settings WHERE key = 'delivery_standard_fee_cents'`).Scan(&ratified)
	if err != nil {
		t.Fatalf("ADR-018 is unresolved, so its placeholder row must exist: %v", err)
	}
	if ratified {
		t.Error("delivery_standard_fee_cents is seeded as ratified=true while ADR-018 is still open")
	}

	// A second Up must be a no-op, because AUTO_MIGRATE and every deploy step call it blindly.
	before := countTables(t, conn)
	v2, _, err := m.Up()
	if err != nil {
		t.Fatalf("re-applying: %v", err)
	}
	if v2 != version {
		t.Errorf("second Up moved the version %d -> %d", version, v2)
	}
	if after := countTables(t, conn); after != before {
		t.Errorf("second Up changed the table count %d -> %d", before, after)
	}

	// And the rollback has to actually undo it — §16's reversibility requirement is only real
	// if it is checked against a server that can say no.
	if _, _, err := m.DownAll(); err != nil {
		t.Fatalf("down -all: %v", err)
	}
	assertSchemas(t, conn, false)
	if got := countTables(t, conn); got != 0 {
		t.Errorf("%d tables survived a full rollback", got)
	}
}

// A single-step rollback is the operation a developer performs under pressure, so it has to be
// exercised rather than assumed.
func TestDownOneStepRollsBackExactlyOneMigration(t *testing.T) {
	conn := connect(t)
	m := NewMigrator(dsn(t), testLog())
	ctx := context.Background()

	reset(t, m)
	if _, _, err := m.Up(); err != nil {
		t.Fatalf("up: %v", err)
	}

	_, _, err := m.Down(1)
	if err != nil {
		t.Fatalf("down 1: %v", err)
	}

	var exists bool
	if err := conn.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables
		 WHERE table_schema = 'administration' AND table_name = 'system_settings')`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("the last migration's table survived a one-step rollback")
	}
	// The schemas belong to migration 0001, which was not rolled back.
	assertSchemas(t, conn, true)

	if _, _, err := m.Up(); err != nil {
		t.Fatalf("recovering: %v", err)
	}
}

// A zero step count must not mean "everything".
func TestDownRefusesANonPositiveStepCount(t *testing.T) {
	m := NewMigrator(dsn(t), testLog())
	for _, steps := range []int{0, -1, -99} {
		if _, _, err := m.Down(steps); err == nil {
			t.Errorf("Down(%d) was accepted", steps)
		}
	}
}

func testLog() *slog.Logger {
	return observability.NewLogger(slog.LevelWarn)
}

// reset tears the database down to nothing. A failure here almost always means the recorded
// version is not in the embedded set — a hand-edited or half-forced database — and `make
// db-reset` is the honest fix, so the message says so.
func reset(t *testing.T, m *Migrator) {
	t.Helper()
	if _, _, err := m.DownAll(); err != nil {
		t.Fatalf("cannot reach an empty database: %v\n"+
			"the recorded version is probably not in migrations/sql; run `make db-reset`", err)
	}
}

func assertSchemas(t *testing.T, conn *pgx.Conn, want bool) {
	t.Helper()
	ctx := context.Background()
	for _, s := range schemas {
		var exists bool
		if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = $1)`, s).Scan(&exists); err != nil {
			t.Fatalf("checking schema %q: %v", s, err)
		}
		if exists != want {
			t.Errorf("schema %q exists = %t, want %t", s, exists, want)
		}
	}
}

func countTables(t *testing.T, conn *pgx.Conn) int {
	t.Helper()
	var n int
	err := conn.QueryRow(context.Background(), `
		SELECT count(*) FROM information_schema.tables
		 WHERE table_schema = ANY($1::text[]) AND table_type = 'BASE TABLE'`,
		schemas).Scan(&n)
	if err != nil {
		t.Fatalf("counting tables: %v", err)
	}
	return n
}
