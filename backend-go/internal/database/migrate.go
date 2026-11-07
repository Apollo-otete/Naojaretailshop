package database

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // registers the postgres:// driver
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/migrations"
)

// Migrator runs the versioned SQL in backend-go/migrations. §16 requires every schema change
// to be reversible, which golang-migrate enforces structurally: an .up.sql with no .down.sql
// is a migration that cannot be applied to a dirty database, and the pair is reviewed together
// because a down that does not restore the previous state is a rollback that does not exist.
type Migrator struct {
	dsn string
	l   *slog.Logger
}

func NewMigrator(dsn string, l *slog.Logger) *Migrator {
	return &Migrator{dsn: dsn, l: l}
}

func (m *Migrator) open() (*migrate.Migrate, error) {
	src, err := iofs.New(migrations.FS, "sql")
	if err != nil {
		return nil, fmt.Errorf("opening embedded migrations: %w", err)
	}
	mg, err := migrate.NewWithSourceInstance("iofs", src, m.dsn)
	if err != nil {
		return nil, fmt.Errorf("preparing migrator: %w", err)
	}
	// golang-migrate reports each step through its own Logger interface; routing it to slog
	// keeps migrations readable in the same stream as the service instead of shouting over
	// request logs on stderr.
	mg.Log = &logAdapter{l: m.l}
	return mg, nil
}

// Up applies every pending migration and reports the resulting version.
func (m *Migrator) Up() (version uint, dirty bool, err error) {
	mg, err := m.open()
	if err != nil {
		return 0, false, err
	}
	defer closeMigrator(m.l, mg)

	if err := mg.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		v, d, _ := m.readVersion(mg)
		return v, d, fmt.Errorf("migrating up: %w", err)
	}
	return m.readVersion(mg)
}

// Down rolls back exactly `steps` migrations. A zero or negative count is refused rather than
// silently treated as "everything" — a mis-typed `0` that tears down a whole schema is the
// kind of failure an operational tool should not be able to cause. Use DownAll for that.
func (m *Migrator) Down(steps int) (version uint, dirty bool, err error) {
	if steps < 1 {
		return 0, false, fmt.Errorf("down requires a positive step count, got %d", steps)
	}
	mg, err := m.open()
	if err != nil {
		return 0, false, err
	}
	defer closeMigrator(m.l, mg)

	if err := mg.Steps(-steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		v, d, _ := m.readVersion(mg)
		return v, d, fmt.Errorf("rolling back %d steps: %w", steps, err)
	}
	return m.readVersion(mg)
}

// DownAll rolls back every applied migration, reaching the empty database that migration 0001
// starts from. It is the operation the Phase 2 exit criteria are really testing: a schema you
// cannot undo from end to end is a schema you cannot safely change.
func (m *Migrator) DownAll() (version uint, dirty bool, err error) {
	mg, err := m.open()
	if err != nil {
		return 0, false, err
	}
	defer closeMigrator(m.l, mg)

	if err := mg.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		v, d, _ := m.readVersion(mg)
		return v, d, fmt.Errorf("rolling back all migrations: %w", err)
	}
	return m.readVersion(mg)
}

// Force records a version as applied without executing its SQL. It exists to unstick a
// database left dirty by a migration that partially ran — the situation where "roll it back"
// is not available — and is therefore reachable only from the CLI behind an explicit -yes.
func (m *Migrator) Force(version uint) error {
	mg, err := m.open()
	if err != nil {
		return err
	}
	defer closeMigrator(m.l, mg)
	return mg.Force(int(version))
}

// Pending lists embedded migration versions that have not been applied yet. golang-migrate
// exposes no "what is left to run" call, so this compares the embedded set against the
// database's recorded version — the same question `make migrate-status` answers for a human
// before anything is changed, and the reason AUTO_MIGRATE can log its intent instead of
// guessing.
func (m *Migrator) Pending() ([]uint, error) {
	embedded, err := embeddedVersions()
	if err != nil {
		return nil, err
	}
	mg, err := m.open()
	if err != nil {
		return nil, err
	}
	defer closeMigrator(m.l, mg)

	cur, _, err := m.readVersion(mg)
	if err != nil {
		return nil, err
	}
	var out []uint
	for _, v := range embedded {
		if v > cur {
			out = append(out, v)
		}
	}
	return out, nil
}

// Version reports the database's current migration version and whether it is dirty.
func (m *Migrator) Version() (version uint, dirty bool, err error) {
	mg, err := m.open()
	if err != nil {
		return 0, false, err
	}
	defer closeMigrator(m.l, mg)
	return m.readVersion(mg)
}

// readVersion normalises migrate.ErrNilVersion — "no migration has ever run here", a fresh
// database and not an error the caller should have to special-case.
func (m *Migrator) readVersion(mg *migrate.Migrate) (uint, bool, error) {
	v, dirty, err := mg.Version()
	switch {
	case err == nil:
		return v, dirty, nil
	case errors.Is(err, migrate.ErrNilVersion):
		return 0, false, nil
	default:
		return 0, dirty, err
	}
}

func embeddedVersions() ([]uint, error) {
	entries, err := fs.ReadDir(migrations.FS, "sql")
	if err != nil {
		return nil, fmt.Errorf("reading embedded migrations: %w", err)
	}
	seen := map[uint]bool{}
	var out []uint
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		v, err := strconv.ParseUint(strings.SplitN(name, "_", 2)[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("migration %q does not start with a numeric version", name)
		}
		u := uint(v)
		if seen[u] {
			return nil, fmt.Errorf("two migrations claim version %d", u)
		}
		seen[u] = true
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	if len(out) == 0 {
		return nil, errors.New("no .up.sql migrations embedded")
	}
	return out, nil
}

// closeMigrator reports both errors: migrate separates a source close failure from a database
// close failure, and swallowing the second one is how a leaked connection disappears.
func closeMigrator(l *slog.Logger, mg *migrate.Migrate) {
	srcErr, dbErr := mg.Close()
	if srcErr != nil {
		l.Error("closing migration source", slog.String("error", srcErr.Error()))
	}
	if dbErr != nil {
		l.Error("closing migration database handle", slog.String("error", dbErr.Error()))
	}
}

type logAdapter struct{ l *slog.Logger }

func (a *logAdapter) Printf(format string, v ...any) {
	a.l.Info("migrate", slog.String("detail", strings.TrimSpace(fmt.Sprintf(format, v...))))
}

func (a *logAdapter) Verbose() bool { return false }
