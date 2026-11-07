package catalogue

import (
	"context"
	"time"
)

// ListProductsFilter describes the storefront/admin query.
type ListProductsFilter struct {
	CategorySlug  string
	Query         string // full-text
	Featured      *bool
	InStock       *bool // requires inventory; ignored for now (no inventory table yet) but kept for contract
	MinPriceCents *int64
	MaxPriceCents *int64
	Publication   *PublicationState // nil = published only for public; admin can pass draft etc.
	Page          int
	PageSize      int
	Sort          string
}

// PageMeta is pagination metadata.
type PageMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// ProductList is a page of products.
type ProductList struct {
	Items []Product
	Meta  PageMeta
}

// CategoryList is a page of categories.
type CategoryList struct {
	Items []Category
	Meta  PageMeta
}

// CategoryStore is the port for categories.
type CategoryStore interface {
	CreateCategory(ctx context.Context, c *Category) error
	CategoryByID(ctx context.Context, id int64) (Category, error)
	CategoryBySlug(ctx context.Context, slug string) (Category, error)
	ListCategories(ctx context.Context, page, pageSize int) (CategoryList, error)
	ActiveCategories(ctx context.Context) ([]Category, error)
	UpdateCategory(ctx context.Context, c Category) error
	DeleteCategory(ctx context.Context, id int64) error
	CountProductsByCategory(ctx context.Context, categoryID int64, publishedOnly bool) (int64, error)
}

// ProductStore is the port for products.
type ProductStore interface {
	CreateProduct(ctx context.Context, p *Product) error
	ProductByID(ctx context.Context, id int64) (Product, error)
	ProductBySlug(ctx context.Context, slug string) (Product, error)
	ProductByReference(ctx context.Context, ref string) (Product, error) // slug or numeric id
	ListProducts(ctx context.Context, f ListProductsFilter) (ProductList, []Category, error)
	// returns products + categories map? We'll fetch category separately. Simpler: ListProducts returns products only; service enriches.
	UpdateProduct(ctx context.Context, p Product) error
	UpdatePrice(ctx context.Context, id int64, priceCents int64, compareAt *int64) error
	SetPublication(ctx context.Context, id int64, state PublicationState, publishedAt, archivedAt *time.Time) error
	ArchiveProduct(ctx context.Context, id int64) error
	DeleteProduct(ctx context.Context, id int64) error
	HasOrderHistory(ctx context.Context, productID int64) (bool, error)

	// images
	CreateImage(ctx context.Context, img *ProductImage) error
	ImagesByProduct(ctx context.Context, productID int64) ([]ProductImage, error)
	DeleteImagesByProduct(ctx context.Context, productID int64) error
	SetPrimaryImage(ctx context.Context, productID, imageID int64) error
}

// Transactor for catalogue writes (price change + audit).
type Transactor interface {
	RunInTx(ctx context.Context, work func(ctx context.Context) error) error
}

var _ = time.Now
