package httpordering

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/authz"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/ordering"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/testhelper"
)

type fakeOrderingService struct {
	calls int

	placeErr      error
	placeReplayed bool
	placeKey      string
	placeReq      ordering.PlaceOrderRequest
	placeOut      ordering.ContractOrderDetail

	getErr   error
	getRef   string
	getOrder ordering.Order
	getItems []ordering.OrderItem

	listOrders []ordering.Order
	listTotal  int64
	listErr    error

	processErr    error
	processTo     ordering.OrderStatus
	processReason string
	processOut    ordering.Order

	trackOut ordering.OrderTracking
	trackErr error
	trackRef string
}

var _ Service = (*fakeOrderingService)(nil)

func (f *fakeOrderingService) TrackOrder(ctx context.Context, ref, phone string) (ordering.OrderTracking, error) {
	f.calls++
	f.trackRef = ref
	return f.trackOut, f.trackErr
}

func (f *fakeOrderingService) PlaceOrder(ctx context.Context, caller identity.Principal, idemKey string, req ordering.PlaceOrderRequest) (ordering.ContractOrderDetail, bool, error) {
	f.calls++
	f.placeKey = idemKey
	f.placeReq = req
	return f.placeOut, f.placeReplayed, f.placeErr
}
func (f *fakeOrderingService) GetOrder(ctx context.Context, caller identity.Principal, ref string) (ordering.Order, []ordering.OrderItem, error) {
	f.calls++
	f.getRef = ref
	return f.getOrder, f.getItems, f.getErr
}
func (f *fakeOrderingService) ListOrders(ctx context.Context, caller identity.Principal, status *ordering.OrderStatus, page, pageSize int) ([]ordering.Order, int64, error) {
	f.calls++
	return f.listOrders, f.listTotal, f.listErr
}
func (f *fakeOrderingService) ProcessStatus(ctx context.Context, caller identity.Principal, ref string, to ordering.OrderStatus, reason string) (ordering.Order, error) {
	f.calls++
	f.processTo = to
	f.processReason = reason
	return f.processOut, f.processErr
}

func signIn(p identity.Principal) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(identity.WithPrincipal(c.Request.Context(), p))
		c.Next()
	}
}

func newRouter(t *testing.T, svc Service, extra ...gin.HandlerFunc) *gin.Engine {
	t.Helper()
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	api.Use(extra...)
	Mount(api, Deps{Ordering: svc, Authorizer: authz.RBAC{Logger: l}, Logger: l})
	return r
}

func post(t *testing.T, r *gin.Engine, path, body string, headers ...[2]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for _, h := range headers {
		req.Header.Set(h[0], h[1])
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func get(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) apierr.Envelope {
	t.Helper()
	var e apierr.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("body is not the error envelope: %d %s", rec.Code, rec.Body.String())
	}
	return e
}

func TestPlaceOrderWithoutASessionAnswersUnauthorized(t *testing.T) {
	svc := &fakeOrderingService{}
	r := newRouter(t, svc)
	rec := post(t, r, "/api/v1/orders", `{"quote_id":1}`, [2]string{"Idempotency-Key", "k1"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401: %s", rec.Code, rec.Body.String())
	}
}

func TestPlaceOrderRequiresIdempotencyKey(t *testing.T) {
	svc := &fakeOrderingService{}
	r := newRouter(t, svc, signIn(identity.Principal{User: identity.User{ID: 42}}))
	rec := post(t, r, "/api/v1/orders", `{"quote_id":1}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestPlaceOrderAnswersCreatedWithDetail(t *testing.T) {
	svc := &fakeOrderingService{placeOut: ordering.ContractOrderDetail{
		ID: 9, Reference: "NJ-2026-0001", Status: "pending", PaymentStatus: "pending",
		Totals: ordering.Totals{SubtotalCents: 30000, DeliveryFeeCents: 15000, TotalCents: 45000, Currency: "KES"},
	}}
	r := newRouter(t, svc, signIn(identity.Principal{User: identity.User{ID: 42}}))
	rec := post(t, r, "/api/v1/orders", `{"quote_id":7,"notes":"leave at gate"}`, [2]string{"Idempotency-Key", "idem-1"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "NJ-2026-0001") {
		t.Errorf("body missing reference: %s", rec.Body.String())
	}
	if svc.placeKey != "idem-1" || svc.placeReq.QuoteID != 7 || svc.placeReq.Notes == nil || *svc.placeReq.Notes != "leave at gate" {
		t.Errorf("placement not forwarded correctly: key=%q req=%+v", svc.placeKey, svc.placeReq)
	}
}

func TestPlaceOrderConflictIsRendered(t *testing.T) {
	svc := &fakeOrderingService{placeErr: apierr.StockInsufficient([]apierr.InsufficientItem{{ProductID: 3, Name: "Bulb", Requested: 2, Available: 1}})}
	r := newRouter(t, svc, signIn(identity.Principal{User: identity.User{ID: 42}}))
	rec := post(t, r, "/api/v1/orders", `{"quote_id":7}`, [2]string{"Idempotency-Key", "idem-1"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409: %s", rec.Code, rec.Body.String())
	}
	e := decodeEnvelope(t, rec)
	if e.Code != apierr.CodeStockInsufficient {
		t.Errorf("code %q, want %q", e.Code, apierr.CodeStockInsufficient)
	}
	if len(e.Insufficient) != 1 || e.Insufficient[0].ProductID != 3 {
		t.Errorf("insufficient not rendered: %+v", e.Insufficient)
	}
}

func TestGetOrderWithoutASessionAnswersUnauthorized(t *testing.T) {
	svc := &fakeOrderingService{}
	r := newRouter(t, svc)
	rec := get(t, r, "/api/v1/orders/NJ-2026-0001")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401: %s", rec.Code, rec.Body.String())
	}
}

func TestGetOrderAnswersTheOrder(t *testing.T) {
	svc := &fakeOrderingService{getOrder: ordering.Order{ID: 9, Reference: "NJ-2026-0001", Status: ordering.StatusPending, PaymentStatus: ordering.PayPending, Currency: "KES"}}
	r := newRouter(t, svc, signIn(identity.Principal{User: identity.User{ID: 42}}))
	rec := get(t, r, "/api/v1/orders/NJ-2026-0001")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "NJ-2026-0001") {
		t.Errorf("body missing reference: %s", rec.Body.String())
	}
}

func TestGetOrderAnswersNotFoundWhenTheServiceDoesNotKnowIt(t *testing.T) {
	svc := &fakeOrderingService{getErr: ordering.ErrNoOrder}
	r := newRouter(t, svc, signIn(identity.Principal{User: identity.User{ID: 42}}))
	rec := get(t, r, "/api/v1/orders/NJ-9999-9999")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404: %s", rec.Code, rec.Body.String())
	}
	e := decodeEnvelope(t, rec)
	if e.Code != apierr.CodeNotFound {
		t.Errorf("code %q, want %q", e.Code, apierr.CodeNotFound)
	}
}

func TestListOrdersAnswersAPage(t *testing.T) {
	svc := &fakeOrderingService{listOrders: []ordering.Order{{ID: 9, Reference: "NJ-2026-0001"}}, listTotal: 1}
	r := newRouter(t, svc, signIn(identity.Principal{User: identity.User{ID: 42}}))
	rec := get(t, r, "/api/v1/orders?page=2&page_size=5")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "NJ-2026-0001") {
		t.Errorf("body missing reference: %s", rec.Body.String())
	}
}

func TestListOrdersWithoutASessionAnswersUnauthorized(t *testing.T) {
	svc := &fakeOrderingService{}
	r := newRouter(t, svc)
	rec := get(t, r, "/api/v1/orders")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401: %s", rec.Code, rec.Body.String())
	}
}

func TestProcessStatusWithoutASessionAnswersUnauthorized(t *testing.T) {
	svc := &fakeOrderingService{}
	r := newRouter(t, svc)
	rec := post(t, r, "/api/v1/orders/NJ-2026-0001/status", `{"status":"paid"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401: %s", rec.Code, rec.Body.String())
	}
}

func TestProcessStatusForwardsTheTransition(t *testing.T) {
	svc := &fakeOrderingService{processOut: ordering.Order{ID: 9, Reference: "NJ-2026-0001"}}
	r := newRouter(t, svc, signIn(identity.Principal{User: identity.User{ID: 42}}))
	rec := post(t, r, "/api/v1/orders/NJ-2026-0001/status", `{"status":"paid","reason":"mpesa confirmation received"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if svc.processTo != ordering.StatusPaid {
		t.Errorf("transition not forwarded: %v", svc.processTo)
	}
	if svc.processReason != "mpesa confirmation received" {
		t.Errorf("reason not forwarded: %q", svc.processReason)
	}
}
