package catalogue

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/audit"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
)

// Deps is the catalogue service's world.
type Deps struct {
	Categories CategoryStore
	Products   ProductStore
	Tx         Transactor
	Audit      audit.Writer
	Clock      func() time.Time
	Logger     *slog.Logger
}

// Service is the application layer of the Catalogue context.
type Service struct {
	d Deps
}

func NewService(d Deps) (*Service, error) {
	if d.Categories == nil || d.Products == nil || d.Tx == nil || d.Audit == nil {
		return nil, errors.New("catalogue service is not fully wired")
	}
	if d.Clock == nil {
		d.Clock = time.Now
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Service{d: d}, nil
}

// ── Categories ──────────────────────────────────────────────────────────────────

type CreateCategoryInput struct {
	Name     string
	Slug     string
	Icon     *string
	ParentID *int64
}

type UpdateCategoryInput struct {
	Name     *string
	Slug     *string
	Icon     *string
	ParentID *int64
	IsActive *bool
}

func (s *Service) ListCategories(ctx context.Context, page, pageSize int) (CategoryList, error) {
	page, pageSize = normalizePage(page, pageSize)
	cl, err := s.d.Categories.ListCategories(ctx, page, pageSize)
	if err != nil {
		return CategoryList{}, err
	}
	// enrich product counts
	for i := range cl.Items {
		n, err := s.d.Categories.CountProductsByCategory(ctx, cl.Items[i].ID, true)
		if err != nil {
			return CategoryList{}, err
		}
		// store count? Category has no count field; we'll handle in transport via separate?
		// We'll ignore for now; transport computes. Actually Category schema has product_count.
		// We'll store in a map? Simpler: attach via a parallel slice is awkward.
		// Decision: add ProductCount to a response DTO in transport. For service, leave domain.
		_ = n
	}
	return cl, nil
}

func (s *Service) ActiveCategories(ctx context.Context) ([]Category, error) {
	return s.d.Categories.ActiveCategories(ctx)
}

func (s *Service) CreateCategory(ctx context.Context, in CreateCategoryInput) (Category, error) {
	c, err := NewCategory(in.Name, in.Slug, in.Icon, in.ParentID)
	if err != nil {
		return Category{}, err
	}
	// check slug uniqueness
	if existing, err := s.d.Categories.CategoryBySlug(ctx, c.Slug); err == nil && existing.ID != 0 {
		return Category{}, ErrCategorySlugTaken
	} else if err != nil && !errors.Is(err, ErrNoCategory) {
		return Category{}, err
	}
	if err := s.d.Categories.CreateCategory(ctx, &c); err != nil {
		if isUniqueViolation(err) {
			return Category{}, ErrCategorySlugTaken
		}
		return Category{}, err
	}
	return c, nil
}

func (s *Service) UpdateCategory(ctx context.Context, id int64, in UpdateCategoryInput) (Category, error) {
	c, err := s.d.Categories.CategoryByID(ctx, id)
	if err != nil {
		return Category{}, err
	}
	if in.Name != nil {
		if err := CheckCategoryName(*in.Name); err != nil {
			return Category{}, err
		}
		c.Name = strings.TrimSpace(*in.Name)
	}
	if in.Slug != nil {
		sl, err := NormaliseSlug(c.Name, *in.Slug)
		if err != nil {
			return Category{}, err
		}
		c.Slug = sl
	}
	if in.Icon != nil {
		c.Icon = in.Icon
	}
	if in.ParentID != nil {
		c.ParentID = in.ParentID
	}
	if in.IsActive != nil {
		c.IsActive = *in.IsActive
	}
	if err := s.d.Categories.UpdateCategory(ctx, c); err != nil {
		if isUniqueViolation(err) {
			return Category{}, ErrCategorySlugTaken
		}
		return Category{}, err
	}
	return c, nil
}

func (s *Service) DeleteCategory(ctx context.Context, id int64) error {
	n, err := s.d.Categories.CountProductsByCategory(ctx, id, false)
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrCategoryHasProducts
	}
	return s.d.Categories.DeleteCategory(ctx, id)
}

// ── Products ────────────────────────────────────────────────────────────────────

type ProductImageInput struct {
	URL       string
	AltText   *string
	Position  int
	IsPrimary bool
}

type CreateProductInput struct {
	SKU                 string
	Name                string
	Slug                string
	Description         *string
	Brand               *string
	PriceCents          int64
	CompareAtPriceCents *int64
	CategoryID          int64
	Subcategory         *string
	IsFeatured          bool
	Images              []ProductImageInput
	Specifications      map[string]any
}

type UpdateProductInput struct {
	Name           *string
	Description    *string
	Brand          *string
	CategoryID     *int64
	Subcategory    *string
	IsFeatured     *bool
	Images         *[]ProductImageInput
	Specifications *map[string]any
}

type PriceChangeInput struct {
	PriceCents          int64
	CompareAtPriceCents *int64
	EffectiveFrom       *time.Time
	Reason              string
}

type PublicationInput struct {
	State  PublicationState
	Reason string
}

func (s *Service) ListProducts(ctx context.Context, f ListProductsFilter, callerCanReadDraft bool) (ProductList, error) {
	// Force published for callers without draft read.
	if !callerCanReadDraft {
		ps := StatePublished
		f.Publication = &ps
	} else if f.Publication == nil {
		// default published for unspecified
		ps := StatePublished
		f.Publication = &ps
	}
	page, pageSize := normalizePage(f.Page, f.PageSize)
	f.Page = page
	f.PageSize = pageSize
	if f.Sort == "" {
		f.Sort = "newest"
	}
	pl, cats, err := s.d.Products.ListProducts(ctx, f)
	if err != nil {
		return ProductList{}, err
	}
	_ = cats
	return pl, nil
}

// helper methods for transport enrichment
func (s *Service) ProductImages(ctx context.Context, productID int64) ([]ProductImage, error) {
	return s.d.Products.ImagesByProduct(ctx, productID)
}

func (s *Service) CategoryByID(ctx context.Context, id int64) (Category, error) {
	return s.d.Categories.CategoryByID(ctx, id)
}

func (s *Service) GetProduct(ctx context.Context, ref string, callerCanReadDraft bool) (Product, error) {
	p, err := s.d.Products.ProductByReference(ctx, ref)
	if err != nil {
		return Product{}, err
	}
	// visibility: published only unless caller can read draft
	if p.PublicationState != StatePublished {
		if !callerCanReadDraft {
			return Product{}, ErrNoProduct
		}
	}
	return p, nil
}

func (s *Service) CreateProduct(ctx context.Context, in CreateProductInput) (Product, error) {
	if err := CheckSKU(in.SKU); err != nil {
		return Product{}, err
	}
	if err := CheckProductName(in.Name); err != nil {
		return Product{}, err
	}
	slug, err := NormaliseSlug(in.Name, in.Slug)
	if err != nil {
		return Product{}, err
	}
	if err := CheckPrice(in.PriceCents); err != nil {
		return Product{}, err
	}
	if err := CheckComparePrice(in.CompareAtPriceCents); err != nil {
		return Product{}, err
	}
	if in.Description != nil && len(*in.Description) > 20000 {
		return Product{}, ErrInvalidDescription
	}
	if err := ValidateSpecifications(in.Specifications); err != nil {
		return Product{}, err
	}
	if in.CategoryID <= 0 {
		return Product{}, errors.New("category_id is required")
	}
	// ensure category exists
	if _, err := s.d.Categories.CategoryByID(ctx, in.CategoryID); err != nil {
		return Product{}, err
	}
	// uniqueness checks
	if _, err := s.d.Products.ProductBySlug(ctx, slug); err == nil {
		return Product{}, ErrSlugTaken
	} else if !errors.Is(err, ErrNoProduct) {
		return Product{}, err
	}
	// sku uniqueness: try create and catch unique violation

	p := Product{
		SKU:                 strings.TrimSpace(in.SKU),
		Name:                strings.TrimSpace(in.Name),
		Slug:                slug,
		Description:         in.Description,
		Brand:               in.Brand,
		PriceCents:          in.PriceCents,
		CompareAtPriceCents: in.CompareAtPriceCents,
		CategoryID:          in.CategoryID,
		Subcategory:         in.Subcategory,
		PublicationState:    StateDraft,
		IsFeatured:          in.IsFeatured,
		Specifications:      in.Specifications,
	}
	if p.Specifications == nil {
		p.Specifications = map[string]any{}
	}
	if err := s.d.Products.CreateProduct(ctx, &p); err != nil {
		if isUniqueViolation(err) {
			if strings.Contains(strings.ToLower(err.Error()), "slug") {
				return Product{}, ErrSlugTaken
			}
			return Product{}, ErrSKUTaken
		}
		return Product{}, err
	}
	// images
	for i, img := range in.Images {
		if strings.TrimSpace(img.URL) == "" {
			return Product{}, ErrInvalidImageURL
		}
		position := img.Position
		if position == 0 && i > 0 {
			position = i
		}
		pi := ProductImage{
			ProductID: p.ID,
			URL:       strings.TrimSpace(img.URL),
			AltText:   img.AltText,
			Position:  position,
			Provider:  "local",
			IsPrimary: img.IsPrimary,
		}
		if err := s.d.Products.CreateImage(ctx, &pi); err != nil {
			return Product{}, err
		}
	}
	// if no primary set, make first primary
	imgs, _ := s.d.Products.ImagesByProduct(ctx, p.ID)
	if len(imgs) > 0 {
		hasPrimary := false
		for _, im := range imgs {
			if im.IsPrimary {
				hasPrimary = true
				break
			}
		}
		if !hasPrimary {
			_ = s.d.Products.SetPrimaryImage(ctx, p.ID, imgs[0].ID)
		}
	}
	return p, nil
}

func (s *Service) UpdateProduct(ctx context.Context, id int64, in UpdateProductInput) (Product, error) {
	p, err := s.d.Products.ProductByID(ctx, id)
	if err != nil {
		return Product{}, err
	}
	if in.Name != nil {
		if err := CheckProductName(*in.Name); err != nil {
			return Product{}, err
		}
		p.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		if len(*in.Description) > 20000 {
			return Product{}, ErrInvalidDescription
		}
		p.Description = in.Description
	}
	if in.Brand != nil {
		p.Brand = in.Brand
	}
	if in.CategoryID != nil {
		if *in.CategoryID <= 0 {
			return Product{}, errors.New("category_id is required")
		}
		if _, err := s.d.Categories.CategoryByID(ctx, *in.CategoryID); err != nil {
			return Product{}, err
		}
		p.CategoryID = *in.CategoryID
	}
	if in.Subcategory != nil {
		p.Subcategory = in.Subcategory
	}
	if in.IsFeatured != nil {
		p.IsFeatured = *in.IsFeatured
	}
	if in.Specifications != nil {
		if err := ValidateSpecifications(*in.Specifications); err != nil {
			return Product{}, err
		}
		p.Specifications = *in.Specifications
		if p.Specifications == nil {
			p.Specifications = map[string]any{}
		}
	}
	if err := s.d.Products.UpdateProduct(ctx, p); err != nil {
		return Product{}, err
	}
	// images replacement if provided
	if in.Images != nil {
		if err := s.d.Products.DeleteImagesByProduct(ctx, p.ID); err != nil {
			return Product{}, err
		}
		for i, img := range *in.Images {
			if strings.TrimSpace(img.URL) == "" {
				return Product{}, ErrInvalidImageURL
			}
			position := img.Position
			if position == 0 && i > 0 {
				position = i
			}
			pi := ProductImage{
				ProductID: p.ID,
				URL:       strings.TrimSpace(img.URL),
				AltText:   img.AltText,
				Position:  position,
				Provider:  "local",
				IsPrimary: img.IsPrimary,
			}
			if err := s.d.Products.CreateImage(ctx, &pi); err != nil {
				return Product{}, err
			}
		}
	}
	return p, nil
}

func (s *Service) ChangePrice(ctx context.Context, actor identity.Principal, id int64, in PriceChangeInput) error {
	if err := CheckPrice(in.PriceCents); err != nil {
		return err
	}
	if err := CheckComparePrice(in.CompareAtPriceCents); err != nil {
		return err
	}
	if len(strings.TrimSpace(in.Reason)) < 8 {
		return ErrPriceChangeReason
	}
	p, err := s.d.Products.ProductByID(ctx, id)
	if err != nil {
		return err
	}
	prev := p.PriceCents
	prevCompare := p.CompareAtPriceCents

	return s.d.Tx.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.d.Products.UpdatePrice(ctx, id, in.PriceCents, in.CompareAtPriceCents); err != nil {
			return err
		}
		entry := audit.ByUser(actor.UserID(), "catalogue.product.price_change", "product", &id)
		entry.PreviousValue = map[string]any{"price_cents": prev, "compare_at_price_cents": prevCompare}
		entry.NewValue = map[string]any{"price_cents": in.PriceCents, "compare_at_price_cents": in.CompareAtPriceCents}
		entry.Reason = strings.TrimSpace(in.Reason)
		entry.IP = "" // transport sets? We'll leave; audit writer uses ctx request id
		return s.d.Audit.Record(ctx, entry)
	})
}

func (s *Service) SetPublication(ctx context.Context, id int64, in PublicationInput) (Product, error) {
	if !in.State.Valid() {
		return Product{}, ErrInvalidPublication
	}
	p, err := s.d.Products.ProductByID(ctx, id)
	if err != nil {
		return Product{}, err
	}
	if in.State == StateArchived && p.PublicationState == StateArchived {
		return p, nil
	}
	if in.State == StatePublished {
		// reason not required when publishing
	} else {
		if strings.TrimSpace(in.Reason) == "" && (in.State == StateUnpublished || in.State == StateArchived) {
			// contract says reason required when leaving published; enforce
			if p.PublicationState == StatePublished {
				return Product{}, errors.New("reason is required when leaving published")
			}
		}
	}
	now := s.d.Clock()
	var publishedAt, archivedAt *time.Time
	switch in.State {
	case StatePublished:
		t := now
		publishedAt = &t
	case StateArchived:
		t := now
		archivedAt = &t
	}
	if err := s.d.Products.SetPublication(ctx, id, in.State, publishedAt, archivedAt); err != nil {
		return Product{}, err
	}
	p.PublicationState = in.State
	if publishedAt != nil {
		p.PublishedAt = publishedAt
	}
	if archivedAt != nil {
		p.ArchivedAt = archivedAt
	}
	return p, nil
}

func (s *Service) ArchiveProduct(ctx context.Context, id int64) error {
	has, err := s.d.Products.HasOrderHistory(ctx, id)
	if err != nil {
		return err
	}
	if has {
		return ErrProductHasOrders
	}
	return s.d.Products.DeleteProduct(ctx, id)
}

// Category count enrichment helper for transport
func (s *Service) CategoryProductCount(ctx context.Context, categoryID int64) (int64, error) {
	return s.d.Categories.CountProductsByCategory(ctx, categoryID, true)
}

// ── helpers ─────────────────────────────────────────────────────────────────────

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func isUniqueViolation(err error) bool {
	// pgx error code 23505
	type pgErr interface{ SQLState() string }
	if e, ok := err.(pgErr); ok {
		return e.SQLState() == "23505"
	}
	// fallback string check
	return strings.Contains(err.Error(), "23505") || strings.Contains(strings.ToLower(err.Error()), "duplicate key")
}

// sortProducts sorts in memory for the slice; real impl would push to SQL. Acceptable for now.
func sortProducts(items []Product, sortKey string) {
	switch sortKey {
	case "price_asc":
		sort.Slice(items, func(i, j int) bool { return items[i].PriceCents < items[j].PriceCents })
	case "price_desc":
		sort.Slice(items, func(i, j int) bool { return items[i].PriceCents > items[j].PriceCents })
	case "name_asc":
		sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	default: // newest
		sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	}
}
