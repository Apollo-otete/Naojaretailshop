package apierr

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// The contract promises one error shape. These tests are the cheap way to keep that promise
// while there is only one place that renders it — by the time Phase 4 has twenty handlers,
// this is the property that stops drifting.

func TestAsFailureHidesAnUnknownError(t *testing.T) {
	cause := errors.New(`pq: duplicate key value violates unique constraint "products_sku_key" ` +
		`DETAIL: Key (sku)=(NJ-P-0042) already exists.`)

	f := AsFailure(cause)

	if f.Status != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", f.Status)
	}
	if f.Code != CodeInternal {
		t.Errorf("code = %q, want %q", f.Code, CodeInternal)
	}
	// The client-visible message must carry none of the cause. A driver error can quote a
	// query, and a query can quote a customer's data.
	if strings.Contains(f.Message, "pq:") || strings.Contains(f.Message, "NJ-P-0042") {
		t.Errorf("message leaked the cause: %q", f.Message)
	}
	// The cause stays reachable for logs and errors.Is, just not for the wire.
	if !errors.Is(f.Unwrap(), cause) {
		t.Error("the cause should remain reachable through Unwrap")
	}
}

func TestAsFailureUnwrapsAFailureItWasGiven(t *testing.T) {
	in := Conflict(CodeQuoteStale, "Quote has expired.")
	out := AsFailure(in)
	if out.Code != CodeQuoteStale || out.Status != http.StatusConflict {
		t.Errorf("AsFailure rewrote a Failure it should have passed through: %+v", out)
	}
}

func TestFailureWrappedByTheCallerStillRendersWithItsOwnStatus(t *testing.T) {
	// fmt.Errorf("%w") is how a use case adds context on the way out. errors.As must see
	// through it, or every wrapped domain error becomes a generic 500.
	wrapped := fmt.Errorf("placing order: %w", NotFound("product"))

	f := AsFailure(wrapped)
	if f.Status != http.StatusNotFound || f.Code != CodeNotFound {
		t.Fatalf("wrapped failure rendered as %d/%q", f.Status, f.Code)
	}
}

func TestForbiddenNamesTheMissingPermission(t *testing.T) {
	f := Forbidden("catalogue.products.write.price")
	if f.Missing != "catalogue.products.write.price" {
		t.Fatalf("Missing = %q", f.Missing)
	}
	if f.Status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", f.Status)
	}
}

func TestEnvelopeJSONMatchesTheContractShape(t *testing.T) {
	f := Validation(Detail{Field: "email", Issue: "must be a valid address"})
	f.Missing = "catalogue.products.read"
	f.Details = append(f.Details, Detail{ProductID: 42, Requested: 5, Available: 2})

	env := EnvelopeFor(f, "3f6a1c2e-0000-4000-8000-aaaaaaaaaaaa")

	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)

	for _, want := range []string{
		`"code":"VALIDATION_FAILED"`,
		`"correlation_id":"3f6a1c2e-0000-4000-8000-aaaaaaaaaaaa"`,
		`"missing_permission":"catalogue.products.read"`,
		`"field":"email"`,
		`"product_id":42`,
		`"requested":5`,
		`"available":2`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("envelope %s is missing %s", got, want)
		}
	}
	// Detail keys are optional per variant: a field error carries no product, and emitting
	// product_id:0 there would tell a client about a product that does not exist.
	if strings.Contains(got, `"product_id":0`) {
		t.Errorf("zero-valued detail keys should be omitted: %s", got)
	}
}

// The contract types `details` as an array and a client should be able to iterate it without
// a null check, so an opaque failure still serialises the key.
func TestDetailsIsAlwaysPresentAndAlwaysAnArray(t *testing.T) {
	for _, f := range []Failure{NotFound("order"), Unauthorized("no session"), Internal(errors.New("cause"))} {
		raw, err := json.Marshal(EnvelopeFor(f, "c"))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		if _, ok := m["details"].([]any); !ok {
			t.Errorf("%s serialised without an array `details`: %s", f.Code, raw)
		}
	}
}

func TestInternalCauseIsNeverSerialized(t *testing.T) {
	f := Internal(errors.New("smtp: 535 5.7.8 Username and Password not accepted for user aws_smtp_secret"))

	// The unexported field is unreachable to encoding/json, but Error() does include it — so
	// the value that must never reach a response body is the one that must never be marshalled.
	env := EnvelopeFor(f, "abc")
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "aws_smtp_secret") {
		t.Fatalf("credential appeared in a response body: %s", raw)
	}
	if !strings.Contains(f.Error(), "aws_smtp_secret") {
		t.Error("the cause should be present for the log line")
	}
}

func TestEveryCodeHasADistinctWireValue(t *testing.T) {
	// Two codes sharing a string would make a client's switch statement ambiguous, and the
	// contract enumerates these values, so a collision is a breaking change to the API.
	seen := map[Code]bool{
		CodeValidationFailed: true, CodePermissionDenied: true, CodeNotAuthenticated: true,
		CodeNotFound: true, CodeMethodNotAllowed: true, CodeConflict: true,
		CodeStockInsufficient: true, CodeQuoteStale: true, CodeTransitionNotAllowed: true,
		CodeOverrideReasonRequired: true, CodeMethodConflict: true, CodeIdempotencyConflict: true,
		CodePriceChangeNotAllowed: true, CodeProductHasHistory: true,
		CodePaymentAlreadySettled: true, CodeRateLimited: true, CodeProviderUnavailable: true,
		CodeInternal: true,
	}
	if len(seen) != 18 {
		t.Fatalf("expected 18 distinct codes, got %d — a constant was duplicated", len(seen))
	}
}
