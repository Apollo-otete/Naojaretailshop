package httpcookie

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// The contract fixes the cookie's wire form, so these tests assert the attributes a browser will
// actually see rather than the string Go happens to emit — the order of attributes is Go's
// business, their presence is ours.

const (
	name  = "naoja_session"
	value = "Zm9vYmFyYmF6cXV1eS5zZXNzaW9uLnRva2Vu"
)

// attrs writes one cookie operation and returns its Set-Cookie header as a lowercase-keyed
// attribute map: the "name=value" pair under "pair", a valueless flag under its own name with an
// empty value.
func attrs(t *testing.T, write func(http.ResponseWriter)) map[string]string {
	t.Helper()

	rec := httptest.NewRecorder()
	write(rec)

	res := rec.Result()
	defer res.Body.Close()

	headers := res.Header.Values("Set-Cookie")
	if len(headers) != 1 {
		t.Fatalf("%d Set-Cookie headers, want exactly 1: %v", len(headers), headers)
	}
	out := map[string]string{}
	for i, part := range strings.Split(headers[0], ";") {
		part = strings.TrimSpace(part)
		if i == 0 {
			out["pair"] = part
			continue
		}
		key, val, _ := strings.Cut(part, "=")
		out[strings.ToLower(key)] = val
	}
	return out
}

func TestTheSessionCookieCarriesExactlyTheContractAttributes(t *testing.T) {
	got := attrs(t, func(w http.ResponseWriter) {
		Session{Name: name, Secure: true}.Set(w, value)
	})

	if want := name + "=" + value; got["pair"] != want {
		t.Errorf("cookie pair = %q, want %q", got["pair"], want)
	}
	if _, ok := got["httponly"]; !ok {
		t.Error("HttpOnly is missing: without it, an XSS reads the credential instead of only using it")
	}
	if _, ok := got["secure"]; !ok {
		t.Error("Secure is missing on a production-shaped cookie")
	}
	if got["samesite"] != "Lax" {
		t.Errorf("SameSite = %q, want Lax", got["samesite"])
	}
	if got["path"] != "/" {
		t.Errorf("Path = %q, want / — a scoped path would not reach /orders", got["path"])
	}
	// No Domain: it would widen the credential to every subdomain of the site.
	if _, ok := got["domain"]; ok {
		t.Errorf("Domain = %q, which widens the cookie to subdomains nobody promised to keep clean", got["domain"])
	}
	// No Max-Age and no Expires: the server decides when a session dies, and a cookie that
	// outlived its own expiry only produces a 401 where a re-login belongs.
	for flag, why := range map[string]string{
		"max-age": "a persistent cookie outlives the server's answer about the session",
		"expires": "same, and a client-set clock makes a mockery of it",
	} {
		if _, ok := got[flag]; ok {
			t.Errorf("%s is set on a browser-session cookie: %s", flag, why)
		}
	}
}

func TestDevelopmentSendsAnInsecureCookieAndSaysSo(t *testing.T) {
	got := attrs(t, func(w http.ResponseWriter) {
		Session{Name: name, Secure: false}.Set(w, value)
	})
	if _, ok := got["secure"]; ok {
		t.Error("Secure is set: a browser on http://localhost would drop the cookie entirely, and nobody would test the login path")
	}
}

func TestClearingUsesTheNamePathAndSiteOfTheOriginal(t *testing.T) {
	s := Session{Name: name, Secure: true}
	got := attrs(t, s.Clear)

	if got["pair"] != name+"=" {
		t.Fatalf("clearing wrote %q; a cookie that still carries a value is not an empty one", got["pair"])
	}
	// Max-Age=0 is the deletion instruction. Go spells it with MaxAge = -1, which is the trap this
	// assertion exists for: MaxAge = 0 emits nothing at all and the live cookie survives.
	if val, ok := got["max-age"]; !ok || val != "0" {
		t.Errorf("Max-Age = %q (present: %t), want 0 — the cookie would outlive the logout", val, ok)
	}
	// Attributes that must match the cookie being removed, or the browser reads this as an
	// unrelated cookie and keeps sending the live one.
	if got["path"] != "/" {
		t.Errorf("Path = %q on the clearing cookie, want / to match", got["path"])
	}
	if got["samesite"] != "Lax" {
		t.Errorf("SameSite = %q on the clearing cookie, want Lax to match", got["samesite"])
	}
	if _, ok := got["httponly"]; !ok {
		t.Error("HttpOnly is missing on the clearing cookie")
	}
	if _, ok := got["secure"]; !ok {
		t.Error("Secure is missing on the clearing cookie")
	}
	if _, ok := got["domain"]; ok {
		t.Errorf("Domain = %q on the clearing cookie, which would not match the one being removed", got["domain"])
	}
}

func TestTheCredentialIsReadFromTheCookieAndFromNowhereElse(t *testing.T) {
	s := Session{Name: name}

	t.Run("cookie", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/orders/NJ-1", nil)
		r.AddCookie(&http.Cookie{Name: name, Value: value})
		if got := s.Read(r); got != value {
			t.Errorf("Read = %q, want the cookie value", got)
		}
	})

	t.Run("query string is not an input", func(t *testing.T) {
		// ADR-019 and contract principle 5: no credential travels in a URL. A query parameter
		// carrying a live token is not authenticated, because a query string reaches the access
		// log, the browser history and the next Referer header.
		r := httptest.NewRequest(http.MethodGet, "/orders/NJ-1", nil)
		r.URL.RawQuery = url.Values{name: {value}}.Encode()
		if got := s.Read(r); got != "" {
			t.Errorf("Read honoured a query-string credential: %q", got)
		}
	})

	t.Run("empty cookie", func(t *testing.T) {
		// The clearing cookie can be replayed by a client that kept the header; an empty value
		// must not become a lookup.
		r := httptest.NewRequest(http.MethodGet, "/orders/NJ-1", nil)
		r.AddCookie(&http.Cookie{Name: name, Value: ""})
		if got := s.Read(r); got != "" {
			t.Errorf("Read = %q for an empty cookie, want the unauthenticated answer", got)
		}
	})

	t.Run("no cookie", func(t *testing.T) {
		if got := s.Read(httptest.NewRequest(http.MethodGet, "/orders/NJ-1", nil)); got != "" {
			t.Errorf("Read = %q with no cookie at all", got)
		}
	})

	t.Run("someone else's cookie", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/orders/NJ-1", nil)
		r.AddCookie(&http.Cookie{Name: "other_session", Value: value})
		if got := s.Read(r); got != "" {
			t.Error("Read returned a value from a cookie with a different name")
		}
	})
}
