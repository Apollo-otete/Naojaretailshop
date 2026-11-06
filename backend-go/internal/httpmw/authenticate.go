package httpmw

import (
	"context"
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpcookie"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpjson"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
)

// Sessions is the per-request read of the cookie's server-side half. *identity.Service
// implements it; the interface is here so this middleware can be tested without a database and
// so nothing in the transport depends on the use cases that sit behind the read.
type Sessions interface {
	Authenticate(ctx context.Context, token string) (identity.Principal, error)
}

// Authenticate turns a session cookie into a principal on the request context.
//
// It is global rather than per-route because the contract declares sessionCookie in the
// *top-level* security list: every operation is either public or session-authenticated, and
// there is no third kind where the cookie is ignored. Being global also means the answer is
// always fresh — ADR-019's revocation-is-immediate is a property of this read happening on
// every request, and B11 (a suspended account fails on the next request, not the next login)
// has nowhere else to live.
//
// A request with no cookie, or with one that no longer works, is **not** refused here. Refusing
// would make every public endpoint fail for a browser holding a stale cookie — the /products
// list is public whether you are signed in or not, and a shop cannot render it at 401. The
// refusal happens one step later, in the operation's own guard, which cannot be satisfied
// without a principal. What this middleware does instead is clear the cookie, so the browser
// stops sending a credential that will never work again and the next request starts clean.
func Authenticate(l *slog.Logger, sessions Sessions, cookie httpcookie.Session) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := cookie.Read(c.Request)
		if token == "" {
			c.Next()
			return
		}

		p, err := sessions.Authenticate(c.Request.Context(), token)
		switch {
		case err == nil:
			c.Request = c.Request.WithContext(identity.WithPrincipal(c.Request.Context(), p))
			c.Next()
			return

		case isDeadSession(err):
			cookie.Clear(c.Writer)
			l.Info("refusing a session that no longer works", slog.String("reason", err.Error()))
			c.Next()
			return

		default:
			// The grants, the account row or the database itself failed. That is not "anonymous"
			// and it is not an expired session: it is a request whose authority cannot be
			// established, and the only safe answer is to stop here. Falling through to
			// anonymous would turn an outage into 401s that tell the client to sign in again,
			// and — worse — into a public route quietly succeeding.
			cookie.Clear(c.Writer)
			httpjson.Fail(c, l, err)
			c.Abort()
			return
		}
	}
}

// isDeadSession is the closed list of reasons a cookie is worthless: the session row is gone or
// says it is over, or the account says it may not act. Everything else is a failure to answer,
// not an answer.
func isDeadSession(err error) bool {
	return errors.Is(err, identity.ErrSessionNotFound) ||
		errors.Is(err, identity.ErrSessionRevoked) ||
		errors.Is(err, identity.ErrSessionExpired) ||
		errors.Is(err, identity.ErrSuspended) ||
		errors.Is(err, identity.ErrInactive)
}
