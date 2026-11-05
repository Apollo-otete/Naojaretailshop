package migrations_test

import (
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/migrations"
)

// These tests run with no database, which is the point: they catch the mistakes that make a
// migration irreversible or a money column a float, at the stage where fixing them costs
// nothing. §16 requires every schema change to be reversible and ADR-021 requires money to be
// an integer — both are properties of the SQL text, so both are checkable from the text.

type file struct {
	version uint
	name    string // the slug between the number and the direction
	up      string
	down    string
}

func load(t *testing.T) []file {
	t.Helper()
	entries, err := fs.ReadDir(migrations.FS, "sql")
	if err != nil {
		t.Fatalf("reading embedded sql/: %v", err)
	}

	byVersion := map[uint]*file{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		m := regexp.MustCompile(`^(\d{4})_([a-z0-9_]+)\.(up|down)\.sql$`).FindStringSubmatch(e.Name())
		if m == nil {
			t.Errorf("%q does not match NNNN_slug.up|down.sql — golang-migrate will refuse it or mis-sort it", e.Name())
			continue
		}
		v, err := strconv.ParseUint(m[1], 10, 32)
		if err != nil {
			t.Errorf("%q: unreadable version: %v", e.Name(), err)
			continue
		}
		raw, err := migrations.FS.ReadFile("sql/" + e.Name())
		if err != nil {
			t.Errorf("%q: %v", e.Name(), err)
			continue
		}
		f := byVersion[uint(v)]
		if f == nil {
			f = &file{version: uint(v), name: m[2]}
			byVersion[uint(v)] = f
		} else if f.name != m[2] {
			t.Errorf("version %d has two different slugs: %q and %q", uint(v), f.name, m[2])
		}
		if m[3] == "up" {
			f.up = string(raw)
		} else {
			f.down = string(raw)
		}
	}

	out := make([]file, 0, len(byVersion))
	for _, f := range byVersion {
		out = append(out, *f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	if len(out) == 0 {
		t.Fatal("no migrations embedded — the //go:embed pattern is not matching sql/*.sql")
	}
	return out
}

func TestEveryUpHasItsDownAndViceVersa(t *testing.T) {
	for _, f := range load(t) {
		if f.up == "" {
			t.Errorf("%04d_%s has a down script but no up script", f.version, f.name)
		}
		if f.down == "" {
			// golang-migrate cannot roll a version back without this file, so a partially
			// applied migration leaves the database dirty with no way out but `force`.
			t.Errorf("%04d_%s.up.sql has no .down.sql — §16 requires every change to be reversible", f.version, f.name)
		}
	}
}

func TestVersionsAreContiguousStartingAtOne(t *testing.T) {
	files := load(t)
	for i, f := range files {
		if want := uint(i + 1); f.version != want {
			t.Fatalf("migration %d is numbered %d — a gap means someone deleted history that a live database still references", i+1, f.version)
		}
	}
}

func TestScriptsAreNonEmptyAndTerminated(t *testing.T) {
	for _, f := range load(t) {
		for _, pair := range []struct {
			dir, sql string
		}{{"up", f.up}, {"down", f.down}} {
			if pair.dir == "down" && pair.sql == "" {
				continue // reported above
			}
			if strings.TrimSpace(pair.sql) == "" {
				t.Errorf("%04d_%s.%s.sql is empty", f.version, f.name, pair.dir)
			}
			// A file may legitimately end with a closing comment, so the rule is about the last
			// statement rather than the last line.
			if code := stripComments(pair.sql); !strings.HasSuffix(code, ";") {
				t.Errorf("%04d_%s.%s.sql does not end its final statement with a semicolon", f.version, f.name, pair.dir)
			}
			if !strings.HasSuffix(pair.sql, "\n") {
				t.Errorf("%04d_%s.%s.sql has no trailing newline — diffs will show a spurious change", f.version, f.name, pair.dir)
			}
		}
	}
}

// ADR-021: money is a count of cents in an integer column. A NUMERIC here is a silent rounding
// change waiting to happen, and `price_cents NUMERIC` would pass every test except the one
// where a discount stops matching a Daraja amount.
func TestMoneyColumnsAreIntegers(t *testing.T) {
	float := regexp.MustCompile(`(?i)\b(double precision|real|float\d*|numeric\s*\(|decimal\s*\(|money)\b`)

	for _, f := range load(t) {
		for i, line := range strings.Split(f.up, "\n") {
			code := stripComment(line)
			if code == "" {
				continue
			}
			if !strings.Contains(strings.ToLower(code), "_cents") {
				continue
			}
			if loc := float.FindStringIndex(code); loc != nil {
				t.Errorf("%04d_%s.up.sql:%d: a *_cents column uses %q — ADR-021 requires an integer\n\t%s",
					f.version, f.name, i+1, code[loc[0]:loc[1]], strings.TrimSpace(code))
			}
		}
	}
}

// ADR-013: the credential column is named password_hash, and a column simply called `password`
// is where a plaintext value lands by accident — which is what the legacy `admins` table has.
func TestNoColumnIsCalledPlainPassword(t *testing.T) {
	bad := regexp.MustCompile(`(?i)\bpassword\s+(text|varchar|char|citext)`)

	for _, f := range load(t) {
		for i, line := range strings.Split(f.up, "\n") {
			if loc := bad.FindStringIndex(stripComment(line)); loc != nil {
				t.Errorf("%04d_%s.up.sql:%d: a credential column must be password_hash with a hash type: %s",
					f.version, f.name, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// A down script that creates things is a rollback that changes the schema forward. Dropping is
// allowed; so is restoring a default, which is not a CREATE.
func TestDownScriptsDoNotCreateSchema(t *testing.T) {
	create := regexp.MustCompile(`(?i)^\s*create\s+(table|index|type|schema|view|extension|function|trigger)\b`)

	for _, f := range load(t) {
		for i, line := range strings.Split(f.down, "\n") {
			if create.MatchString(stripComment(line)) {
				t.Errorf("%04d_%s.down.sql:%d: a rollback should not %s — make the up script reversible instead\n\t%s",
					f.version, f.name, i+1, strings.Fields(strings.TrimSpace(line))[1], strings.TrimSpace(line))
			}
		}
	}
}

// A migration that truncates or deletes rows destroys data on `make migrate-down`, which is
// meant to be a safe local operation.
func TestNoMigrationDeletesRowsFromItself(t *testing.T) {
	destroy := regexp.MustCompile(`(?i)^\s*(truncate|delete\s+from)\b`)

	for _, f := range load(t) {
		for _, pair := range []struct{ dir, sql string }{{"up", f.up}, {"down", f.down}} {
			for i, line := range strings.Split(pair.sql, "\n") {
				if destroy.MatchString(stripComment(line)) {
					t.Errorf("%04d_%s.%s.sql:%d destroys data: %s",
						f.version, f.name, pair.dir, i+1, strings.TrimSpace(line))
				}
			}
		}
	}
}

// The nine bounded contexts are the domain model's spine (docs/phase-1-domain-model-erd.md).
// A table created in a tenth schema means the ERD was changed without saying so.
func TestOnlyTheDocumentedSchemasAreCreated(t *testing.T) {
	want := map[string]bool{
		"identity": true, "catalogue": true, "commerce": true, "payments": true,
		"inventory": true, "fulfilment": true, "engagement": true, "analytics": true,
		"administration": true,
	}
	created := regexp.MustCompile(`(?i)create\s+schema\s+(?:if\s+not\s+exists\s+)?([a-z_]+)`)

	for _, f := range load(t) {
		for _, m := range created.FindAllStringSubmatch(f.up, -1) {
			if !want[m[1]] {
				t.Errorf("%04d_%s creates schema %q, which is not in the domain model", f.version, f.name, m[1])
			}
		}
	}
}

// Every seeded threshold must say whether the business ratified it, or a placeholder value
// (delivery_standard_fee_cents today) reads as an approved term.
func TestUnratifiedSeedsAreMarkedAsPlaceholders(t *testing.T) {
	for _, f := range load(t) {
		for _, line := range strings.Split(f.up, "\n") {
			code := strings.ToLower(stripComment(line))
			if !strings.Contains(code, "false") || !strings.Contains(code, "'cents'") {
				continue
			}
			if !strings.Contains(code, "placeholder") {
				t.Errorf("a seeded *_cents value with ratified=false must be described as a placeholder so it cannot be read as an approved term:\n\t%s",
					strings.TrimSpace(line))
			}
		}
	}
}

// stripComments removes -- comments and returns the remaining SQL, trailing whitespace cut.
// Block comments are not used in this directory; if one ever is, the line-based rules below
// would need the same treatment.
func stripComments(sql string) string {
	var kept []string
	for _, line := range strings.Split(sql, "\n") {
		if code := strings.TrimSpace(stripComment(line)); code != "" {
			kept = append(kept, code)
		}
	}
	return strings.Join(kept, "\n")
}

func stripComment(line string) string {
	if i := strings.Index(line, "--"); i >= 0 {
		return line[:i]
	}
	return line
}
