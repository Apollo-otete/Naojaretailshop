//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/catalogue"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/dbtest"
)

func TestCatalogueStoreRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	dbtest.Truncate(t, pool, "catalogue.product_images", "catalogue.products", "catalogue.categories")
	ctx := context.Background()

	store, err := New(pool)
	if err != nil {
		t.Fatal(err)
	}

	// category
	cat := catalogue.Category{Name: "Electricals", Slug: "electricals", IsActive: true}
	if err := store.CreateCategory(ctx, &cat); err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	if cat.ID == 0 {
		t.Fatal("expected id")
	}
	got, err := store.CategoryBySlug(ctx, "electricals")
	if err != nil || got.ID != cat.ID {
		t.Fatalf("CategoryBySlug: %v", err)
	}

	// product
	p := catalogue.Product{
		SKU: "SKU-1", Name: "Bulb", Slug: "bulb", PriceCents: 120000,
		CategoryID: cat.ID, PublicationState: catalogue.StateDraft, Specifications: map[string]any{},
	}
	if err := store.CreateProduct(ctx, &p); err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	// image
	img := catalogue.ProductImage{ProductID: p.ID, URL: "https://x/y.jpg", IsPrimary: true, Provider: "local"}
	if err := store.CreateImage(ctx, &img); err != nil {
		t.Fatalf("CreateImage: %v", err)
	}
	imgs, err := store.ImagesByProduct(ctx, p.ID)
	if err != nil || len(imgs) != 1 {
		t.Fatalf("ImagesByProduct: %v len=%d", err, len(imgs))
	}

	// list published should be empty (draft)
	pl, _, err := store.ListProducts(ctx, catalogue.ListProductsFilter{Page: 1, PageSize: 20, Sort: "newest"})
	if err != nil {
		t.Fatal(err)
	}
	_ = pl
	ps := catalogue.StatePublished
	pl2, _, err := store.ListProducts(ctx, catalogue.ListProductsFilter{Page: 1, PageSize: 20, Publication: &ps})
	if err != nil {
		t.Fatal(err)
	}
	if pl2.Meta.Total != 0 {
		t.Fatalf("expected 0 published, got %d", pl2.Meta.Total)
	}

	// publish
	if err := store.SetPublication(ctx, p.ID, catalogue.StatePublished, nil, nil); err != nil {
		t.Fatal(err)
	}
	pl3, _, err := store.ListProducts(ctx, catalogue.ListProductsFilter{Page: 1, PageSize: 20, Publication: &ps})
	if err != nil {
		t.Fatal(err)
	}
	if pl3.Meta.Total != 1 {
		t.Fatalf("expected 1 published, got %d", pl3.Meta.Total)
	}

	// full-text search
	pl4, _, err := store.ListProducts(ctx, catalogue.ListProductsFilter{Page: 1, PageSize: 20, Query: "Bulb"})
	if err != nil {
		t.Fatal(err)
	}
	if pl4.Meta.Total != 1 {
		t.Fatalf("search expected 1, got %d", pl4.Meta.Total)
	}

	// update price
	if err := store.UpdatePrice(ctx, p.ID, 150000, nil); err != nil {
		t.Fatal(err)
	}
	pp, _ := store.ProductByID(ctx, p.ID)
	if pp.PriceCents != 150000 {
		t.Fatalf("price not updated")
	}

	// delete product (no order history)
	if err := store.DeleteProduct(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ProductByID(ctx, p.ID); err != catalogue.ErrNoProduct {
		t.Fatalf("expected ErrNoProduct, got %v", err)
	}

	// delete category
	if err := store.DeleteCategory(ctx, cat.ID); err != nil {
		t.Fatal(err)
	}
}
