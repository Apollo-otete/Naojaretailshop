// Package migrations embeds the versioned SQL so the service can migrate a database without
// shipping the files alongside the binary. The same directory is readable by the
// golang-migrate CLI (migrate -path migrations/sql -database "$DATABASE_URL" up) — the embed
// is for convenience in containers, not a lock-in.
package migrations

import "embed"

// FS holds every .sql file under sql/. All of them are embedded because a partial set is how
// an environment ends up half-migrated with no down script available.
//
//go:embed sql/*.sql
var FS embed.FS
