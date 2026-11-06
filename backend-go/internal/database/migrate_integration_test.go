//go:build integration

// This file is the external test package on purpose: the DSN rule that decides which database may
// be destroyed lives in internal/dbtest, and dbtest imports this package to run the migrations.
// An in-package test could not import it without a cycle, and the alternative — a second copy of
// the safety rule — is the thing to avoid.
package database_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/database"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/dbtest"
)

// The migration pipeline is only worth having if the up and the down are both proven against a
// real server. Everything else in this repository's test suite runs without a database; this
// file is the reason `make test-integration` exists as a separate target.
//
// It is destructive by design — `DownAll` drops nine schemas — which is why it shares dbtest's
// scratch-database rule rather than inventing its own, and why `make test-integration` runs the
// packages one at a time.

func migrator(t *testing.T) *database.Migrator {
	t.Helper()
	return database.NewMigrator(dbtest.DSN(t), dbtest.Logger())
}

// reset tears the database down to nothing. A failure here almost always means the recorded
// version is not in the embedded set — a hand-edited or half-forced database — and `make
// db-reset` is the honest fix, so the message says so.
func reset(t *testing.T, m *database.Migrator) {
	t.Helper()
	if _, _, err := m.DownAll(); err != nil {
		t.Fatalf("cannot reach an empty database: %v\n"+
			"the recorded version is probably not in migrations/sql; run `make db-reset`", err)
	}
}

func TestMigrationsRunUpAndBackDownAgainstPostgres(t *testing.T) {
	conn := dbtest.Conn(t)
	m := migrator(t)

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

	// Leaving the database empty would make every other integration package in this run migrate
	// from nothing, which is correct but slow — and it means the next assertion in this file has
	// a schema to assert against.
	if _, _, err := m.Up(); err != nil {
		t.Fatalf("restoring: %v", err)
	}
}

// A single-step rollback is the operation a developer performs under pressure, so it has to be
// exercised rather than assumed.
func TestDownOneStepRollsBackExactlyOneMigration(t *testing.T) {
	conn := dbtest.Conn(t)
	m := migrator(t)

	reset(t, m)
	if _, _, err := m.Up(); err != nil {
		t.Fatalf("up: %v", err)
	}

	_, _, err := m.Down(1)
	if err != nil {
		t.Fatalf("down 1: %v", err)
	}

	head := tableExists(t, conn, "identity", "users")
	if head {
		t.Error("identity.users survived a one-step rollback of the head migration")
	}
	// system_settings belongs to the migration below it, which was not rolled back. When a 0004
	// lands this assertion is the one that starts failing, and the fix is to move both checks one
	// migration forward rather than to delete them.
	if !tableExists(t, conn, "administration", "system_settings") {
		t.Error("a one-step rollback removed a table it did not own")
	}
	// The schemas belong to migration 0001, which was not rolled back.
	assertSchemas(t, conn, true)

	if _, _, err := m.Up(); err != nil {
		t.Fatalf("recovering: %v", err)
	}
}

func tableExists(t *testing.T, conn *pgx.Conn, schema, table string) bool {
	t.Helper()
	var exists bool
	if err := conn.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables
		 WHERE table_schema = $1 AND table_name = $2)`, schema, table).Scan(&exists); err != nil {
		t.Fatalf("checking %s.%s: %v", schema, table, err)
	}
	return exists
}

// A zero step count must not mean "everything".
func TestDownRefusesANonPositiveStepCount(t *testing.T) {
	m := migrator(t)
	for _, steps := range []int{0, -1, -99} {
		if _, _, err := m.Down(steps); err == nil {
			t.Errorf("Down(%d) was accepted", steps)
		}
	}
}

func assertSchemas(t *testing.T, conn *pgx.Conn, want bool) {
	t.Helper()
	ctx := context.Background()
	for _, s := range dbtest.Schemas {
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
		dbtest.Schemas).Scan(&n)
	if err != nil {
		t.Fatalf("counting tables: %v", err)
	}
	return n
}
