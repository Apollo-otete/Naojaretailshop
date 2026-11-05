package httpjson

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/observability"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/testhelper"
)

func init() { gin.SetMode(gin.TestMode) }

// do runs one request through a one-route engine whose handler body is h. The logger and the
// buffer it writes to are created here and passed to h, so a test can assert on both the
// response and the log record produced by the same call.
func do(t *testing.T, h func(*gin.Context, *slog.Logger)) (*http.Response, string, *testhelper.Buffer) {
	t.Helper()
	l, buf := testhelper.Logger(t, slog.LevelDebug)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(observability.WithRequestID(c.Request.Context(), "corr-1"))
		c.Next()
	})
	r.GET("/x", func(c *gin.Context) { h(c, l) })

	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/x", nil))

	out := resp.Result()
	body, err := io.ReadAll(out.Body)
	if err != nil {
		t.Fatal(err)
	}
	out.Body.Close()
	return out, string(body), buf
}

func TestWriteRendersTheContractEnvelopeWithTheCorrelationID(t *testing.T) {
	resp, body, _ := do(t, func(c *gin.Context, l *slog.Logger) {
		Write(c, l, apierr.New(http.StatusUnprocessableEntity, apierr.CodeValidationFailed,
			"The request did not pass validation.", apierr.Detail{Field: "phone", Issue: "must be a Kenyan number"}))
	})

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content-type = %q", ct)
	}

	var env apierr.Envelope
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("body is not an Error envelope: %v\n%s", err, body)
	}
	if env.Code != apierr.CodeValidationFailed {
		t.Errorf("code = %q", env.Code)
	}
	// The client gets the same id the server logged; without it a support conversation about
	// "the order that vanished" has nothing to search on.
	if env.CorrelationID != "corr-1" {
		t.Errorf("correlation_id = %q, want the request id", env.CorrelationID)
	}
	if len(env.Details) != 1 || env.Details[0].Field != "phone" {
		t.Errorf("details = %+v", env.Details)
	}
}

// A null in place of an empty list forces every consumer to write two checks. The contract
// says details is an array; the renderer guarantees it.
func TestDetailsSerializeAsAnEmptyArrayRatherThanNull(t *testing.T) {
	_, body, _ := do(t, func(c *gin.Context, l *slog.Logger) { Write(c, l, apierr.NotFound("order")) })
	if !strings.Contains(body, `"details":[]`) {
		t.Fatalf("expected an empty details array, got %s", body)
	}
}

func TestRetryAfterIsSetOnlyWhenTheCallerSuppliesIt(t *testing.T) {
	throttled := apierr.Conflict(apierr.CodeRateLimited, "Too many requests.")
	throttled.RetryIn = 30

	resp, _, _ := do(t, func(c *gin.Context, l *slog.Logger) { Write(c, l, throttled) })
	if got := resp.Header.Get("Retry-After"); got != "30" {
		t.Errorf("Retry-After = %q, want 30", got)
	}

	resp2, _, _ := do(t, func(c *gin.Context, l *slog.Logger) {
		Write(c, l, apierr.Conflict(apierr.CodeQuoteStale, "Quote expired."))
	})
	if got := resp2.Header.Get("Retry-After"); got != "" {
		t.Errorf("Retry-After should be absent, got %q", got)
	}
}

func TestForbiddenCarriesTheMissingPermissionToTheClient(t *testing.T) {
	_, body, _ := do(t, func(c *gin.Context, l *slog.Logger) {
		Write(c, l, apierr.Forbidden("payments.refund.approve"))
	})
	var env apierr.Envelope
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatal(err)
	}
	if env.MissingPermission != "payments.refund.approve" {
		t.Fatalf("missing_permission = %q — a client cannot fix a role gap it cannot name", env.MissingPermission)
	}
}

func TestInternalErrorsAreLoggedWithTheirCauseButNeverSent(t *testing.T) {
	cause := errors.New(`dial tcp 10.0.0.7:5432: connect: connection refused (user "naoja_owner" password "hunter2")`)

	resp, body, buf := do(t, func(c *gin.Context, l *slog.Logger) { Fail(c, l, cause) })

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if strings.Contains(body, "hunter2") || strings.Contains(body, "10.0.0.7") {
		t.Fatalf("the response body leaked the cause: %s", body)
	}

	rec := buf.Find(t, "request failed")
	if rec == nil {
		t.Fatalf("expected one Error record; got:\n%s", buf)
	}
	// The client was handed this id in its error body; the log line is unfindable without it.
	if rec["request_id"] != "corr-1" {
		t.Errorf("rejection log has request_id = %v, want corr-1", rec["request_id"])
	}
	if rec["level"] != "ERROR" {
		t.Errorf("level = %v, want ERROR for a 5xx", rec["level"])
	}
	if !strings.Contains(asText(rec["error"]), "hunter2") {
		t.Errorf("the log line should keep the cause for whoever is debugging: %v", rec["error"])
	}
	// route and status are on the same record, so one grep finds one failure.
	if rec["status"] != float64(500) || rec["code"] != string(apierr.CodeInternal) {
		t.Errorf("record = %v", rec)
	}
}

func TestClientErrorsAreLoggedAtInfoNotError(t *testing.T) {
	// A customer typing a bad phone number is not an incident. Paging on it teaches whoever
	// reads the logs to ignore them.
	_, _, buf := do(t, func(c *gin.Context, l *slog.Logger) {
		Fail(c, l, apierr.Validation(apierr.Detail{Field: "phone", Issue: "invalid"}))
	})

	rec := buf.Find(t, "request rejected")
	if rec == nil {
		t.Fatalf("expected a 'request rejected' record; got:\n%s", buf)
	}
	if rec["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", rec["level"])
	}
}

// Once a status line has gone out there is no correct second body: writing one would append a
// JSON object to a response the client has already parsed. Fail must be a no-op, not a corruptor.
func TestWriteIsANoOpAfterTheResponseHasStarted(t *testing.T) {
	resp, body, _ := do(t, func(c *gin.Context, l *slog.Logger) {
		c.JSON(http.StatusAccepted, gin.H{"ok": true})
		Write(c, l, apierr.Internal(errors.New("late failure")))
	})

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status changed after the body started: %d", resp.StatusCode)
	}
	if strings.Contains(body, "INTERNAL_ERROR") {
		t.Fatalf("a second body was appended: %s", body)
	}
	if strings.TrimSpace(body) != `{"ok":true}` {
		t.Fatalf("body was altered: %q", body)
	}
}

func asText(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if v == nil {
		return ""
	}
	b, _ := json.Marshal(v)
	return string(b)
}
