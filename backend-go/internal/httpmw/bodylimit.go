package httpmw

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BodyLimit caps request body size as a safety net (§10). Handlers may apply stricter limits.
// 10 MiB accommodates image uploads; most JSON endpoints are far smaller.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil && c.Request.ContentLength != 0 {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}
