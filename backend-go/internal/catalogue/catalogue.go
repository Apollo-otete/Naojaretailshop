// Package catalogue is the Catalogue bounded context (§7): products, categories, images and
// publication state.
//
// §8 keeps business rules here rather than in Gin, so this package imports no HTTP types.
package catalogue

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// PublicationState governs storefront visibility independently of stock status (§5 workflow
// step 4). A product's stock truth is a separate concern (inventory context).
type PublicationState string

const (
	StateDraft       PublicationState = "draft"
	StatePublished   PublicationState = "published"
	StateUnpublished PublicationState = "unpublished"
	StateArchived    PublicationState = "archived"
)

func (s PublicationState) Valid() bool {
	switch s {
	case StateDraft, StatePublished, StateUnpublished, StateArchived:
		return true
	}
	return false
}

// StockAvailability is a derived response label (§9), never stored.
type StockAvailability string

const (
	AvailInStock    StockAvailability = "in_stock"
	AvailLowStock   StockAvailability = "low_stock"
	AvailOutOfStock StockAvailability = "out_of_stock"
)

// Money is integer cents. Whole-shilling prices carry % 100 = 0 (ADR-021).
type Money int64

// Category is a node in the (optional, single-level-today) category tree.
type Category struct {
	ID        int64
	Name      string
	Slug      string
	Icon      *string
	ParentID  *int64
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ProductImage is one image attached to a product.
type ProductImage struct {
	ID          int64
	ProductID   int64
	URL         string
	AltText     *string
	Position    int
	Provider    string
	ProviderRef *string
	IsPrimary   bool
	CreatedAt   time.Time
}

// Product is the catalogue's central aggregate.
type Product struct {
	ID                     int64
	SKU                    string
	Name                   string
	Slug                   string
	Description            *string
	Brand                  *string
	PriceCents             int64
	CompareAtPriceCents    *int64
	CategoryID             int64
	Subcategory            *string
	PublicationState       PublicationState
	IsFeatured             bool
	Specifications         map[string]any
	Views                  int
	RatingAvg              float64
	RatingCount            int
	LowStockThreshold      *int
	CriticalStockThreshold *int
	CreatedAt              time.Time
	UpdatedAt              time.Time
	PublishedAt            *time.Time
	ArchivedAt             *time.Time
}

// ProductDetail is the full read shape (ProductSummary + detail fields). Images and category
// are attached at the application/transport layer.
type ProductDetail struct {
	Product
	Images   []ProductImage
	Category Category
}

// Category errors and validation.
var (
	ErrNoCategory = errors.New("no such category")
	ErrNoProduct  = errors.New("no such product")

	ErrInvalidSKU          = errors.New("sku must be between 1 and 64 characters")
	ErrInvalidName         = errors.New("name must be between 2 and 200 characters")
	ErrInvalidSlug         = errors.New("slug must be lowercase letters, digits and hyphens")
	ErrInvalidPrice        = errors.New("price must be a non-negative whole-shilling amount")
	ErrInvalidComparePrice = errors.New("compare-at price must be a non-negative whole-shilling amount")
	ErrInvalidDescription  = errors.New("description is too long")
	ErrInvalidCategoryName = errors.New("category name must be between 1 and 120 characters")
	ErrSKUTaken            = errors.New("a product already uses this sku")
	ErrSlugTaken           = errors.New("a product already uses this slug")
	ErrCategorySlugTaken   = errors.New("a category already uses this slug")
	ErrCategoryHasProducts = errors.New("category still has products")
	ErrProductHasOrders    = errors.New("product has order history")
	ErrPriceChangeReason   = errors.New("a price change requires a reason of at least 8 characters")
	ErrInvalidPublication  = errors.New("unknown publication state")
	ErrInvalidImageURL     = errors.New("image url is required")
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// CheckSKU validates the SKU length.
func CheckSKU(sku string) error {
	n := utf8.RuneCountInString(strings.TrimSpace(sku))
	if n < 1 || n > 64 {
		return ErrInvalidSKU
	}
	return nil
}

// CheckProductName validates product name length.
func CheckProductName(name string) error {
	n := utf8.RuneCountInString(strings.TrimSpace(name))
	if n < 2 || n > 200 {
		return ErrInvalidName
	}
	return nil
}

// CheckCategoryName validates category name length.
func CheckCategoryName(name string) error {
	n := utf8.RuneCountInString(strings.TrimSpace(name))
	if n < 1 || n > 120 {
		return ErrInvalidCategoryName
	}
	return nil
}

// CheckSlug validates slug format.
func CheckSlug(slug string) error {
	if !slugPattern.MatchString(slug) {
		return ErrInvalidSlug
	}
	return nil
}

// CheckPrice validates price cents (non-negative, whole shillings).
func CheckPrice(cents int64) error {
	if cents < 0 || cents%100 != 0 {
		return ErrInvalidPrice
	}
	return nil
}

// CheckComparePrice validates compare-at price.
func CheckComparePrice(cents *int64) error {
	if cents == nil {
		return nil
	}
	if *cents < 0 || *cents%100 != 0 {
		return ErrInvalidComparePrice
	}
	return nil
}

// NormaliseSlug generates a slug from a name if empty, else validates.
func NormaliseSlug(name, provided string) (string, error) {
	s := strings.TrimSpace(provided)
	if s == "" {
		s = slugify(name)
	}
	s = strings.ToLower(s)
	if err := CheckSlug(s); err != nil {
		return "", err
	}
	return s, nil
}

func slugify(name string) string {
	// Simple slugify: lowercase, replace non-alphanumeric with hyphens, collapse.
	lower := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	prevHyphen := false
	for _, r := range lower {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevHyphen = false
		default:
			if !prevHyphen && b.Len() > 0 {
				b.WriteRune('-')
				prevHyphen = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "product"
	}
	return out
}

// ValidateSpecifications ensures it's a valid JSON object (map). Returns nil if nil.
func ValidateSpecifications(specs map[string]any) error {
	if specs == nil {
		return nil
	}
	// ensure serialisable
	if _, err := json.Marshal(specs); err != nil {
		return fmt.Errorf("invalid specifications: %w", err)
	}
	return nil
}

// NewCategory builds a validated Category.
func NewCategory(name, slug string, icon *string, parentID *int64) (Category, error) {
	if err := CheckCategoryName(name); err != nil {
		return Category{}, err
	}
	s, err := NormaliseSlug(name, slug)
	if err != nil {
		return Category{}, err
	}
	return Category{
		Name:     strings.TrimSpace(name),
		Slug:     s,
		Icon:     icon,
		ParentID: parentID,
		IsActive: true,
	}, nil
}

// NewProduct builds a validated Product in draft state.
func NewProduct(sku, name, slug string, priceCents int64, categoryID int64) (Product, error) {
	if err := CheckSKU(sku); err != nil {
		return Product{}, err
	}
	if err := CheckProductName(name); err != nil {
		return Product{}, err
	}
	s, err := NormaliseSlug(name, slug)
	if err != nil {
		return Product{}, err
	}
	if err := CheckPrice(priceCents); err != nil {
		return Product{}, err
	}
	if categoryID <= 0 {
		return Product{}, errors.New("category_id is required")
	}
	return Product{
		SKU:              strings.TrimSpace(sku),
		Name:             strings.TrimSpace(name),
		Slug:             s,
		PriceCents:       priceCents,
		CategoryID:       categoryID,
		PublicationState: StateDraft,
		Specifications:   map[string]any{},
	}, nil
}
