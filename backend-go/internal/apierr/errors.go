// Package apierr is the single vocabulary of failures this service produces, with no HTTP or
// database imports: a use case returns an apierr.Failure and the transport layer decides what
// status and body that becomes, which is how Gin stays out of the domain (§8).
//
// The wire shape is dictated by contracts/openapi.yaml (components.schemas.Error). Every
// failure in this service — including ones this phase has not imagined yet — renders through
// this package, so the "consistent error shape" the contract promises is a property of the
// code rather than of each handler's discipline.
package apierr

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Code is a stable, machine-readable identifier. Clients switch on it; they must never match
// on Message, which is prose for a human and may change without warning.
type Code string

const (
	CodeValidationFailed       Code = "VALIDATION_FAILED"
	CodePermissionDenied       Code = "PERMISSION_DENIED"
	CodeNotAuthenticated       Code = "NOT_AUTHENTICATED"
	CodeNotFound               Code = "NOT_FOUND"
	CodeMethodNotAllowed       Code = "METHOD_NOT_ALLOWED"
	CodeConflict               Code = "CONFLICT"
	CodeStockInsufficient      Code = "STOCK_INSUFFICIENT"
	CodeQuoteStale             Code = "QUOTE_STALE"
	CodeTransitionNotAllowed   Code = "TRANSITION_NOT_ALLOWED"
	CodeOverrideReasonRequired Code = "OVERRIDE_REASON_REQUIRED"
	CodeMethodConflict         Code = "METHOD_CONFLICT"
	CodeIdempotencyConflict    Code = "IDEMPOTENCY_KEY_CONFLICT"
	CodePriceChangeNotAllowed  Code = "PRICE_CHANGE_NOT_ALLOWED"
	CodeProductHasHistory      Code = "PRODUCT_HAS_ORDER_HISTORY"
	CodePaymentAlreadySettled  Code = "PAYMENT_ALREADY_SETTLED"
	CodeRateLimited            Code = "RATE_LIMITED"
	CodeProviderUnavailable    Code = "PROVIDER_UNAVAILABLE"
	CodeInternal               Code = "INTERNAL_ERROR"
)

// Detail is one field- or entity-level explanation. The extra keys mirror the contract's
// STOCK_INSUFFICIENT items, which need to name a product and quantify the shortfall.
type Detail struct {
	Field     string `json:"field,omitempty"`
	Issue     string `json:"issue,omitempty"`
	ProductID int64  `json:"product_id,omitempty"`
	Requested int    `json:"requested,omitempty"`
	Available int    `json:"available,omitempty"`
}

// Failure is an error that knows how it should be rendered.
type Failure struct {
	Status   int
	Code     Code
	Message  string
	Details  []Detail
	Missing  string // permission key, for PERMISSION_DENIED (contract: PermissionError)
	RetryIn  int    // seconds, for RATE_LIMITED
	internal error  // logged, never serialized
}

func (f Failure) Error() string {
	if f.internal != nil {
		return fmt.Sprintf("%s: %s: %v", f.Code, f.Message, f.internal)
	}
	return fmt.Sprintf("%s: %s", f.Code, f.Message)
}

// Unwrap exposes the cause to errors.Is/As without ever putting it on the wire.
func (f Failure) Unwrap() error { return f.internal }

// AsFailure extracts a Failure from any error, converting unknown errors into a 500 whose
// message says nothing. This default-deny behaviour is the point: a panic or an unexpected
// driver error cannot leak a SQL fragment or a provider credential into a response body.
func AsFailure(err error) Failure {
	var f Failure
	if errors.As(err, &f) {
		return f
	}
	return Failure{
		Status:   http.StatusInternalServerError,
		Code:     CodeInternal,
		Message:  "The request could not be completed. The support team has been notified.",
		internal: err,
	}
}

// New builds a Failure. Prefer the named helpers below for anything a client can trigger.
func New(status int, code Code, msg string, details ...Detail) Failure {
	return Failure{Status: status, Code: code, Message: msg, Details: details}
}

func Unauthorized(msg string) Failure {
	return Failure{Status: http.StatusUnauthorized, Code: CodeNotAuthenticated, Message: msg}
}

// Forbidden names the absent permission. The caller only reaches this if the resource is
// known to exist for them; where existence itself is secret, use NotFound instead.
func Forbidden(permission string) Failure {
	return Failure{
		Status:  http.StatusForbidden,
		Code:    CodePermissionDenied,
		Message: "You do not have permission to perform this action.",
		Missing: permission,
	}
}

func NotFound(what string) Failure {
	return Failure{Status: http.StatusNotFound, Code: CodeNotFound, Message: fmt.Sprintf("No %s was found.", what)}
}

func Validation(details ...Detail) Failure {
	return Failure{
		Status:  http.StatusUnprocessableEntity,
		Code:    CodeValidationFailed,
		Message: "The request did not pass validation.",
		Details: details,
	}
}

func Conflict(code Code, msg string, details ...Detail) Failure {
	return Failure{Status: http.StatusConflict, Code: code, Message: msg, Details: details}
}

func Internal(err error) Failure {
	return Failure{
		Status:   http.StatusInternalServerError,
		Code:     CodeInternal,
		Message:  "The request could not be completed. The support team has been notified.",
		internal: err,
	}
}

// Envelope is the wire format.
type Envelope struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	// No omitempty here on purpose: the contract types `details` as an array, and a client
	// that writes error.details.forEach() must not crash on the failures that carry no
	// specifics. Together with Write's nil-to-empty coercion, the field is always present.
	Details       []Detail `json:"details"`
	CorrelationID string   `json:"correlation_id"`
	// MissingPermission is emitted for PERMISSION_DENIED, matching the contract's
	// PermissionError so a client can diagnose a role gap without reading server logs.
	MissingPermission string `json:"missing_permission,omitempty"`
}

// EnvelopeFor is the only way an Envelope is built, so the coercion that keeps `details` an
// array lives next to the tag that makes it visible.
func EnvelopeFor(f Failure, correlationID string) Envelope {
	details := f.Details
	if details == nil {
		details = []Detail{}
	}
	return Envelope{
		Code:              f.Code,
		Message:           f.Message,
		Details:           details,
		CorrelationID:     correlationID,
		MissingPermission: f.Missing,
	}
}

func (e Envelope) String() string {
	b, _ := json.Marshal(e)
	return string(b)
}
