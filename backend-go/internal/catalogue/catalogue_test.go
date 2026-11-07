package catalogue

import (
	"errors"
	"strings"
	"testing"
)

// The catalogue context's rules are pure functions, so they are tested without a store, a
// transaction or a database: the whole of §5's workflow step 2 (draft on create, publish as an
// explicit act, price changes with a reason) lives in here or in New*/Check* above.

func TestCheckSKU(t *testing.T) {
	if err := CheckSKU("ABC-1234"); err != nil {
		t.Errorf("a normal sku was refused: %v", err)
	}
	if err := CheckSKU("  abc123  "); err != nil {
		t.Errorf("surrounding whitespace must not change the verdict: %v", err)
	}
	if err := CheckSKU(""); err == nil {
		t.Error("an empty sku was accepted")
	}
	if err := CheckSKU(strings.Repeat("a", 65)); err == nil {
		t.Error("a 65-character sku was accepted, want the 64-character ceiling")
	}
	if err := CheckSKU(strings.Repeat("a", 64)); err != nil {
		t.Errorf("a 64-character sku was refused: %v", err)
	}
}

func TestCheckProductName(t *testing.T) {
	if err := CheckProductName("Fridge"); err != nil {
		t.Errorf("a one-word product name was refused: %v", err)
	}
	if err := CheckProductName("  Microwave Oven  "); err != nil {
		t.Errorf("surrounding whitespace must not change the verdict: %v", err)
	}
	if err := CheckProductName("A"); err == nil {
		t.Error("a one-character name was accepted")
	}
	if err := CheckProductName("  "); err == nil {
		t.Error("a whitespace-only name was accepted")
	}
	long := strings.Repeat("Fridge ", 60) // 300 characters
	if err := CheckProductName(long); err == nil {
		t.Error("a 300-character name was accepted, want the 200-character ceiling")
	}
}

func TestCheckCategoryName(t *testing.T) {
	if err := CheckCategoryName("Lighting"); err != nil {
		t.Errorf("a normal category name was refused: %v", err)
	}
	if err := CheckCategoryName(""); err == nil {
		t.Error("an empty category name was accepted")
	}
	if err := CheckCategoryName(strings.Repeat("C", 121)); err == nil {
		t.Error("a 121-character category name was accepted, want the 120-character ceiling")
	}
}

func TestCheckSlug(t *testing.T) {
	if err := CheckSlug("fridge-42"); err != nil {
		t.Errorf("a normalised slug was refused: %v", err)
	}
	for _, bad := range []string{"", "Fridge", "fridge 42", "fridge_42", "-leading", "trailing-", "frïidge"} {
		if err := CheckSlug(bad); err == nil {
			t.Errorf("slug %q was accepted", bad)
		}
	}
}

func TestCheckPrice(t *testing.T) {
	if err := CheckPrice(15000); err != nil { // KES 150.00, whole shillings
		t.Errorf("a whole-shilling price was refused: %v", err)
	}
	if err := CheckPrice(14999); err == nil {
		t.Error("a price that is not a whole shilling was accepted")
	}
	if err := CheckPrice(-1); err == nil {
		t.Error("a negative price was accepted")
	}
}

func TestCheckComparePrice(t *testing.T) {
	if err := CheckComparePrice(nil); err != nil {
		t.Errorf("a nil compare-at price must be allowed (no sale): %v", err)
	}
	if err := CheckComparePrice(nil); err != nil {
		t.Errorf("a nil compare-at price was refused")
	}
	cents := int64(30000)
	if err := CheckComparePrice(&cents); err != nil {
		t.Errorf("a whole-shilling compare-at price was refused: %v", err)
	}
	if err := CheckComparePrice(&[]int64{25001}[0]); err == nil {
		t.Error("a fractional compare-at price was accepted")
	}
}

func TestNormaliseSlug(t *testing.T) {
	s, err := NormaliseSlug("Fridge 42 Inch", "")
	if err != nil || s != "fridge-42-inch" {
		t.Errorf("NormaliseSlug(%q) = %q, %v — want the slugified name %q", "Fridge 42 Inch", s, err, "fridge-42-inch")
	}
	s, err = NormaliseSlug("Fridge", "FRIDGE-42")
	if err != nil || s != "fridge-42" {
		t.Errorf("NormaliseSlug with a provided slug = %q, %v — want %q", s, err, "fridge-42")
	}
	if _, err := NormaliseSlug("Fridge", "Not A Slug!"); err == nil {
		t.Error("NormaliseSlug accepted a slug it should have refused")
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "Hello World", want: "hello-world"},
		{in: "  Microwave  Oven!! ", want: "microwave-oven"},
		{in: "Ünicode Fridge", want: "nicode-fridge"}, // non-ASCII folds to a separator
		{in: "", want: "product"},
	}
	for _, tc := range cases {
		if got := slugify(tc.in); got != tc.want {
			t.Errorf("slugify(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNewCategory(t *testing.T) {
	c, err := NewCategory("Lighting", "lighting", nil, nil)
	if err != nil {
		t.Fatalf("a valid category was refused: %v", err)
	}
	if c.Name != "Lighting" || c.Slug != "lighting" || !c.IsActive {
		t.Errorf("NewCategory = %+v", c)
	}
	if c.ParentID != nil {
		t.Errorf("a parentless category must carry a nil ParentID, got %+v", c.ParentID)
	}

	if _, err := NewCategory("", "x", nil, nil); !errors.Is(err, ErrInvalidCategoryName) {
		t.Errorf("NewCategory error = %v, want ErrInvalidCategoryName", err)
	}
}

func TestNewProductDefaultsToDraft(t *testing.T) {
	p, err := NewProduct("ABC-1", "Fridge", "", int64(15000), 3)
	if err != nil {
		t.Fatalf("a valid product was refused: %v", err)
	}
	if p.PublicationState != StateDraft {
		t.Errorf("a new product must start in draft, got %q", p.PublicationState)
	}
	if p.SKU != "ABC-1" || p.Name != "Fridge" || p.Slug != "fridge" || p.PriceCents != 15000 {
		t.Errorf("NewProduct = %+v", p)
	}
	if p.CategoryID != 3 {
		t.Errorf("CategoryID = %d, want 3", p.CategoryID)
	}
}

func TestNewProductRejectsInvalidInput(t *testing.T) {
	if _, err := NewProduct("", "Fridge", "", int64(15000), 3); !errors.Is(err, ErrInvalidSKU) {
		t.Errorf("NewProduct error = %v, want ErrInvalidSKU", err)
	}
	if _, err := NewProduct("ABC-1", "A", "", int64(15000), 3); !errors.Is(err, ErrInvalidName) {
		t.Errorf("NewProduct error = %v, want ErrInvalidName", err)
	}
	if _, err := NewProduct("ABC-1", "Fridge", "", int64(15000), 3); err != nil {
		t.Fatalf("sanity: a valid NewProduct should not fail: %v", err)
	}
	if _, err := NewProduct("ABC-1", "Fridge", "", 14999, 3); !errors.Is(err, ErrInvalidPrice) {
		t.Errorf("NewProduct error = %v, want ErrInvalidPrice", err)
	}
	if _, err := NewProduct("ABC-1", "Fridge", "", int64(15000), 0); err == nil {
		t.Error("NewProduct accepted a missing category")
	}
}

func TestValidateSpecificationsAcceptsAnEmptyMap(t *testing.T) {
	if err := ValidateSpecifications(map[string]any{"voltage": "240"}); err != nil {
		t.Errorf("a valid specifications map was refused: %v", err)
	}
	if err := ValidateSpecifications(nil); err != nil {
		t.Errorf("a nil specifications map (no specs) was refused: %v", err)
	}
}

func TestPublicationStateValidation(t *testing.T) {
	for _, ok := range []PublicationState{StateDraft, StatePublished, StateUnpublished, StateArchived} {
		if !ok.Valid() {
			t.Errorf("state %q must be valid", ok)
		}
	}
	if PublicationState("").Valid() {
		t.Error("an empty publication state must not be valid")
	}
}
