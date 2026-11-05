package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/config"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/database"
)

// migrateIfNeeded applies pending migrations only when explicitly asked.
//
// The default is off. A process that migrates a shared database on every boot makes an
// accidental rollout of an unreviewed schema change possible, and in a deployment with more
// than one replica the replicas race each other. Migrations belong to the deploy step
// (`make migrate-up` or cmd/migrate), which is also the step that can pause and take a backup.
//
// Whatever the setting, the count of pending migrations is reported: silence about an unmigrated
// database is how "the API is up but every read fails" becomes a debugging session.
func migrateIfNeeded(cfg config.Config, l *slog.Logger) error {
	m := database.NewMigrator(cfg.Database.URL, l)

	pending, err := m.Pending()
	if err != nil {
		l.Warn("could not read migration state", slog.Any("error", err))
		return nil
	}

	if len(pending) > 0 {
		l.Info("pending migrations", slog.Any("versions", pending), slog.String("action", "run: make migrate-up"))
	} else {
		l.Debug("database is at the latest migration")
	}

	on, err := strconv.ParseBool(envOr("AUTO_MIGRATE", "false"))
	if err != nil || !on {
		return nil
	}

	version, dirty, err := m.Up()
	if err != nil {
		return fmt.Errorf("auto-migration: %w", err)
	}
	l.Info("migrations applied",
		slog.Any("version", version),
		slog.Bool("dirty", dirty),
	)
	return nil
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
