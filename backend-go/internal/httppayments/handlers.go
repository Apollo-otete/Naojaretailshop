package httppayments

import (
	"context"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpjson"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/payments"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/payments/daraja"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
)

type Service interface {
	InitiateSTK(ctx context.Context, orderRef string, phone string) (payments.InitiateResult, error)
	ProcessCallback(ctx context.Context, cb daraja.CallbackResult) error
	PaymentByOrderRef(ctx context.Context, ref string) (payments.Payment, []payments.Attempt, []payments.Transaction, error)
}

type Deps struct {
	Payments   Service
	Authorizer port.Authorizer
	Logger     *slog.Logger
}

// MountAuthenticated registers user-facing payment operations (behind session/CSRF).
func MountAuthenticated(r gin.IRouter, d Deps) {
	r.POST("/orders/:reference/payments/stk-push", initiateSTK(d))
	r.GET("/orders/:reference/payment", getPayment(d))
}

// MountCallback registers the Daraja callback (no session/CSRF). Must be mounted before auth middleware.
func MountCallback(r gin.IRouter, d Deps) {
	r.POST("/payments/daraja/callback", callback(d))
	r.POST("/api/v1/payments/daraja/callback", callback(d))
	r.POST("/payment/callback", callback(d))     // compat
	r.POST("/api/payment/callback", callback(d)) // compat with legacy/prod URL shape
}

func initiateSTK(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := identity.PrincipalFrom(c.Request.Context()); !ok {
			httpjson.Write(c, d.Logger, apierr.Unauthorized("Sign in."))
			return
		}
		var in struct {
			Phone string `json:"phone"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			httpjson.Write(c, d.Logger, apierr.BadRequest("The request body is not valid JSON."))
			return
		}
		res, err := d.Payments.InitiateSTK(c.Request.Context(), c.Param("reference"), in.Phone)
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		httpjson.Accepted(c, gin.H{
			"order_reference":     c.Param("reference"),
			"checkout_request_id": res.CheckoutRequestID,
			"merchant_request_id": res.Attempt.MerchantRequestID,
			"customer_message":    res.CustomerMessage,
			"status":              "pending",
			"attempt":             res.Attempt,
		})
	}
}

func getPayment(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := identity.PrincipalFrom(c.Request.Context()); !ok {
			httpjson.Write(c, d.Logger, apierr.Unauthorized("Sign in."))
			return
		}
		p, atts, txs, err := d.Payments.PaymentByOrderRef(c.Request.Context(), c.Param("reference"))
		if err != nil {
			if err == payments.ErrNoPayment {
				httpjson.Write(c, d.Logger, apierr.NotFound("payment"))
				return
			}
			httpjson.Fail(c, d.Logger, err)
			return
		}
		httpjson.OK(c, gin.H{"payment": p, "attempts": atts, "transactions": txs})
	}
}

func callback(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
		if err != nil {
			// Ack to avoid harmful retry storms when body is unreadable; log.
			if d.Logger != nil {
				d.Logger.Warn("daraja callback unreadable", slog.Any("err", err))
			}
			c.JSON(http.StatusOK, gin.H{"ResultCode": 0, "ResultDesc": "Accepted"})
			return
		}
		cb, err := daraja.ParseCallback(body)
		if err != nil {
			if d.Logger != nil {
				d.Logger.Warn("daraja callback parse failed", slog.Any("err", err))
			}
			c.JSON(http.StatusOK, gin.H{"ResultCode": 0, "ResultDesc": "Accepted"})
			return
		}
		if err := d.Payments.ProcessCallback(c.Request.Context(), cb); err != nil {
			if d.Logger != nil {
				d.Logger.Error("daraja callback processing failed", slog.Any("err", err), slog.String("checkout", cb.CheckoutRequestID))
			}
			// Return non-2xx so Daraja may retry; processing is idempotent.
			c.JSON(http.StatusInternalServerError, gin.H{"ResultCode": 1, "ResultDesc": "Failed"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ResultCode": 0, "ResultDesc": "Accepted"})
	}
}

var _ = http.StatusOK
