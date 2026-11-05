// Package config loads service configuration from the environment and refuses to start on
// values that would produce an insecure or ambiguous runtime (§10: secrets in environment,
// never in Git or logs).
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env      string // development | test | production
	Address  string
	LogLevel slog.Level

	Database     Database
	CORS         CORS
	HTTPTimeouts Timeouts

	// SessionCookieName is referenced by the middleware contract even though sessions
	// themselves land in Phase 3 (ADR-019); it is here so the cookie attributes have one
	// authoritative source rather than being re-decided per call site.
	SessionCookieName string

	RequestIDTrust bool // accept an inbound X-Request-Id (only when behind a proxy we control)
}

type Database struct {
	URL        string
	MaxConns   int32
	MinConns   int32
	PingBudget time.Duration
}

type CORS struct {
	AllowedOrigins []string
}

type Timeouts struct {
	Read  time.Duration
	Write time.Duration
	Idle  time.Duration
}

func (c Config) IsProduction() bool { return c.Env == "production" }

// RedactedURL strips the password so a connection string can appear in a log line or a
// startup banner without becoming a credential leak (§10: secrets never in logs).
//
// The mask is the word REDACTED rather than asterisks: url.URL.String() percent-encodes
// password userinfo, and "******" would arrive in the log as %2A%2A%2A%2A%2A%2A — unreadable
// and easy to mistake for an encoded real secret.
func (d Database) RedactedURL() string {
	if d.URL == "" {
		return "<unset>"
	}
	u, err := url.Parse(d.URL)
	if err != nil {
		return "<unparseable>"
	}
	if _, ok := u.User.Password(); ok {
		u.User = url.UserPassword(u.User.Username(), "REDACTED")
	}
	return u.String()
}

// Validate centralises the rules that would otherwise be discovered at runtime by a customer.
func (c Config) Validate() error {
	var errs []error

	if c.Env != "development" && c.Env != "test" && c.Env != "production" {
		errs = append(errs, fmt.Errorf("APP_ENV must be development, test or production, got %q", c.Env))
	}
	if c.Address == "" {
		errs = append(errs, errors.New("ADDR must be set, e.g. :8080"))
	}
	if c.Database.URL == "" {
		errs = append(errs, errors.New("DATABASE_URL must be set"))
	}
	if err := validatePostgresURL(c.Database.URL); err != nil {
		errs = append(errs, err)
	}

	// ADR-019 consequence: credentialed cross-origin requests need explicit origins. The
	// Express service fell back to "*" when FRONTEND_URL was unset, which browsers reject
	// alongside credentials — so the misconfiguration surfaced as a mysterious login failure.
	// Here it is a startup error instead.
	if len(c.CORS.AllowedOrigins) == 0 {
		errs = append(errs, errors.New("ALLOWED_ORIGINS must list at least one explicit origin; wildcards are not supported with credentialed cookies"))
	} else {
		for _, o := range c.CORS.AllowedOrigins {
			if strings.ContainsAny(o, "*") {
				errs = append(errs, fmt.Errorf("ALLOWED_ORIGINS entry %q contains a wildcard", o))
			}
			if !strings.HasPrefix(o, "http://") && !strings.HasPrefix(o, "https://") {
				errs = append(errs, fmt.Errorf("ALLOWED_ORIGINS entry %q must include a scheme", o))
			}
		}
	}

	if c.SessionCookieName == "" {
		errs = append(errs, errors.New("SESSION_COOKIE_NAME must be set"))
	}

	return errors.Join(errs...)
}

func validatePostgresURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("DATABASE_URL is not a parseable URL: %w", err)
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return fmt.Errorf("DATABASE_URL must use postgres:// or postgresql://, got %q", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("DATABASE_URL has no host")
	}
	if u.User.Username() == "" {
		return errors.New("DATABASE_URL must carry a user")
	}
	return nil
}

// Load reads configuration from the process environment. A missing file is not an error:
// containerized deployments inject variables directly.
func Load() (Config, error) {
	if err := LoadDotenv(".env"); err != nil {
		return Config{}, fmt.Errorf("loading .env: %w", err)
	}

	c := Config{
		Env:               envStr("APP_ENV", "development"),
		Address:           envStr("ADDR", ":8080"),
		LogLevel:          envLevel("LOG_LEVEL", slog.LevelInfo),
		Database:          Database{URL: envStr("DATABASE_URL", "")},
		CORS:              CORS{AllowedOrigins: envList("ALLOWED_ORIGINS")},
		SessionCookieName: envStr("SESSION_COOKIE_NAME", "naoja_session"),
		RequestIDTrust:    envBool("TRUST_PROXY_REQUEST_ID", false),
	}

	c.Database.MaxConns = int32(envInt("DB_MAX_CONNS", 12))
	c.Database.MinConns = int32(envInt("DB_MIN_CONNS", 1))
	c.Database.PingBudget = envDuration("DB_PING_TIMEOUT", 3*time.Second)
	c.HTTPTimeouts = Timeouts{
		Read:  envDuration("HTTP_READ_TIMEOUT", 15*time.Second),
		Write: envDuration("HTTP_WRITE_TIMEOUT", 30*time.Second),
		Idle:  envDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
	}

	if c.Database.MinConns > c.Database.MaxConns {
		return Config{}, fmt.Errorf("DB_MIN_CONNS (%d) cannot exceed DB_MAX_CONNS (%d)", c.Database.MinConns, c.Database.MaxConns)
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func envStr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func envList(key string) []string {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return b
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(strings.TrimSpace(v)); err == nil {
			return d
		}
	}
	return fallback
}

func envLevel(key string, fallback slog.Level) slog.Level {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	var l slog.Level
	if err := l.UnmarshalText([]byte(strings.TrimSpace(v))); err != nil {
		return fallback
	}
	return l
}
