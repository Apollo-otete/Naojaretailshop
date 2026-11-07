package httpshopping

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/authz"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/shopping"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/testhelper"
)

// These tests hold the transport to contracts/openapi.yaml's Cart section: the mounted operation
// set is pinned to the contract, the guest session token is the only thing a cart needs before it
// asks a use case, and every refusal leaves the one error shape behind.

const cartCookieName = "naoja_cart"

// The fake use case stands in for the Shopping service, so the assertions that matter here are
// about what the transport asked for and what it answered, not about the store behind it.

type addedCall struct {
	caller       *identity.Principal
	sessionToken string
	productID    int64
	qty          int
}

type updatedCall struct {
	caller       *identity.Principal
	sessionToken string
	productID    int64
	qty          int
}

type removedCall struct {
	caller       *identity.Principal
	sessionToken string
	productID    int64
}

type fakeShoppingService struct {
	mu sync.Mutex

	calls []string

	view      shopping.CartView
	viewErr   error
	addErr    error
	updateErr error

	added   []addedCall
	updated []updatedCall
	removed []removedCall
	cleared int

	viewCalls  int
	lastCaller *identity.Principal
	lastToken  string
}

var _ Service = (*fakeShoppingService)(nil)

func (f *fakeShoppingService) ViewCart(ctx context.Context, caller *identity.Principal, sessionToken string) (shopping.CartView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "ViewCart")
	f.viewCalls++
	f.lastCaller = caller
	f.lastToken = sessionToken
	if f.viewErr != nil {
		return shopping.CartView{}, f.viewErr
	}
	if caller == nil && strings.TrimSpace(sessionToken) == "" {
		return shopping.CartView{}, shopping.ErrGuestSessionRequired
	}
	return f.view, nil
}

func (f *fakeShoppingService) AddItem(ctx context.Context, caller *identity.Principal, sessionToken string, productID int64, qty int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "AddItem")
	f.added = append(f.added, addedCall{caller: caller, sessionToken: sessionToken, productID: productID, qty: qty})
	if caller == nil && strings.TrimSpace(sessionToken) == "" {
		return shopping.ErrGuestSessionRequired
	}
	return f.addErr
}

func (f *fakeShoppingService) UpdateItem(ctx context.Context, caller *identity.Principal, sessionToken string, productID int64, qty int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "UpdateItem")
	f.updated = append(f.updated, updatedCall{caller: caller, sessionToken: sessionToken, productID: productID, qty: qty})
	if caller == nil && strings.TrimSpace(sessionToken) == "" {
		return shopping.ErrGuestSessionRequired
	}
	return f.updateErr
}

func (f *fakeShoppingService) RemoveItem(ctx context.Context, caller *identity.Principal, sessionToken string, productID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "RemoveItem")
	f.removed = append(f.removed, removedCall{caller: caller, sessionToken: sessionToken, productID: productID})
	if caller == nil && strings.TrimSpace(sessionToken) == "" {
		return shopping.ErrGuestSessionRequired
	}
	return nil
}

func (f *fakeShoppingService) ClearCart(ctx context.Context, caller *identity.Principal, sessionToken string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "ClearCart")
	f.cleared++
	if caller == nil && strings.TrimSpace(sessionToken) == "" {
		return shopping.ErrGuestSessionRequired
	}
	return nil
}

func newFakeShopping() *fakeShoppingService {
	return &fakeShoppingService{view: shopping.CartView{Items: []shopping.CartLine{}}}
}

func newRouter(t *testing.T, svc Service, extra ...gin.HandlerFunc) *gin.Engine {
	t.Helper()
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	api.Use(extra...)
	Mount(api, Deps{Shopping: svc, Authorizer: authz.RBAC{Logger: l}, Logger: l})
	return r
}

func do(t *testing.T, r *gin.Engine, method, path, body, cookie string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: cartCookieName, Value: cookie})
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
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

func signIn(p identity.Principal) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(identity.WithPrincipal(c.Request.Context(), p))
		c.Next()
	}
}

func cartCaller() *identity.Principal {
	p := identity.Principal{
		User:        identity.User{ID: 42, FullName: "Achieng' Otieno", Account: identity.KindCustomer},
		SessionID:   7,
		CSRFToken:   "csrf-for-this-session",
		Roles:       []string{"customer"},
		Permissions: []string{"shopping.cart.read", "shopping.cart.modify"},
		Conditions: map[string][]string{
			"shopping.cart.read":   {authz.ConditionOwnerIsCaller},
			"shopping.cart.modify": {authz.ConditionOwnerIsCaller},
		},
	}
	return &p
}

// ── the mount is the contract ──────────────────────────────────────────────────────────

func TestTheMountedOperationsAreExactlyTheContractedOnes(t *testing.T) {
	r := newRouter(t, newFakeShopping())

	want := map[string]bool{
		"GET /api/v1/cart":                      true,
		"DELETE /api/v1/cart":                   true,
		"POST /api/v1/cart/items":               true,
		"PATCH /api/v1/cart/items/:product_id":  true,
		"DELETE /api/v1/cart/items/:product_id": true,
	}
	got := map[string]bool{}
	for _, rt := range r.Routes() {
		got[rt.Method+" "+rt.Path] = true
	}
	if len(got) != len(want) {
		t.Errorf("mounted %v, want exactly the contracted set %v", keysOf(got), keysOf(want))
	}
	for k := range want {
		if !got[k] {
			t.Errorf("mounted operation %q is missing from the contract surface", k)
		}
	}
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// ── the read ────────────────────────────────────────────────────────────────────────

func TestGetCartWithoutASessionAnswersBadRequest(t *testing.T) {
	svc := newFakeShopping()
	r := newRouter(t, svc)

	rec := do(t, r, http.MethodGet, "/api/v1/cart", "", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400: %s", rec.Code, rec.Body.String())
	}
	e := decodeEnvelope(t, rec)
	if e.Code != apierr.CodeBadRequest {
		t.Errorf("code %q, want %q", e.Code, apierr.CodeBadRequest)
	}
}

func TestGetCartAnswersTheCartItIsGiven(t *testing.T) {
	svc := newFakeShopping()
	svc.view = shopping.CartView{
		Items:         []shopping.CartLine{{ProductID: 5, SKU: "ABC-1", Name: "Fridge", Quantity: 2, UnitPriceCents: 15000, LineTotalCents: 30000}},
		SubtotalCents: 30000,
		ItemCount:     2,
	}
	r := newRouter(t, svc)

	rec := do(t, r, http.MethodGet, "/api/v1/cart", "", "guest-token")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"subtotal_cents"`) || !strings.Contains(rec.Body.String(), `"item_count"`) {
		t.Errorf("the cart body is not the contract Cart shape: %s", rec.Body.String())
	}

	svc.mu.Lock()
	calls := append([]string(nil), svc.calls...)
	svc.mu.Unlock()
	if len(calls) != 1 || calls[0] != "ViewCart" {
		t.Errorf("the read reached the use case %v, want exactly one ViewCart", calls)
	}
}

func TestGetCartForASignedInCallerForwardsTheCallerAndTheToken(t *testing.T) {
	svc := newFakeShopping()
	r := newRouter(t, svc, signIn(*cartCaller()))

	rec := do(t, r, http.MethodGet, "/api/v1/cart", "", "guest-token")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body.String())
	}

	svc.mu.Lock()
	caller, token, calls := svc.lastCaller, svc.lastToken, append([]string(nil), svc.calls...)
	svc.mu.Unlock()
	if caller == nil || caller.UserID() != 42 {
		t.Errorf("the read was not handed the signed-in caller: %+v", caller)
	}
	if token != "guest-token" {
		t.Errorf("the read was not handed the cart session token: %q", token)
	}
	if len(calls) != 1 || calls[0] != "ViewCart" {
		t.Errorf("the read reached the use case %v", calls)
	}
}

// ── the add ─────────────────────────────────────────────────────────────────────────

func TestAddToCartAnswersCreatedWithTheCart(t *testing.T) {
	svc := newFakeShopping()
	svc.view = shopping.CartView{Items: []shopping.CartLine{}, SubtotalCents: 0, ItemCount: 0}
	r := newRouter(t, svc)

	rec := do(t, r, http.MethodPost, "/api/v1/cart/items", `{"product_id":5,"quantity":2}`, "guest-token")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, want 201: %s", rec.Code, rec.Body.String())
	}

	svc.mu.Lock()
	added, calls := svc.added, append([]string(nil), svc.calls...)
	svc.mu.Unlock()
	if len(added) != 1 || added[0].productID != 5 || added[0].qty != 2 {
		t.Errorf("the add was not forwarded: %+v", added)
	}
	if added[0].sessionToken != "guest-token" {
		t.Errorf("the add was not handed the guest session token: %q", added[0].sessionToken)
	}
	if len(calls) != 2 || calls[0] != "AddItem" || calls[1] != "ViewCart" {
		t.Errorf("the add should read the cart back afterwards, in %v", calls)
	}
	if !strings.Contains(rec.Body.String(), `"subtotal_cents"`) {
		t.Errorf("the 201 body is not the cart: %s", rec.Body.String())
	}
}

func TestAddToCartWithoutASessionAnswersBadRequest(t *testing.T) {
	svc := newFakeShopping()
	r := newRouter(t, svc)

	rec := do(t, r, http.MethodPost, "/api/v1/cart/items", `{"product_id":5,"quantity":2}`, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestAnAddRefusalIsTheErrorEnvelope(t *testing.T) {
	svc := newFakeShopping()
	svc.addErr = errors.New("the pool is closed")
	r := newRouter(t, svc)

	rec := do(t, r, http.MethodPost, "/api/v1/cart/items", `{"product_id":5,"quantity":2}`, "guest-token")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500: %s", rec.Code, rec.Body.String())
	}
	e := decodeEnvelope(t, rec)
	if e.Code != apierr.CodeInternal {
		t.Errorf("code %q, want %q", e.Code, apierr.CodeInternal)
	}
	if strings.Contains(rec.Body.String(), "pool is closed") {
		t.Errorf("the internal cause reached the client: %s", rec.Body.String())
	}
}

// ── the update ──────────────────────────────────────────────────────────────────────

func TestPatchCartItemForwardsTheQuantityAndReturnsTheCart(t *testing.T) {
	svc := newFakeShopping()
	svc.view = shopping.CartView{Items: []shopping.CartLine{}, SubtotalCents: 0, ItemCount: 0}
	r := newRouter(t, svc)

	rec := do(t, r, http.MethodPatch, "/api/v1/cart/items/5", `{"quantity":3}`, "guest-token")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body.String())
	}

	svc.mu.Lock()
	updated, calls := svc.updated, append([]string(nil), svc.calls...)
	svc.mu.Unlock()
	if len(updated) != 1 || updated[0].productID != 5 || updated[0].qty != 3 {
		t.Errorf("the update was not forwarded: %+v", updated)
	}
	if len(calls) != 2 || calls[0] != "UpdateItem" || calls[1] != "ViewCart" {
		t.Errorf("the update should read the cart back afterwards, in %v", calls)
	}
}

func TestPatchCartItemAnswersBadRequestForAnInvalidProductID(t *testing.T) {
	svc := newFakeShopping()
	r := newRouter(t, svc)

	rec := do(t, r, http.MethodPatch, "/api/v1/cart/items/not-an-id", `{"quantity":3}`, "guest-token")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400: %s", rec.Code, rec.Body.String())
	}

	svc.mu.Lock()
	calls := append([]string(nil), svc.calls...)
	svc.mu.Unlock()
	if len(calls) != 0 {
		t.Errorf("the use case ran for an unparsable product id: %v", calls)
	}
}

// ── the remove ──────────────────────────────────────────────────────────────────────

func TestRemoveCartItemAnswersNoContent(t *testing.T) {
	svc := newFakeShopping()
	r := newRouter(t, svc)

	rec := do(t, r, http.MethodDelete, "/api/v1/cart/items/5", "", "guest-token")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d, want 204: %s", rec.Code, rec.Body.String())
	}

	svc.mu.Lock()
	removed := svc.removed
	svc.mu.Unlock()
	if len(removed) != 1 || removed[0].productID != 5 {
		t.Errorf("the line was not removed: %+v", removed)
	}
}

// ── the clear ───────────────────────────────────────────────────────────────────────

func TestClearCartAnswersNoContent(t *testing.T) {
	svc := newFakeShopping()
	r := newRouter(t, svc)

	rec := do(t, r, http.MethodDelete, "/api/v1/cart", "", "guest-token")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d, want 204: %s", rec.Code, rec.Body.String())
	}

	svc.mu.Lock()
	cleared := svc.cleared
	svc.mu.Unlock()
	if cleared != 1 {
		t.Errorf("the cart was cleared %d times, want once", cleared)
	}
}

func TestClearCartWithoutASessionAnswersBadRequest(t *testing.T) {
	svc := newFakeShopping()
	r := newRouter(t, svc)

	rec := do(t, r, http.MethodDelete, "/api/v1/cart", "", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400: %s", rec.Code, rec.Body.String())
	}
}
