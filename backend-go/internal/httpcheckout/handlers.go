package httpcheckout

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpjson"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/ordering"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
)

type Service interface {
	CreateQuote(ctx context.Context, caller *identity.Principal, sessionToken string, req ordering.QuoteRequest) (ordering.Quote, error)
}

type Deps struct {
	Ordering   Service
	Authorizer port.Authorizer
	Logger     *slog.Logger
}

func Mount(r gin.IRouter, d Deps) {
	r.POST("/checkout/quotes", createQuote(d))
}

func decode(c *gin.Context, target any) error {
	if err := json.NewDecoder(c.Request.Body).Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return apierr.BadRequest("A JSON request body is required.")
		}
		return apierr.BadRequest("The request body is not valid JSON.")
	}
	return nil
}

func sessionToken(c *gin.Context) string {
	if v, err := c.Cookie("naoja_cart"); err == nil && v != "" {
		return v
	}
	if v := c.GetHeader("X-Cart-Session"); v != "" {
		return v
	}
	return ""
}

func createQuote(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var caller *identity.Principal
		if p, ok := identity.PrincipalFrom(c.Request.Context()); ok {
			pp := p
			caller = &pp
		}
		var req ordering.QuoteRequest
		if err := decode(c, &req); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		q, err := d.Ordering.CreateQuote(c.Request.Context(), caller, sessionToken(c), req)
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		httpjson.Created(c, q)
	}
}
