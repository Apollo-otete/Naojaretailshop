package httpshopping

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpjson"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/shopping"
)

type Service interface {
	ViewCart(ctx context.Context, caller *identity.Principal, sessionToken string) (shopping.CartView, error)
	AddItem(ctx context.Context, caller *identity.Principal, sessionToken string, productID int64, qty int) error
	UpdateItem(ctx context.Context, caller *identity.Principal, sessionToken string, productID int64, qty int) error
	RemoveItem(ctx context.Context, caller *identity.Principal, sessionToken string, productID int64) error
}

type Deps struct {
	Shopping   Service
	Authorizer port.Authorizer
	Logger     *slog.Logger
}

func Mount(r gin.IRouter, d Deps) {
	l := d.Logger
	if l == nil {
		l = slog.Default()
	}
	r.GET("/cart", viewCart(d))
	r.POST("/cart/items", addItem(d))
	r.PATCH("/cart/items/:product_id", updateItem(d))
	r.DELETE("/cart/items/:product_id", removeItem(d))
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

func callerPtr(c *gin.Context) *identity.Principal {
	if p, ok := identity.PrincipalFrom(c.Request.Context()); ok {
		pp := p
		return &pp
	}
	return nil
}

func viewCart(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, err := d.Shopping.ViewCart(c.Request.Context(), callerPtr(c), sessionToken(c))
		if err != nil {
			if errors.Is(err, shopping.ErrGuestSessionRequired) {
				httpjson.Write(c, d.Logger, apierr.BadRequest("A cart session is required for guest carts."))
				return
			}
			httpjson.Fail(c, d.Logger, err)
			return
		}
		httpjson.OK(c, v)
	}
}

func addItem(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			ProductID int64 `json:"product_id"`
			Quantity  int   `json:"quantity"`
		}
		if err := decode(c, &in); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		if in.Quantity < 1 {
			in.Quantity = 1
		}
		if err := d.Shopping.AddItem(c.Request.Context(), callerPtr(c), sessionToken(c), in.ProductID, in.Quantity); err != nil {
			if errors.Is(err, shopping.ErrGuestSessionRequired) {
				httpjson.Write(c, d.Logger, apierr.BadRequest("A cart session is required for guest carts."))
				return
			}
			httpjson.Fail(c, d.Logger, err)
			return
		}
		v, err := d.Shopping.ViewCart(c.Request.Context(), callerPtr(c), sessionToken(c))
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		httpjson.OK(c, v)
	}
}

func updateItem(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		pid, err := strconv.ParseInt(c.Param("product_id"), 10, 64)
		if err != nil {
			httpjson.Write(c, d.Logger, apierr.BadRequest("Invalid product id."))
			return
		}
		var in struct {
			Quantity int `json:"quantity"`
		}
		if err := decode(c, &in); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		if err := d.Shopping.UpdateItem(c.Request.Context(), callerPtr(c), sessionToken(c), pid, in.Quantity); err != nil {
			if errors.Is(err, shopping.ErrGuestSessionRequired) {
				httpjson.Write(c, d.Logger, apierr.BadRequest("A cart session is required for guest carts."))
				return
			}
			httpjson.Fail(c, d.Logger, err)
			return
		}
		v, err := d.Shopping.ViewCart(c.Request.Context(), callerPtr(c), sessionToken(c))
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		httpjson.OK(c, v)
	}
}

func removeItem(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		pid, err := strconv.ParseInt(c.Param("product_id"), 10, 64)
		if err != nil {
			httpjson.Write(c, d.Logger, apierr.BadRequest("Invalid product id."))
			return
		}
		if err := d.Shopping.RemoveItem(c.Request.Context(), callerPtr(c), sessionToken(c), pid); err != nil {
			if errors.Is(err, shopping.ErrGuestSessionRequired) {
				httpjson.Write(c, d.Logger, apierr.BadRequest("A cart session is required for guest carts."))
				return
			}
			httpjson.Fail(c, d.Logger, err)
			return
		}
		v, err := d.Shopping.ViewCart(c.Request.Context(), callerPtr(c), sessionToken(c))
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		httpjson.OK(c, v)
	}
}

var _ = http.StatusOK
