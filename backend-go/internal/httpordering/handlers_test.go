package httpordering

import (
	"context"
	"encoding/json"
	"errors"
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

// These tests hold the transport to contracts/openapi.yaml's Orders section, for the operations
// the slice has landed: the create-from-cart placement, the read of one order, the list, and the
// status transition. The payment and tracking operations are the next slices.

type fakeOrderingService struct {
	calls int

	createErr error
	createIn  ordering.CreateOrderInput
	createOut ordering.Order

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
}

var _ Service = (*fakeOrderingService)(nil)

func (f *fakeOrderingService) CreateFromCart(ctx context.Context, caller identity.Principal, sessionToken string, in ordering.CreateOrderInput) (ordering.Order, error) {
	f.calls++
	f.createIn = in
	return f.createOut, f.createErr
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

func post(t *testing.T, r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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

func TestCreateOrderWithoutASessionAnswersUnauthorized(t *testing.T) {
	svc := &fakeOrderingService{}
	r := newRouter(t, svc)

	rec := post(t, r, "/api/v1/orders", `{"delivery_fee_cents":15000}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401: %s", rec.Code, rec.Body.String())
	}
	e := decodeEnvelope(t, rec)
	if e.Code != apierr.CodeNotAuthenticated {
		t.Errorf("code %q, want %q", e.Code, apierr.CodeNotAuthenticated)
	}
}

func TestCreateOrderAnswersCreatedWithTheOrder(t *testing.T) {
	svc := &fakeOrderingService{createOut: ordering.Order{
		ID: 9, Reference: "NJ-2026-0001", Status: ordering.StatusPendingPayment,
		PaymentStatus: ordering.PayPending, SubtotalCents: 30000, DeliveryFeeCents: 15000, TotalCents: 45000,
		Currency: "KES",
	}}
	r := newRouter(t, svc, signIn(identity.Principal{User: identity.User{ID: 42}}))

	rec := post(t, r, "/api/v1/orders", `{"delivery_fee_cents":15000}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "NJ-2026-0001") {
		t.Errorf("the 201 body is not the placed order: %s", rec.Body.String())
	}
}

func TestCreateOrderForwardsTheDeliveryInput(t *testing.T) {
	svc := &fakeOrderingService{}
	r := newRouter(t, svc, signIn(identity.Principal{User: identity.User{ID: 42}}))

	rec := post(t, r, "/api/v1/orders",
		`{"delivery_address":"Kenyan Road, Nairobi","contact_phone":"+254700000000","delivery_instructions":"Gate code 1234","delivery_fee_cents":15000}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if svc.createIn.DeliveryAddress == nil || *svc.createIn.DeliveryAddress != "Kenyan Road, Nairobi" {
		t.Errorf("delivery address was not forwarded: %+v", svc.createIn)
	}
	if svc.createIn.ContactPhone == nil || *svc.createIn.ContactPhone != "+254700000000" {
		t.Errorf("contact phone was not forwarded: %+v", svc.createIn)
	}
	if svc.createIn.DeliveryInstructions == nil || *svc.createIn.DeliveryInstructions != "Gate code 1234" {
		t.Errorf("delivery instructions were not forwarded: %+v", svc.createIn)
	}
	if svc.createIn.DeliveryFeeCents != 15000 {
		t.Errorf("delivery fee was not forwarded: %+v", svc.createIn)
	}
}

func TestAnOrderRefusalIsTheErrorEnvelope(t *testing.T) {
	svc := &fakeOrderingService{createErr: errors.New("the pool is closed")}
	r := newRouter(t, svc, signIn(identity.Principal{User: identity.User{ID: 42}}))

	rec := post(t, r, "/api/v1/orders", `{"delivery_fee_cents":15000}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500: %s", rec.Code, rec.Body.String())
	}
	e := decodeEnvelope(t, rec)
	if e.Code != apierr.CodeInternal {
		t.Errorf("code %q, want %q", e.Code, apierr.CodeInternal)
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
	svc := &fakeOrderingService{getOrder: ordering.Order{ID: 9, Reference: "NJ-2026-0001", Status: ordering.StatusPendingPayment}}
	r := newRouter(t, svc, signIn(identity.Principal{User: identity.User{ID: 42}}))

	rec := get(t, r, "/api/v1/orders/NJ-2026-0001")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "NJ-2026-0001") {
		t.Errorf("the 200 body is not the order: %s", rec.Body.String())
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
		t.Errorf("the 200 body is not the listed orders: %s", rec.Body.String())
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

	rec := post(t, r, "/api/v1/orders/NJ-2026-0001/status",
		`{"status":"paid","reason":"mpesa confirmation received"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if svc.processTo != ordering.StatusPaid {
		t.Errorf("the transition was not forwarded: %v", svc.processTo)
	}
	if svc.processReason != "mpesa confirmation received" {
		t.Errorf("the reason was not forwarded: %q", svc.processReason)
	}
}
