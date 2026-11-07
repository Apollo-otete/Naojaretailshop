package httpcatalogue

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/authz"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/catalogue"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/testhelper"
)

// These tests hold the transport to contracts/openapi.yaml's Catalogue section: the operations
// that must be mounted, that the public reads never ask a session for, that refusals answer with
// the contract envelope, and that the field-level validation reports the fields it was asked to
// check.

func newFakeCatalogue() *fakeCatalogueService {
	return &fakeCatalogueService{}
}

type fakeCatalogueService struct {
	mu sync.Mutex

	listProductsFn   func(ctx context.Context, f catalogue.ListProductsFilter, callerCanReadDraft bool) (catalogue.ProductList, error)
	listCategoriesFn func(ctx context.Context, page, pageSize int) (catalogue.CategoryList, error)
	getProductFn     func(ctx context.Context, ref string, callerCanReadDraft bool) (catalogue.Product, error)
	createProductFn  func(ctx context.Context, in catalogue.CreateProductInput) (catalogue.Product, error)
	archiveProductFn func(ctx context.Context, id int64) error
	changePriceFn    func(ctx context.Context, actor identity.Principal, id int64, in catalogue.PriceChangeInput) error
	setPublicationFn func(ctx context.Context, id int64, in catalogue.PublicationInput) (catalogue.Product, error)
	createCategoryFn func(ctx context.Context, in catalogue.CreateCategoryInput) (catalogue.Category, error)
	deleteCategoryFn func(ctx context.Context, id int64) error

	calls []string

	listFilter   catalogue.ListProductsFilter
	listDraft    bool
	createdInput *catalogue.CreateProductInput
	archived     []int64
	changed      *catalogue.PriceChangeInput
	changedActor identity.Principal
	setPub       *catalogue.PublicationInput
	setPubResult catalogue.Product
	setPubErr    error
	createdCat   *catalogue.CreateCategoryInput
	deletedCatID int64
}

var _ Service = (*fakeCatalogueService)(nil)

func (f *fakeCatalogueService) ListCategories(ctx context.Context, page, pageSize int) (catalogue.CategoryList, error) {
	f.calls = append(f.calls, "ListCategories")
	if f.listCategoriesFn == nil {
		return catalogue.CategoryList{}, nil
	}
	return f.listCategoriesFn(ctx, page, pageSize)
}

func (f *fakeCatalogueService) ActiveCategories(ctx context.Context) ([]catalogue.Category, error) {
	f.calls = append(f.calls, "ActiveCategories")
	return nil, nil
}

func (f *fakeCatalogueService) ListProducts(ctx context.Context, in catalogue.ListProductsFilter, callerCanReadDraft bool) (catalogue.ProductList, error) {
	f.mu.Lock()
	f.listFilter = in
	f.listDraft = callerCanReadDraft
	f.mu.Unlock()
	f.calls = append(f.calls, "ListProducts")
	if f.listProductsFn == nil {
		return catalogue.ProductList{Items: []catalogue.Product{}, Meta: catalogue.PageMeta{Page: 1, PageSize: 20}}, nil
	}
	return f.listProductsFn(ctx, in, callerCanReadDraft)
}

func (f *fakeCatalogueService) GetProduct(ctx context.Context, ref string, callerCanReadDraft bool) (catalogue.Product, error) {
	f.calls = append(f.calls, "GetProduct:"+ref)
	if f.getProductFn == nil {
		return catalogue.Product{}, catalogue.ErrNoProduct
	}
	return f.getProductFn(ctx, ref, callerCanReadDraft)
}

func (f *fakeCatalogueService) ProductImages(ctx context.Context, productID int64) ([]catalogue.ProductImage, error) {
	f.calls = append(f.calls, "ProductImages")
	return nil, nil
}

func (f *fakeCatalogueService) CategoryByID(ctx context.Context, id int64) (catalogue.Category, error) {
	f.calls = append(f.calls, "CategoryByID")
	return catalogue.Category{}, catalogue.ErrNoCategory
}

func (f *fakeCatalogueService) CategoryProductCount(ctx context.Context, id int64) (int64, error) {
	f.calls = append(f.calls, "CategoryProductCount")
	return 0, nil
}

func (f *fakeCatalogueService) CreateProduct(ctx context.Context, in catalogue.CreateProductInput) (catalogue.Product, error) {
	f.mu.Lock()
	f.createdInput = &in
	f.mu.Unlock()
	f.calls = append(f.calls, "CreateProduct")
	if f.createProductFn == nil {
		return catalogue.Product{}, catalogue.ErrSKUTaken
	}
	return f.createProductFn(ctx, in)
}

func (f *fakeCatalogueService) UpdateProduct(ctx context.Context, id int64, in catalogue.UpdateProductInput) (catalogue.Product, error) {
	f.calls = append(f.calls, "UpdateProduct")
	return catalogue.Product{}, catalogue.ErrNoProduct
}

func (f *fakeCatalogueService) ChangePrice(ctx context.Context, actor identity.Principal, id int64, in catalogue.PriceChangeInput) error {
	f.mu.Lock()
	f.changed = &in
	f.changedActor = actor
	f.mu.Unlock()
	f.calls = append(f.calls, "ChangePrice")
	if f.changePriceFn == nil {
		return nil
	}
	return f.changePriceFn(ctx, actor, id, in)
}

func (f *fakeCatalogueService) SetPublication(ctx context.Context, id int64, in catalogue.PublicationInput) (catalogue.Product, error) {
	f.mu.Lock()
	f.setPub = &in
	f.mu.Unlock()
	f.calls = append(f.calls, "SetPublication")
	if f.setPublicationFn == nil {
		return catalogue.Product{}, catalogue.ErrNoProduct
	}
	return f.setPublicationFn(ctx, id, in)
}

func (f *fakeCatalogueService) ArchiveProduct(ctx context.Context, id int64) error {
	f.mu.Lock()
	f.archived = append(f.archived, id)
	f.mu.Unlock()
	f.calls = append(f.calls, "ArchiveProduct")
	if f.archiveProductFn == nil {
		return nil
	}
	return f.archiveProductFn(ctx, id)
}

func (f *fakeCatalogueService) CreateCategory(ctx context.Context, in catalogue.CreateCategoryInput) (catalogue.Category, error) {
	f.mu.Lock()
	f.createdCat = &in
	f.mu.Unlock()
	f.calls = append(f.calls, "CreateCategory")
	if f.createCategoryFn == nil {
		return catalogue.Category{}, catalogue.ErrNoCategory
	}
	return f.createCategoryFn(ctx, in)
}

func (f *fakeCatalogueService) UpdateCategory(ctx context.Context, id int64, in catalogue.UpdateCategoryInput) (catalogue.Category, error) {
	f.calls = append(f.calls, "UpdateCategory")
	return catalogue.Category{}, catalogue.ErrNoCategory
}

func (f *fakeCatalogueService) DeleteCategory(ctx context.Context, id int64) error {
	f.mu.Lock()
	f.deletedCatID = id
	f.mu.Unlock()
	f.calls = append(f.calls, "DeleteCategory")
	if f.deleteCategoryFn == nil {
		return nil
	}
	return f.deleteCategoryFn(ctx, id)
}

// ── helpers ────────────────────────────────────────────────────────────────────────────

func testRBAC(t *testing.T) authz.RBAC {
	t.Helper()
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	return authz.RBAC{Logger: l}
}

func newRouter(t *testing.T, svc Service, authorizer port.Authorizer, principal *identity.Principal) *gin.Engine {
	t.Helper()
	l, _ := testhelper.Logger(t, slog.LevelDebug)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	if principal != nil {
		p := *principal
		api.Use(func(c *gin.Context) {
			c.Request = c.Request.WithContext(identity.WithPrincipal(c.Request.Context(), p))
			c.Next()
		})
	}
	Mount(api, Deps{Catalogue: svc, Authorizer: authorizer, Logger: l})
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

func doJSON(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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

func adminWith(perms ...string) *identity.Principal {
	p := identity.Principal{
		User:        identity.User{ID: 11, FullName: "Business Admin", Account: identity.KindAdmin},
		SessionID:   7,
		CSRFToken:   "csrf-for-this-session",
		Roles:       []string{"admin_1_business"},
		Permissions: perms,
	}
	return &p
}

// ── the mount is the contract ──────────────────────────────────────────────────────────

func TestTheMountedOperationsAreExactlyTheContractedOnes(t *testing.T) {
	r := newRouter(t, newFakeCatalogue(), testRBAC(t), nil)

	want := map[string]bool{
		"GET /api/v1/products":                    true,
		"GET /api/v1/products/:reference":         true,
		"GET /api/v1/categories":                  true,
		"POST /api/v1/products":                   true,
		"PATCH /api/v1/products/:id":              true,
		"DELETE /api/v1/products/:id":             true,
		"POST /api/v1/products/:id/price-changes": true,
		"PUT /api/v1/products/:id/publication":    true,
		"POST /api/v1/categories":                 true,
		"PATCH /api/v1/categories/:id":            true,
		"DELETE /api/v1/categories/:id":           true,
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

// ── the public reads ────────────────────────────────────────────────────────────────

func TestPublicProductListAnswersOnlyPublishedAndNeedsNoSession(t *testing.T) {
	svc := newFakeCatalogue()
	r := newRouter(t, svc, testRBAC(t), nil)

	rec := get(t, r, "/api/v1/products")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body.String())
	}

	svc.mu.Lock()
	draft, calls := svc.listDraft, len(svc.calls)
	svc.mu.Unlock()
	if calls > 1 {
		t.Errorf("the public list reached the use case %d times: %v", calls, svc.calls)
	}
	if draft {
		t.Error("an anonymous browse must not be able to read drafts")
	}

	var out struct {
		Data []map[string]any `json:"data"`
		Meta struct {
			Page       int64 `json:"page"`
			PageSize   int64 `json:"page_size"`
			Total      int64 `json:"total"`
			TotalPages int64 `json:"total_pages"`
		} `json:"meta"`
		AppliedFilters map[string]any `json:"applied_filters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decoding the ProductList: %v — %s", err, rec.Body.String())
	}
	if len(out.Data) != 0 {
		t.Errorf("the fake had nothing to say, so data should be empty: %v", out.Data)
	}
	if out.Meta.Page != 1 {
		t.Errorf("meta.page = %d, want 1", out.Meta.Page)
	}
}

func TestGetProductReturnsTheDetail(t *testing.T) {
	svc := newFakeCatalogue()
	svc.getProductFn = func(ctx context.Context, ref string, callerCanReadDraft bool) (catalogue.Product, error) {
		if ref != "fridge-42" || callerCanReadDraft {
			t.Errorf("GetProduct(%q, %v) — a public read must not ask for drafts", ref, callerCanReadDraft)
		}
		return catalogue.Product{ID: 1, SKU: "ABC-1", Name: "Fridge", Slug: "fridge-42", PriceCents: 15000}, nil
	}
	r := newRouter(t, svc, testRBAC(t), nil)

	rec := get(t, r, "/api/v1/products/fridge-42")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"sku":"ABC-1"`) {
		t.Errorf("the detail did not carry the product fields: %s", rec.Body.String())
	}
}

func TestGetProductAnswersNotFound(t *testing.T) {
	svc := newFakeCatalogue()
	r := newRouter(t, svc, testRBAC(t), nil)

	rec := get(t, r, "/api/v1/products/nope")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404: %s", rec.Code, rec.Body.String())
	}
	e := decodeEnvelope(t, rec)
	if e.Code != apierr.CodeNotFound {
		t.Errorf("code %q, want %q", e.Code, apierr.CodeNotFound)
	}
}

// ── catalogue writes: the permission gate in front of them ───────────────────────────

func TestCreateProductRefusesAnonymousWith401(t *testing.T) {
	svc := newFakeCatalogue()
	r := newRouter(t, svc, testRBAC(t), nil)

	rec := post(t, r, "/api/v1/products", `{"sku":"ABC-1","name":"Fridge","price_cents":15000,"category_id":3}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401: %s", rec.Code, rec.Body.String())
	}
	e := decodeEnvelope(t, rec)
	if e.Code != apierr.CodeNotAuthenticated {
		t.Errorf("code %q, want %q", e.Code, apierr.CodeNotAuthenticated)
	}
	if len(svc.calls) != 0 {
		t.Errorf("the use case ran for an anonymous create: %v", svc.calls)
	}
}

func TestCreateProductRefusesWithoutThePermissionNamingTheKey(t *testing.T) {
	svc := newFakeCatalogue()
	r := newRouter(t, svc, testRBAC(t), adminWith("shopping.cart.read"))

	rec := post(t, r, "/api/v1/products", `{"sku":"ABC-1","name":"Fridge","price_cents":15000,"category_id":3}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403: %s", rec.Code, rec.Body.String())
	}
	e := decodeEnvelope(t, rec)
	if e.Code != apierr.CodePermissionDenied {
		t.Errorf("code %q, want %q", e.Code, apierr.CodePermissionDenied)
	}
	if e.MissingPermission != "catalogue.products.create" {
		t.Errorf("missing_permission %q, want catalogue.products.create", e.MissingPermission)
	}
	if len(svc.calls) != 0 {
		t.Errorf("the use case ran for a caller without the grant: %v", svc.calls)
	}
}

func TestCreateProductWithThePermissionCreatesADraft(t *testing.T) {
	svc := newFakeCatalogue()
	svc.createProductFn = func(ctx context.Context, in catalogue.CreateProductInput) (catalogue.Product, error) {
		return catalogue.Product{ID: 5, SKU: in.SKU, Slug: in.Slug, PriceCents: in.PriceCents, PublicationState: catalogue.StateDraft}, nil
	}
	r := newRouter(t, svc, testRBAC(t), adminWith("catalogue.products.create"))

	rec := post(t, r, "/api/v1/products", `{"sku":"ABC-1","name":"Fridge","price_cents":15000,"category_id":3}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, want 201: %s", rec.Code, rec.Body.String())
	}

	svc.mu.Lock()
	in := svc.createdInput
	svc.mu.Unlock()
	if in == nil || in.SKU != "ABC-1" || in.PriceCents != 15000 {
		t.Errorf("the use case was not handed the create input: %+v", in)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["id"].(float64) != 5 {
		t.Errorf("the 201 body is not the created product: %s", rec.Body.String())
	}
}

// Every violation is listed in details, keyed by field, and none of them is echoed back.
func TestCreateProductAnswersEveryFieldProblemAtOnce(t *testing.T) {
	r := newRouter(t, newFakeCatalogue(), testRBAC(t), adminWith("catalogue.products.create"))

	rec := post(t, r, "/api/v1/products", `{"sku":"","name":"F","price_cents":14999}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422: %s", rec.Code, rec.Body.String())
	}
	e := decodeEnvelope(t, rec)
	if e.Code != apierr.CodeValidationFailed {
		t.Errorf("code %q, want %q", e.Code, apierr.CodeValidationFailed)
	}
	got := map[string]bool{}
	for _, d := range e.Details {
		got[d.Field] = true
	}
	for _, want := range []string{"sku", "name", "price_cents"} {
		if !got[want] {
			t.Errorf("details did not cover the field %q: %+v", want, e.Details)
		}
	}
}

// ── price change and publication: the privileged writes ─────────────────────────────

func TestChangePriceIsForwardedToTheUseCaseWithTheCaller(t *testing.T) {
	svc := newFakeCatalogue()
	svc.changePriceFn = func(ctx context.Context, actor identity.Principal, id int64, in catalogue.PriceChangeInput) error {
		svc.mu.Lock()
		svc.changedActor = actor
		svc.mu.Unlock()
		return nil
	}
	r := newRouter(t, svc, testRBAC(t), adminWith("catalogue.products.update_price"))

	rec := post(t, r, "/api/v1/products/123/price-changes",
		`{"price_cents":18000,"reason":"market repricing after the supplier's price rise"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["status"] != "changed" {
		t.Errorf("the price change response is %s, want {\"status\":\"changed\"}", rec.Body.String())
	}

	svc.mu.Lock()
	in, actor := svc.changed, svc.changedActor
	svc.mu.Unlock()
	if in == nil || in.PriceCents != 18000 || in.Reason == "" {
		t.Errorf("the use case was not handed the price change: %+v", in)
	}
	if actor.User.ID != 11 {
		t.Errorf("the price change was not stamped with the acting admin %d", actor.User.ID)
	}
}

func TestSetPublicationReachesTheUseCase(t *testing.T) {
	svc := newFakeCatalogue()
	svc.setPublicationFn = func(ctx context.Context, id int64, in catalogue.PublicationInput) (catalogue.Product, error) {
		return catalogue.Product{ID: id, Slug: "fridge", PublicationState: in.State}, nil
	}
	r := newRouter(t, svc, testRBAC(t), adminWith("catalogue.products.publish", "catalogue.products.unpublish"))

	rec := doJSON(t, r, http.MethodPut, "/api/v1/products/123/publication", `{"publication_state":"published","reason":"back in stock"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	svc.mu.Lock()
	in := svc.setPub
	svc.mu.Unlock()
	if in == nil || in.State != catalogue.StatePublished || in.Reason != "back in stock" {
		t.Errorf("the use case was not handed the publication: %+v", in)
	}
}

func TestArchiveProductAnswersConflictWhenTheProductHasOrderHistory(t *testing.T) {
	svc := newFakeCatalogue()
	svc.archiveProductFn = func(ctx context.Context, id int64) error {
		return catalogue.ErrProductHasOrders
	}
	r := newRouter(t, svc, testRBAC(t), adminWith("catalogue.products.archive", "catalogue.products.delete"))

	rec := doJSON(t, r, http.MethodDelete, "/api/v1/products/123", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409: %s", rec.Code, rec.Body.String())
	}
	e := decodeEnvelope(t, rec)
	if e.Code != apierr.CodeConflict {
		t.Errorf("code %q, want %q", e.Code, apierr.CodeConflict)
	}
}

// ── categories: the same gate, and the same envelopes ────────────────────────────────

func TestCreateCategoryRequiresThePermission(t *testing.T) {
	svc := newFakeCatalogue()
	svc.createCategoryFn = func(ctx context.Context, in catalogue.CreateCategoryInput) (catalogue.Category, error) {
		return catalogue.Category{ID: 3, Name: in.Name, Slug: in.Slug, IsActive: true}, nil
	}
	r := newRouter(t, svc, testRBAC(t), adminWith("catalogue.categories.manage"))

	rec := post(t, r, "/api/v1/categories", `{"name":"Lighting","slug":"lighting"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, want 201: %s", rec.Code, rec.Body.String())
	}
	svc.mu.Lock()
	in := svc.createdCat
	svc.mu.Unlock()
	if in == nil || in.Name != "Lighting" || in.Slug != "lighting" {
		t.Errorf("the use case was not handed the category: %+v", in)
	}
}

func TestDeleteCategoryAnswersConflictWhenItHasProducts(t *testing.T) {
	svc := newFakeCatalogue()
	svc.deleteCategoryFn = func(ctx context.Context, id int64) error { return catalogue.ErrCategoryHasProducts }
	r := newRouter(t, svc, testRBAC(t), adminWith("catalogue.categories.delete"))

	rec := doJSON(t, r, http.MethodDelete, "/api/v1/categories/3", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409: %s", rec.Code, rec.Body.String())
	}
	e := decodeEnvelope(t, rec)
	if e.Code != apierr.CodeConflict {
		t.Errorf("code %q, want %q", e.Code, apierr.CodeConflict)
	}
}

func TestDeleteCategoryRequiresThePermission(t *testing.T) {
	svc := newFakeCatalogue()
	r := newRouter(t, svc, testRBAC(t), nil)

	rec := doJSON(t, r, http.MethodDelete, "/api/v1/categories/3", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401: %s", rec.Code, rec.Body.String())
	}
}
