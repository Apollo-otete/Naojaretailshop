// Command migrate is the operational entry point for schema changes: apply, roll back, or
// report. It reads only DATABASE_URL (plus LOG_LEVEL), because a migration tool that fails on a
// missing CORS origin list is a migration tool that gets skipped.
//
// Usage:
//
//	migrate up                 apply every pending migration
//	migrate down [n]           roll back n migrations (default 1)
//	migrate down -all          roll back everything
//	migrate status             report current version, dirtiness and what is pending
//	migrate force <version>    mark a version applied without running it (-yes required)
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/config"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/database"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/observability"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	// Flags and positionals are separated before parsing so `migrate down -all` and
	// `migrate -all down` mean the same thing. Go's flag package stops at the first
	// positional, which would otherwise make `-yes` unreachable in the reading people
	// naturally reach for.
	flags, positional := splitFlags(args)

	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	all := fs.Bool("all", false, "with down: roll back every applied migration")
	yes := fs.Bool("yes", false, "confirm a destructive recovery operation (force)")
	fs.Usage = func() { fmt.Fprintln(os.Stderr, usageString) }

	if len(positional) == 0 {
		fs.Usage()
		return fmt.Errorf("no command given")
	}
	if err := fs.Parse(flags); err != nil {
		return err
	}

	verb := positional[0]
	if verb == "-h" || verb == "--help" || verb == "help" {
		fs.Usage()
		return nil
	}
	rest := positional[1:]

	// The same .env the API reads, so `make migrate-up` works wherever `make run` does. Load
	// is deliberately not used: a missing CORS list must not stop a schema being applied.
	if err := config.LoadDotenv(".env"); err != nil {
		return fmt.Errorf("loading .env: %w", err)
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL is not set (export it or put it in .env)")
	}

	l := observability.NewLogger(envLevel())
	m := database.NewMigrator(dsn, l)

	switch verb {
	case "up":
		version, dirty, err := m.Up()
		if err != nil {
			return err
		}
		fmt.Printf("applied; now at version %d (dirty=%t)\n", version, dirty)

	case "down":
		if *all {
			version, dirty, err := m.DownAll()
			if err != nil {
				return err
			}
			fmt.Printf("rolled back every migration; now at version %d (dirty=%t)\n", version, dirty)
			fmt.Println("verify: make migrate-up && make test-integration")
			return nil
		}
		steps, err := resolveSteps(rest)
		if err != nil {
			return err
		}
		version, dirty, err := m.Down(steps)
		if err != nil {
			return err
		}
		fmt.Printf("rolled back %d step(s); now at version %d (dirty=%t)\n", steps, version, dirty)
		fmt.Println("verify: make migrate-up && make test-integration")

	case "status":
		version, dirty, err := m.Version()
		if err != nil {
			return err
		}
		fmt.Printf("current version: %d (dirty=%t)\n", version, dirty)
		if dirty {
			fmt.Println("a migration failed part-way; inspect it, then `migrate -yes force <version>` only if the database is in the state that migration's up SQL leaves behind")
		}
		pending, err := m.Pending()
		if err != nil {
			return err
		}
		if len(pending) == 0 {
			fmt.Println("database is at the latest migration")
			return nil
		}
		fmt.Printf("pending: %v\n", pending)
		return fmt.Errorf("%d migration(s) not yet applied", len(pending))

	case "force":
		if !*yes {
			return fmt.Errorf("force marks a version as applied without running its SQL; refusing without -yes")
		}
		if len(rest) != 1 {
			return fmt.Errorf("force needs one version number")
		}
		v, err := strconv.ParseUint(rest[0], 10, 64)
		if err != nil {
			return fmt.Errorf("version %q: %w", rest[0], err)
		}
		if err := m.Force(uint(v)); err != nil {
			return err
		}
		fmt.Printf("marked version %d applied without executing its SQL\n", v)

	default:
		fs.Usage()
		return fmt.Errorf("unknown command %q", verb)
	}

	return nil
}

func splitFlags(args []string) (flags, positional []string) {
	for _, a := range args {
		if len(a) > 1 && a[0] == '-' {
			flags = append(flags, a)
			continue
		}
		positional = append(positional, a)
	}
	return flags, positional
}

func resolveSteps(args []string) (int, error) {
	if len(args) == 0 {
		return 1, nil
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return 0, fmt.Errorf("step count %q: %w", args[0], err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("step count must be positive; use -all to roll everything back")
	}
	return n, nil
}

func envLevel() slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(os.Getenv("LOG_LEVEL"))); err != nil {
		return slog.LevelInfo
	}
	return l
}

const usageString = `usage: migrate <command> [arguments] [flags]

  up                 apply every pending migration
  down [n]           roll back n migrations (default 1)
  down -all          roll back every applied migration
  status             report current version, dirtiness and what is still pending (exit 1 if pending)
  force <version>    mark a version applied without running its SQL (needs -yes)

Flags may appear before or after the command: ` + "`migrate down -all`" + ` and
` + "`migrate -all down`" + ` are the same invocation.

Environment: DATABASE_URL (required), LOG_LEVEL (default info).
The SQL is embedded from migrations/sql, so this binary carries the whole history.`
