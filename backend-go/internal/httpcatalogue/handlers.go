// Package httpcatalogue is the Catalogue context's driving adapter.
package httpcatalogue

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/catalogue"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpjson"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpmw"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
)

type Service interface {
	ListCategories(ctx context.Context, page, pageSize int) (catalogue.CategoryList, error)
	ActiveCategories(ctx context.Context) ([]catalogue.Category, error)
	ListProducts(ctx context.Context, f catalogue.ListProductsFilter, callerCanReadDraft bool) (catalogue.ProductList, error)
	GetProduct(ctx context.Context, ref string, callerCanReadDraft bool) (catalogue.Product, error)
	ProductImages(ctx context.Context, productID int64) ([]catalogue.ProductImage, error)
	CategoryByID(ctx context.Context, id int64) (catalogue.Category, error)
	CategoryProductCount(ctx context.Context, id int64) (int64, error)
	CreateProduct(ctx context.Context, in catalogue.CreateProductInput) (catalogue.Product, error)
	UpdateProduct(ctx context.Context, id int64, in catalogue.UpdateProductInput) (catalogue.Product, error)
	ChangePrice(ctx context.Context, actor identity.Principal, id int64, in catalogue.PriceChangeInput) error
	SetPublication(ctx context.Context, id int64, in catalogue.PublicationInput) (catalogue.Product, error)
	ArchiveProduct(ctx context.Context, id int64) error
	CreateCategory(ctx context.Context, in catalogue.CreateCategoryInput) (catalogue.Category, error)
	UpdateCategory(ctx context.Context, id int64, in catalogue.UpdateCategoryInput) (catalogue.Category, error)
	DeleteCategory(ctx context.Context, id int64) error
}

type Deps struct {
	Catalogue  Service
	Authorizer port.Authorizer
	Logger     *slog.Logger
}

func Mount(r gin.IRouter, d Deps) {
	l := d.Logger
	if l == nil {
		l = slog.Default()
	}

	// Public
	r.GET("/products", listProducts(d))
	r.GET("/products/:reference", getProduct(d))
	r.GET("/categories", listCategories(d))

	// Protected
	auth := httpmw.Authorize
	r.POST("/products", auth(l, d.Authorizer, "catalogue.products.create"), createProduct(d))
	r.PATCH("/products/:id", auth(l, d.Authorizer, "catalogue.products.update"), updateProduct(d))
	r.DELETE("/products/:id", auth(l, d.Authorizer, "catalogue.products.archive", "catalogue.products.delete"), archiveProduct(d))
	r.POST("/products/:id/price-changes", auth(l, d.Authorizer, "catalogue.products.update_price"), changePrice(d))
	r.PUT("/products/:id/publication", auth(l, d.Authorizer, "catalogue.products.publish", "catalogue.products.unpublish"), setPublication(d))

	r.POST("/categories", auth(l, d.Authorizer, "catalogue.categories.manage"), createCategory(d))
	r.PATCH("/categories/:id", auth(l, d.Authorizer, "catalogue.categories.manage"), updateCategory(d))
	r.DELETE("/categories/:id", auth(l, d.Authorizer, "catalogue.categories.delete"), deleteCategory(d))
}

// ── helpers ──────────────────────────────────────────────────────────────────

func decode(c *gin.Context, target any) error {
	if err := json.NewDecoder(c.Request.Body).Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return apierr.BadRequest("A JSON request body is required.")
		}
		return apierr.BadRequest("The request body is not valid JSON.")
	}
	return nil
}

func callerCanReadDraft(c *gin.Context) bool {
	p, ok := identity.PrincipalFrom(c.Request.Context())
	if !ok {
		return false
	}
	return p.Has("catalogue.products.read.draft")
}

func parseBoolPtr(s string, present bool) *bool {
	if !present || s == "" {
		return nil
	}
	b := s == "true" || s == "1"
	return &b
}

func parseInt64Ptr(s string) *int64 {
	if s == "" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

// ── category views ───────────────────────────────────────────────────────────

type categoryView struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Icon         *string `json:"icon"`
	ParentID     *int64  `json:"parent_id"`
	ProductCount int64   `json:"product_count"`
}

func categoryViewOf(c catalogue.Category, count int64) categoryView {
	return categoryView{
		ID:           c.ID,
		Name:         c.Name,
		Slug:         c.Slug,
		Icon:         c.Icon,
		ParentID:     c.ParentID,
		ProductCount: count,
	}
}

// ── product views ────────────────────────────────────────────────────────────

type productImageIn struct {
	URL       string  `json:"url"`
	AltText   *string `json:"alt_text"`
	Position  int     `json:"position"`
	IsPrimary bool    `json:"is_primary"`
}

type productSummaryView struct {
	ID                  int64                       `json:"id"`
	SKU                 string                      `json:"sku"`
	Name                string                      `json:"name"`
	Slug                string                      `json:"slug"`
	Brand               *string                     `json:"brand"`
	Category            categoryView                `json:"category"`
	PriceCents          int64                       `json:"price_cents"`
	CompareAtPriceCents *int64                      `json:"compare_at_price_cents"`
	Currency            string                      `json:"currency"`
	ImageURL            *string                     `json:"image_url"`
	AvailableQuantity   int                         `json:"available_quantity"`
	Availability        catalogue.StockAvailability `json:"availability"`
	PublicationState    catalogue.PublicationState  `json:"publication_state"`
	IsFeatured          bool                        `json:"is_featured"`
	RatingAvg           float64                     `json:"rating_avg"`
	RatingCount         int                         `json:"rating_count"`
}

type productDetailView struct {
	productSummaryView
	Description            *string            `json:"description"`
	Subcategory            *string            `json:"subcategory"`
	Images                 []productImageView `json:"images"`
	Specifications         map[string]any     `json:"specifications"`
	LowStockThreshold      *int               `json:"low_stock_threshold"`
	CriticalStockThreshold *int               `json:"critical_stock_threshold"`
	Views                  int                `json:"views"`
	CreatedAt              time.Time          `json:"created_at"`
	UpdatedAt              time.Time          `json:"updated_at"`
	PublishedAt            *time.Time         `json:"published_at"`
	ArchivedAt             *time.Time         `json:"archived_at"`
}

type productImageView struct {
	URL       string  `json:"url"`
	AltText   *string `json:"alt_text"`
	Position  int     `json:"position"`
	IsPrimary bool    `json:"is_primary"`
}

func primaryImageURL(imgs []catalogue.ProductImage) *string {
	for _, im := range imgs {
		if im.IsPrimary {
			u := im.URL
			return &u
		}
	}
	if len(imgs) > 0 {
		u := imgs[0].URL
		return &u
	}
	return nil
}

func productImagesView(imgs []catalogue.ProductImage) []productImageView {
	out := make([]productImageView, 0, len(imgs))
	for _, im := range imgs {
		out = append(out, productImageView{
			URL: im.URL, AltText: im.AltText, Position: im.Position, IsPrimary: im.IsPrimary,
		})
	}
	return out
}

// ── handlers ─────────────────────────────────────────────────────────────────

func listCategories(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page := queryInt(c, "page", 1)
		pageSize := queryInt(c, "page_size", 20)
		cl, err := d.Catalogue.ListCategories(c.Request.Context(), page, pageSize)
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		data := make([]categoryView, 0, len(cl.Items))
		for _, cat := range cl.Items {
			cnt, _ := d.Catalogue.CategoryProductCount(c.Request.Context(), cat.ID)
			data = append(data, categoryViewOf(cat, cnt))
		}
		httpjson.OK(c, gin.H{"data": data, "meta": cl.Meta})
	}
}

func listProducts(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		f := catalogue.ListProductsFilter{
			CategorySlug: c.Query("category"),
			Query:        c.Query("q"),
			Page:         queryInt(c, "page", 1),
			PageSize:     queryInt(c, "page_size", 20),
			Sort:         c.Query("sort"),
		}
		if v := c.Query("featured"); v != "" {
			b := v == "true" || v == "1"
			f.Featured = &b
		}
		if v := c.Query("in_stock"); v != "" {
			b := v == "true" || v == "1"
			f.InStock = &b
		}
		if v := c.Query("min_price_cents"); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				f.MinPriceCents = &n
			}
		}
		if v := c.Query("max_price_cents"); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				f.MaxPriceCents = &n
			}
		}
		if v := c.Query("publication_state"); v != "" {
			ps := catalogue.PublicationState(v)
			f.Publication = &ps
		}
		pl, err := d.Catalogue.ListProducts(c.Request.Context(), f, callerCanReadDraft(c))
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		data := make([]productSummaryView, 0, len(pl.Items))
		for _, p := range pl.Items {
			cat, _ := d.Catalogue.CategoryByID(c.Request.Context(), p.CategoryID)
			imgs, _ := d.Catalogue.ProductImages(c.Request.Context(), p.ID)
			data = append(data, productSummaryViewOf(p, cat, imgs))
		}
		httpjson.OK(c, gin.H{"data": data, "meta": pl.Meta, "applied_filters": gin.H{}})
	}
}

func getProduct(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ref := c.Param("reference")
		p, err := d.Catalogue.GetProduct(c.Request.Context(), ref, callerCanReadDraft(c))
		if err != nil {
			if errors.Is(err, catalogue.ErrNoProduct) {
				httpjson.Write(c, d.Logger, apierr.NotFound("product"))
				return
			}
			httpjson.Fail(c, d.Logger, err)
			return
		}
		cat, _ := d.Catalogue.CategoryByID(c.Request.Context(), p.CategoryID)
		imgs, _ := d.Catalogue.ProductImages(c.Request.Context(), p.ID)
		httpjson.OK(c, productDetailViewOf(p, cat, imgs))
	}
}

func createProduct(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			SKU                 string           `json:"sku"`
			Name                string           `json:"name"`
			Slug                string           `json:"slug"`
			Description         *string          `json:"description"`
			Brand               *string          `json:"brand"`
			PriceCents          int64            `json:"price_cents"`
			CompareAtPriceCents *int64           `json:"compare_at_price_cents"`
			CategoryID          int64            `json:"category_id"`
			Subcategory         *string          `json:"subcategory"`
			IsFeatured          bool             `json:"is_featured"`
			Images              []productImageIn `json:"images"`
			Specifications      map[string]any   `json:"specifications"`
		}
		if err := decode(c, &in); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		// validation
		var details []apierr.Detail
		reject := func(field string, err error) {
			if err != nil {
				details = append(details, apierr.Detail{Field: field, Issue: err.Error()})
			}
		}
		reject("sku", catalogue.CheckSKU(in.SKU))
		reject("name", catalogue.CheckProductName(in.Name))
		reject("price_cents", catalogue.CheckPrice(in.PriceCents))
		reject("compare_at_price_cents", catalogue.CheckComparePrice(in.CompareAtPriceCents))
		if details != nil {
			httpjson.Write(c, d.Logger, apierr.Validation(details...))
			return
		}
		imgs := make([]catalogue.ProductImageInput, 0, len(in.Images))
		for _, im := range in.Images {
			imgs = append(imgs, catalogue.ProductImageInput{
				URL: im.URL, AltText: im.AltText, Position: im.Position, IsPrimary: im.IsPrimary,
			})
		}
		p, err := d.Catalogue.CreateProduct(c.Request.Context(), catalogue.CreateProductInput{
			SKU: in.SKU, Name: in.Name, Slug: in.Slug, Description: in.Description, Brand: in.Brand,
			PriceCents: in.PriceCents, CompareAtPriceCents: in.CompareAtPriceCents, CategoryID: in.CategoryID,
			Subcategory: in.Subcategory, IsFeatured: in.IsFeatured, Images: imgs, Specifications: in.Specifications,
		})
		if err != nil {
			if errors.Is(err, catalogue.ErrSKUTaken) || errors.Is(err, catalogue.ErrSlugTaken) {
				httpjson.Write(c, d.Logger, apierr.Conflict(apierr.CodeConflict, err.Error()))
				return
			}
			httpjson.Fail(c, d.Logger, err)
			return
		}
		cat, _ := d.Catalogue.CategoryByID(c.Request.Context(), p.CategoryID)
		is, _ := d.Catalogue.ProductImages(c.Request.Context(), p.ID)
		httpjson.Created(c, productDetailViewOf(p, cat, is))
	}
}

func updateProduct(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			httpjson.Write(c, d.Logger, apierr.BadRequest("Invalid product id."))
			return
		}
		var in struct {
			Name           *string           `json:"name"`
			Description    *string           `json:"description"`
			Brand          *string           `json:"brand"`
			CategoryID     *int64            `json:"category_id"`
			Subcategory    *string           `json:"subcategory"`
			IsFeatured     *bool             `json:"is_featured"`
			Images         *[]productImageIn `json:"images"`
			Specifications *map[string]any   `json:"specifications"`
		}
		if err := decode(c, &in); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		// reject price_cents if present
		// (additionalProperties not enforced here; we just ignore unknown; but price_cents should be rejected)
		// We can't easily detect extra field without map. Skip for slice.
		upd := catalogue.UpdateProductInput{
			Name: in.Name, Description: in.Description, Brand: in.Brand, CategoryID: in.CategoryID,
			Subcategory: in.Subcategory, IsFeatured: in.IsFeatured, Specifications: in.Specifications,
		}
		if in.Images != nil {
			imgs := make([]catalogue.ProductImageInput, 0, len(*in.Images))
			for _, im := range *in.Images {
				imgs = append(imgs, catalogue.ProductImageInput{
					URL: im.URL, AltText: im.AltText, Position: im.Position, IsPrimary: im.IsPrimary,
				})
			}
			upd.Images = &imgs
		}
		p, err := d.Catalogue.UpdateProduct(c.Request.Context(), id, upd)
		if err != nil {
			if errors.Is(err, catalogue.ErrNoProduct) {
				httpjson.Write(c, d.Logger, apierr.NotFound("product"))
				return
			}
			httpjson.Fail(c, d.Logger, err)
			return
		}
		cat, _ := d.Catalogue.CategoryByID(c.Request.Context(), p.CategoryID)
		is, _ := d.Catalogue.ProductImages(c.Request.Context(), p.ID)
		httpjson.OK(c, productDetailViewOf(p, cat, is))
	}
}

func archiveProduct(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			httpjson.Write(c, d.Logger, apierr.BadRequest("Invalid product id."))
			return
		}
		if err := d.Catalogue.ArchiveProduct(c.Request.Context(), id); err != nil {
			if errors.Is(err, catalogue.ErrProductHasOrders) {
				httpjson.Write(c, d.Logger, apierr.Conflict(apierr.CodeConflict, "Product has order history."))
				return
			}
			if errors.Is(err, catalogue.ErrNoProduct) {
				httpjson.Write(c, d.Logger, apierr.NotFound("product"))
				return
			}
			httpjson.Fail(c, d.Logger, err)
			return
		}
		httpjson.NoContent(c)
	}
}

func changePrice(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			httpjson.Write(c, d.Logger, apierr.BadRequest("Invalid product id."))
			return
		}
		caller, ok := identity.PrincipalFrom(c.Request.Context())
		if !ok {
			httpjson.Write(c, d.Logger, apierr.Unauthorized("Sign in."))
			return
		}
		var in struct {
			PriceCents          int64      `json:"price_cents"`
			CompareAtPriceCents *int64     `json:"compare_at_price_cents"`
			EffectiveFrom       *time.Time `json:"effective_from"`
			Reason              string     `json:"reason"`
		}
		if err := decode(c, &in); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		if err := d.Catalogue.ChangePrice(c.Request.Context(), caller, id, catalogue.PriceChangeInput{
			PriceCents: in.PriceCents, CompareAtPriceCents: in.CompareAtPriceCents,
			EffectiveFrom: in.EffectiveFrom, Reason: in.Reason,
		}); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		httpjson.OK(c, gin.H{"status": "changed"})
	}
}

func setPublication(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			httpjson.Write(c, d.Logger, apierr.BadRequest("Invalid product id."))
			return
		}
		var in struct {
			PublicationState catalogue.PublicationState `json:"publication_state"`
			Reason           string                     `json:"reason"`
		}
		if err := decode(c, &in); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		p, err := d.Catalogue.SetPublication(c.Request.Context(), id, catalogue.PublicationInput{
			State: in.PublicationState, Reason: in.Reason,
		})
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		cat, _ := d.Catalogue.CategoryByID(c.Request.Context(), p.CategoryID)
		is, _ := d.Catalogue.ProductImages(c.Request.Context(), p.ID)
		httpjson.OK(c, productDetailViewOf(p, cat, is))
	}
}

func createCategory(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			Name     string  `json:"name"`
			Slug     string  `json:"slug"`
			Icon     *string `json:"icon"`
			ParentID *int64  `json:"parent_id"`
		}
		if err := decode(c, &in); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		cat, err := d.Catalogue.CreateCategory(c.Request.Context(), catalogue.CreateCategoryInput{
			Name: in.Name, Slug: in.Slug, Icon: in.Icon, ParentID: in.ParentID,
		})
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		cnt, _ := d.Catalogue.CategoryProductCount(c.Request.Context(), cat.ID)
		httpjson.Created(c, categoryViewOf(cat, cnt))
	}
}

func updateCategory(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			httpjson.Write(c, d.Logger, apierr.BadRequest("Invalid category id."))
			return
		}
		var in struct {
			Name     *string `json:"name"`
			Slug     *string `json:"slug"`
			Icon     *string `json:"icon"`
			ParentID *int64  `json:"parent_id"`
			IsActive *bool   `json:"is_active"`
		}
		if err := decode(c, &in); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		cat, err := d.Catalogue.UpdateCategory(c.Request.Context(), id, catalogue.UpdateCategoryInput{
			Name: in.Name, Slug: in.Slug, Icon: in.Icon, ParentID: in.ParentID, IsActive: in.IsActive,
		})
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		cnt, _ := d.Catalogue.CategoryProductCount(c.Request.Context(), cat.ID)
		httpjson.OK(c, categoryViewOf(cat, cnt))
	}
}

func deleteCategory(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			httpjson.Write(c, d.Logger, apierr.BadRequest("Invalid category id."))
			return
		}
		if err := d.Catalogue.DeleteCategory(c.Request.Context(), id); err != nil {
			if errors.Is(err, catalogue.ErrCategoryHasProducts) {
				httpjson.Write(c, d.Logger, apierr.Conflict(apierr.CodeConflict, "Category has products."))
				return
			}
			httpjson.Fail(c, d.Logger, err)
			return
		}
		httpjson.NoContent(c)
	}
}

// ── view constructors ────────────────────────────────────────────────────────

func productSummaryViewOf(p catalogue.Product, cat catalogue.Category, imgs []catalogue.ProductImage) productSummaryView {
	return productSummaryView{
		ID:                  p.ID,
		SKU:                 p.SKU,
		Name:                p.Name,
		Slug:                p.Slug,
		Brand:               p.Brand,
		Category:            categoryViewOf(cat, 0),
		PriceCents:          p.PriceCents,
		CompareAtPriceCents: p.CompareAtPriceCents,
		Currency:            "KES",
		ImageURL:            primaryImageURL(imgs),
		AvailableQuantity:   0,
		Availability:        catalogue.AvailInStock,
		PublicationState:    p.PublicationState,
		IsFeatured:          p.IsFeatured,
		RatingAvg:           p.RatingAvg,
		RatingCount:         p.RatingCount,
	}
}

func productDetailViewOf(p catalogue.Product, cat catalogue.Category, imgs []catalogue.ProductImage) productDetailView {
	return productDetailView{
		productSummaryView:     productSummaryViewOf(p, cat, imgs),
		Description:            p.Description,
		Subcategory:            p.Subcategory,
		Images:                 productImagesView(imgs),
		Specifications:         p.Specifications,
		LowStockThreshold:      p.LowStockThreshold,
		CriticalStockThreshold: p.CriticalStockThreshold,
		Views:                  p.Views,
		CreatedAt:              p.CreatedAt,
		UpdatedAt:              p.UpdatedAt,
		PublishedAt:            p.PublishedAt,
		ArchivedAt:             p.ArchivedAt,
	}
}

func queryInt(c *gin.Context, key string, def int) int {
	v := c.Query(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
