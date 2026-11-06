package httpmw

import (
	"crypto/subtle"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpjson"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
)

// CSRF refuses an unsafe method that a browser would have sent on someone's behalf.
//
// The credential is an ambient one — an HttpOnly cookie the client cannot read and a cross-site
// form will happily attach — so "the cookie was present" does not prove the request came from our
// own frontend. It is part of the declared security scheme (contracts/openapi.yaml: sessionCookie
// *and* csrfHeader on every unsafe method), which is why a failure here is NOT_AUTHENTICATED
// rather than PERMISSION_DENIED: nothing was decided about the caller's authority, and a
// PERMISSION_DENIED body would promise a missing_permission that does not exist.
//
// The check runs after authentication because the value it compares against belongs to the
// session, not to the request: the header is proof of intent, never a means of identification.
func CSRF(l *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isSafeMethod(c.Request.Method) {
			c.Next()
			return
		}
		caller, ok := identity.PrincipalFrom(c.Request.Context())
		if !ok {
			// No session carried this request, so there is no ambient credential to abuse.
			// /auth/register and /auth/login land here; rate limiting, not CSRF, is their
			// protection (ADR-019 pairs the two).
			c.Next()
			return
		}

		supplied := c.GetHeader(identity.CSRFHeaderName)
		if !tokenMatches(caller.CSRFToken, supplied) {
			httpjson.Write(c, l, apierr.Unauthorized(
				"A session cookie is not proof of intent: send the "+identity.CSRFHeaderName+
					" header that GET /auth/session issues."))
			return
		}
		c.Next()
	}
}

// isSafeMethod follows RFC 9110: these three ask for data and change nothing, so a forged one is
// at worst a read. OPTIONS is the CORS preflight, which by definition carries no cookies —
// requiring the header there would make every browser request in the frontend fail before it
// started.
func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

// tokenMatches fails when either side is empty: a session with no stored token cannot perform an
// unsafe method, so a row written before the column existed is refused rather than let through.
func tokenMatches(want, got string) bool {
	if want == "" || got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}
