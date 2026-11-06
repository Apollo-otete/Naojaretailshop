// Package httpcookie is the transport's single decision point about cookies. The attributes are
// not per-call-site preferences: contracts/openapi.yaml fixes the exact string
// `naoja_session=<opaque>; HttpOnly; Secure; SameSite=Lax; Path=/`, ADR-019 makes the cookie the
// only place a session credential may appear, and every one of those attributes is doing a job.
//
//	Path=/      a cookie scoped to /auth would not be sent to /orders, and logout would
//	            "succeed" against a session that is still live
//	HttpOnly    no script can read the credential, which is what makes XSS a bad day rather
//	            than an account takeover
//	SameSite=Lax a cross-site navigation cannot carry the credential onto an unsafe method
//	Secure      set in production; without it the cookie travels in cleartext on the first
//	            http:// request a captive portal can make
package httpcookie

import (
	"net/http"
)

// Session is one cookie's worth of policy. Secure comes from the environment because a
// Secure-only cookie cannot be set over http://localhost, and a development machine that cannot
// sign in is a machine where the cookie path never gets tested at all.
type Session struct {
	Name   string
	Secure bool
}

// Set writes a browser-session cookie: no Max-Age and no Expires, on purpose. The server is the
// authority on when a session dies — it reads the row on every request (ADR-019) — so a cookie
// that outlived its own expiry would only produce a 401 where a clean re-login belongs. A
// browser that is closed forgets the value, which is the right amount of persistence for a
// credential that can be revoked.
func (s Session) Set(w http.ResponseWriter, token string) {
	http.SetCookie(w, s.cookie(token))
}

// Clear empties the cookie in place. The name, path and SameSite have to match the original or
// the browser treats it as a different cookie and keeps sending the live one — a logout that
// looks like it worked.
func (s Session) Clear(w http.ResponseWriter) {
	// Go's http.Cookie reads MaxAge as: >0 emit it, <0 emit Max-Age=0, 0 emit nothing. So the
	// deletion instruction the contract spells as "Max-Age=0" is -1 here, and 0 would silently
	// produce a session cookie that stays.
	c := s.cookie("")
	c.MaxAge = -1
	http.SetCookie(w, c)
}

func (s Session) cookie(value string) *http.Cookie {
	return &http.Cookie{
		Name:  s.Name,
		Value: value,
		// Path is "/" and there is no Domain: a scoped path would not reach /orders, and a
		// Domain attribute widens the cookie to every subdomain, including the ones nobody
		// promised to keep clean.
		Path:     "/",
		HttpOnly: true,
		Secure:   s.Secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// Read returns the cookie value, or "" when the request did not carry one.
//
// There is deliberately no query-parameter or header fallback. ADR-019 forbids a credential in a
// URL outright — a query string reaches the access log, the browser history, and the
// Referer header of whatever link the user clicks next — and contract principle 5 notes that the
// current API leaks exactly this way. A request that "sends the token in the query string" is
// therefore not authenticated, and it is not an error either: there is no such input.
func (s Session) Read(r *http.Request) string {
	c, err := r.Cookie(s.Name)
	if err != nil || c.Value == "" {
		return ""
	}
	return c.Value
}
