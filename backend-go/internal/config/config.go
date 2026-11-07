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

	Session Session
	Login   Login

	SessionCookieName string
	RequestIDTrust    bool // accept an inbound X-Request-Id (only when behind a proxy we control)

	// Mpesa configures the Daraja (M-Pesa) integration (P1). Optional when unset.
	Mpesa Mpesa

	// Cloudinary configures image uploads. Optional when unset.
	Cloudinary Cloudinary

	// Email (e.g. Resend) for transactional messages. Optional when unset.
	Email Email

	// Pusher for realtime/in-app notifications. Optional when unset.
	Pusher Pusher

	// Public/Frontend URLs (used for links/callback composition). Optional.
	FrontendURL   string
	PublicBaseURL string
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

// Session is the idle-window and cap pair ADR-019 describes. Absolute is measured from session
// creation, not from the last request, which is what keeps "revocable at any time" from becoming
// "revocable whenever somebody notices".
type Session struct {
	Idle      time.Duration
	AdminIdle time.Duration
	Absolute  time.Duration
}

// Login is ADR-019's failure policy: the per-account lockout (MaxFailures in Lockout) and the
// per-address rate limit that stands beside it — one attacker rotating between accounts must not
// buy five guesses per account.
type Login struct {
	MaxFailures int
	Lockout     time.Duration

	// RateLimit is the per-address attempt ceiling. The window is RateLimitWindow, and both are
	// deployment terms rather than constants in the domain: the defaults mirror the lockout policy
	// so a deployment that changes one changes the other coherently.
	RateLimit       int
	RateLimitWindow time.Duration
}

// Mpesa holds Daraja integration settings. Secrets must come from the environment/secret store,
// never from code or git. When Environment is empty or required fields are missing, the
// integration is treated as disabled (safe default).
type Mpesa struct {
	Environment    string // sandbox | production
	ConsumerKey    string
	ConsumerSecret string
	Shortcode      string
	Passkey        string
	CallbackURL    string
}

func (m Mpesa) Enabled() bool {
	return strings.TrimSpace(m.Environment) != "" &&
		strings.TrimSpace(m.ConsumerKey) != "" &&
		strings.TrimSpace(m.ConsumerSecret) != "" &&
		strings.TrimSpace(m.Shortcode) != "" &&
		strings.TrimSpace(m.Passkey) != "" &&
		strings.TrimSpace(m.CallbackURL) != ""
}

type Cloudinary struct {
	CloudName string
	APIKey    string
	APISecret string
}

func (c Cloudinary) Enabled() bool {
	return strings.TrimSpace(c.CloudName) != "" && strings.TrimSpace(c.APIKey) != "" && strings.TrimSpace(c.APISecret) != ""
}

type Email struct {
	Provider  string // e.g. resend
	APIKey    string
	FromEmail string
	FromName  string
}

func (e Email) Enabled() bool { return strings.TrimSpace(e.APIKey) != "" }

type Pusher struct {
	InstanceID string
	APIKey     string
}

func (p Pusher) Enabled() bool {
	return strings.TrimSpace(p.APIKey) != "" || strings.TrimSpace(p.InstanceID) != ""
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

	// ADR-019's shape, checked rather than assumed: an admin window longer than the customer's
	// would invert the one security claim the ADR makes about session lifetime, and a cap below
	// the idle window means every session silently expires at the cap.
	switch {
	case c.Session.Idle <= 0:
		errs = append(errs, fmt.Errorf("SESSION_IDLE must be positive, got %s", c.Session.Idle))
	case c.Session.AdminIdle <= 0:
		errs = append(errs, fmt.Errorf("SESSION_ADMIN_IDLE must be positive, got %s", c.Session.AdminIdle))
	case c.Session.AdminIdle > c.Session.Idle:
		errs = append(errs, fmt.Errorf("SESSION_ADMIN_IDLE (%s) must not exceed SESSION_IDLE (%s) — an administrative session is the shorter-lived class",
			c.Session.AdminIdle, c.Session.Idle))
	case c.Session.Absolute < c.Session.Idle:
		errs = append(errs, fmt.Errorf("SESSION_ABSOLUTE (%s) cannot be shorter than SESSION_IDLE (%s)", c.Session.Absolute, c.Session.Idle))
	}
	if c.Login.MaxFailures < 1 {
		errs = append(errs, fmt.Errorf("LOGIN_MAX_FAILURES must be at least 1, got %d", c.Login.MaxFailures))
	}
	if c.Login.Lockout <= 0 {
		errs = append(errs, fmt.Errorf("LOGIN_LOCKOUT must be positive, got %s", c.Login.Lockout))
	}
	if c.Login.RateLimit < 1 {
		errs = append(errs, fmt.Errorf("LOGIN_RATE_LIMIT must be at least 1, got %d", c.Login.RateLimit))
	}
	if c.Login.RateLimitWindow <= 0 {
		errs = append(errs, fmt.Errorf("LOGIN_RATE_WINDOW must be positive, got %s", c.Login.RateLimitWindow))
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
	c.Session = Session{
		Idle:      envDuration("SESSION_IDLE", 30*time.Minute),
		AdminIdle: envDuration("SESSION_ADMIN_IDLE", 15*time.Minute),
		Absolute:  envDuration("SESSION_ABSOLUTE", 12*time.Hour),
	}
	c.Login = Login{
		MaxFailures:     envInt("LOGIN_MAX_FAILURES", 5),
		Lockout:         envDuration("LOGIN_LOCKOUT", 15*time.Minute),
		RateLimit:       envInt("LOGIN_RATE_LIMIT", 5),
		RateLimitWindow: envDuration("LOGIN_RATE_WINDOW", 15*time.Minute),
	}

	c.Mpesa = loadMpesa()
	c.Cloudinary = Cloudinary{
		CloudName: envStr("CLOUDINARY_CLOUD_NAME", ""),
		APIKey:    envStr("CLOUDINARY_API_KEY", ""),
		APISecret: envStr("CLOUDINARY_API_SECRET", ""),
	}
	c.Email = Email{
		Provider:  envStr("EMAIL_PROVIDER", "resend"),
		APIKey:    envStr("RESEND_API_KEY", envStr("EMAIL_API_KEY", "")),
		FromEmail: envStr("EMAIL_FROM", ""),
		FromName:  envStr("EMAIL_FROM_NAME", "Naoja"),
	}
	c.Pusher = Pusher{
		InstanceID: envStr("PUSHER_INSTANCE_ID", ""),
		APIKey:     envStr("PUSHER_API_KEY", ""),
	}
	c.FrontendURL = envStr("FRONTEND_URL", "")
	c.PublicBaseURL = envStr("PUBLIC_BASE_URL", "")

	if c.Database.MinConns > c.Database.MaxConns {
		return Config{}, fmt.Errorf("DB_MIN_CONNS (%d) cannot exceed DB_MAX_CONNS (%d)", c.Database.MinConns, c.Database.MaxConns)
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func loadMpesa() Mpesa {
	env := envStr("MPESA_ENV", envStr("ENVIRONMENT", ""))
	if env == "" {
		return Mpesa{}
	}
	prod := strings.EqualFold(env, "production") || strings.EqualFold(env, "prod")
	if prod {
		return Mpesa{
			Environment:    "production",
			ConsumerKey:    envStr("MPESA_CONSUMER_KEY_PROD", envStr("MPESA_CONSUMER_KEY", "")),
			ConsumerSecret: envStr("MPESA_CONSUMER_SECRET_PROD", envStr("MPESA_CONSUMER_SECRET", "")),
			Shortcode:      envStr("MPESA_SHORTCODE_PROD", envStr("MPESA_SHORTCODE", "")),
			Passkey:        envStr("MPESA_PASSKEY_PROD", envStr("MPESA_PASSKEY", "")),
			CallbackURL:    envStr("MPESA_CALLBACK_URL_PROD", envStr("MPESA_CALLBACK_URL", "")),
		}
	}
	return Mpesa{
		Environment:    "sandbox",
		ConsumerKey:    envStr("MPESA_CONSUMER_KEY", ""),
		ConsumerSecret: envStr("MPESA_CONSUMER_SECRET", ""),
		Shortcode:      envStr("MPESA_SHORTCODE", ""),
		Passkey:        envStr("MPESA_PASSKEY", ""),
		CallbackURL:    envStr("MPESA_CALLBACK_URL", ""),
	}
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
