package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func valid() Config {
	return Config{
		Env:               "development",
		Address:           ":8080",
		LogLevel:          slog.LevelInfo,
		Database:          Database{URL: "postgres://naoja:s3cr3t-word@localhost:5433/naoja?sslmode=disable", MaxConns: 12, MinConns: 1, PingBudget: 3 * time.Second},
		CORS:              CORS{AllowedOrigins: []string{"http://localhost:5173"}},
		SessionCookieName: "naoja_session",
	}
}

func TestValidateAcceptsAWorkableConfiguration(t *testing.T) {
	if err := valid().Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateRejectsEachInsecureOrAmbiguousValue(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{
			name:   "unknown environment",
			mutate: func(c *Config) { c.Env = "staging" },
			want:   "APP_ENV",
		},
		{
			// This is the exact misconfiguration the Express service shipped: `*` combined with
			// credentialed cookies, which browsers refuse. Here it stops the process starting.
			name:   "wildcard origin",
			mutate: func(c *Config) { c.CORS.AllowedOrigins = []string{"*"} },
			want:   "wildcard",
		},
		{
			name:   "empty origin list",
			mutate: func(c *Config) { c.CORS.AllowedOrigins = nil },
			want:   "ALLOWED_ORIGINS",
		},
		{
			name:   "origin without a scheme",
			mutate: func(c *Config) { c.CORS.AllowedOrigins = []string{"localhost:5173"} },
			want:   "must include a scheme",
		},
		{
			name:   "missing database url",
			mutate: func(c *Config) { c.Database.URL = "" },
			want:   "DATABASE_URL",
		},
		{
			name:   "non-postgres scheme",
			mutate: func(c *Config) { c.Database.URL = "mysql://u:p@localhost/db" },
			want:   "postgres://",
		},
		{
			name:   "url without a user",
			mutate: func(c *Config) { c.Database.URL = "postgres://localhost:5433/naoja" },
			want:   "must carry a user",
		},
		{
			name:   "no listen address",
			mutate: func(c *Config) { c.Address = "" },
			want:   "ADDR",
		},
		{
			name:   "no cookie name",
			mutate: func(c *Config) { c.SessionCookieName = "" },
			want:   "SESSION_COOKIE_NAME",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := valid()
			tc.mutate(&c)
			err := c.Validate()
			if err == nil {
				t.Fatalf("expected an error mentioning %q, got none", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

// Errors.Join reports every problem at once. An operator fixing a deploy should not have to
// restart three times to discover three typos.
func TestValidateReportsEveryProblemAtOnce(t *testing.T) {
	c := valid()
	c.Env = "prod"
	c.CORS.AllowedOrigins = nil
	c.SessionCookieName = ""

	err := c.Validate()
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	for _, want := range []string{"APP_ENV", "ALLOWED_ORIGINS", "SESSION_COOKIE_NAME"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error should mention %q, got: %s", want, msg)
		}
	}
}

func TestRedactedURLNeverCarriesThePassword(t *testing.T) {
	const secret = "s3cr3t-word"
	d := Database{URL: "postgres://naoja:" + secret + "@localhost:5433/naoja?sslmode=disable"}

	got := d.RedactedURL()
	if strings.Contains(got, secret) {
		t.Fatalf("redacted URL still contains the password: %s", got)
	}
	// The username and host stay visible — they identify which database failed, which is the
	// whole reason the string is logged.
	for _, want := range []string{"naoja:", "localhost:5433", "sslmode=disable"} {
		if !strings.Contains(got, want) {
			t.Errorf("redacted URL %q lost %q", got, want)
		}
	}
	if !strings.Contains(got, "REDACTED") {
		t.Errorf("redacted URL %q does not show a mask", got)
	}
}

func TestRedactedURLKeepsAPasswordlessConnectionStringReadable(t *testing.T) {
	d := Database{URL: "postgres://localhost:5433/naoja?sslmode=disable"}
	if got := d.RedactedURL(); strings.Contains(got, "unparseable") {
		t.Fatalf("a valid passwordless URL should render as itself, got %q", got)
	}
}

func TestRedactedURLDoesNotPanicOnGarbage(t *testing.T) {
	for _, raw := range []string{"", "not a url at all", "://missing-scheme"} {
		if got := (Database{URL: raw}).RedactedURL(); got == "" {
			t.Errorf("RedactedURL(%q) returned empty", raw)
		}
	}
}

func TestLoadRejectsAPoolThatCannotFitItself(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://naoja:x@localhost:5433/naoja")
	t.Setenv("ALLOWED_ORIGINS", "http://localhost:5173")
	t.Setenv("DB_MIN_CONNS", "20")
	t.Setenv("DB_MAX_CONNS", "5")

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "DB_MIN_CONNS") {
		t.Fatalf("expected a DB_MIN_CONNS error, got %v", err)
	}
}

func TestLoadParsesTheEnvironmentItIsGiven(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("ADDR", ":9999")
	t.Setenv("LOG_LEVEL", "warn")
	t.Setenv("DATABASE_URL", "postgres://naoja:x@db.internal:5432/naoja?sslmode=require")
	t.Setenv("ALLOWED_ORIGINS", "https://shop.naojaventures.co.ke, https://admin.naojaventures.co.ke")
	t.Setenv("DB_MAX_CONNS", "30")
	t.Setenv("DB_PING_TIMEOUT", "1500ms")
	t.Setenv("HTTP_READ_TIMEOUT", "7s")
	t.Setenv("TRUST_PROXY_REQUEST_ID", "true")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !c.IsProduction() {
		t.Error("expected production")
	}
	if len(c.CORS.AllowedOrigins) != 2 {
		t.Errorf("expected 2 origins, got %v", c.CORS.AllowedOrigins)
	}
	if c.CORS.AllowedOrigins[1] != "https://admin.naojaventures.co.ke" {
		t.Errorf("origins were not trimmed: %q", c.CORS.AllowedOrigins[1])
	}
	if c.Database.MaxConns != 30 {
		t.Errorf("MaxConns = %d, want 30", c.Database.MaxConns)
	}
	if c.Database.PingBudget != 1500*time.Millisecond {
		t.Errorf("PingBudget = %s", c.Database.PingBudget)
	}
	if c.HTTPTimeouts.Read != 7*time.Second {
		t.Errorf("Read timeout = %s", c.HTTPTimeouts.Read)
	}
	if !c.RequestIDTrust {
		t.Error("expected RequestIDTrust")
	}
	if c.LogLevel != slog.LevelWarn {
		t.Errorf("LogLevel = %s", c.LogLevel)
	}
}

// A malformed duration must not silently become zero: a zero read timeout on a live listener
// would hang every request.
func TestLoadFallsBackWhenADurationIsUnparseable(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://naoja:x@localhost:5433/naoja")
	t.Setenv("ALLOWED_ORIGINS", "http://localhost:5173")
	t.Setenv("HTTP_READ_TIMEOUT", "15 seconds")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.HTTPTimeouts.Read != 15*time.Second {
		t.Errorf("Read = %s, want the 15s default", c.HTTPTimeouts.Read)
	}
}
