package product_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"tiktok-trend-shop/internal/product"
	"tiktok-trend-shop/internal/store"
)

// openTestDB returns a fresh in-memory equivalent SQLite database with all
// migrations applied.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open("sqlite:///" + filepath.Join(t.TempDir(), "test.sqlite3"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestUpsertProductPreservesLegacyIdentity(t *testing.T) {
	repo := product.NewRepository(openTestDB(t))
	ctx := context.Background()

	legacy, err := repo.UpsertProduct(ctx, product.ProductInput{
		Title:    "Mini Fan",
		Region:   "CN",
		Provider: "manual",
	})
	if err != nil {
		t.Fatalf("upsert legacy: %v", err)
	}
	if legacy.ASIN != nil {
		t.Fatalf("expected nil ASIN for legacy product, got %#v", legacy.ASIN)
	}
	if legacy.Marketplace != nil {
		t.Fatalf("expected nil marketplace for legacy product, got %#v", legacy.Marketplace)
	}
}

func TestUpsertProductDeduplicatesByMarketplaceASIN(t *testing.T) {
	repo := product.NewRepository(openTestDB(t))
	ctx := context.Background()

	marketplace := "US"
	asin := "B0EXAMPLE01"
	first, err := repo.UpsertProduct(ctx, product.ProductInput{
		Title:       "Desk Lamp",
		Region:      "US",
		Marketplace: &marketplace,
		ASIN:        &asin,
		Provider:    "amazon-sp-api",
	})
	if err != nil {
		t.Fatalf("upsert first: %v", err)
	}
	second, err := repo.UpsertProduct(ctx, product.ProductInput{
		Title:       "Desk Lamp (renamed)",
		Region:      "US",
		Marketplace: &marketplace,
		ASIN:        &asin,
		Provider:    "amazon-sp-api",
	})
	if err != nil {
		t.Fatalf("upsert second: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected dedupe to reuse id %q, got %q", first.ID, second.ID)
	}
}

func TestAddSourceSnapshotUpdatesLastCapturedAt(t *testing.T) {
	repo := product.NewRepository(openTestDB(t))
	ctx := context.Background()

	productID, err := repo.UpsertProduct(ctx, product.ProductInput{
		Title:    "Wireless Earbuds",
		Region:   "US",
		Provider: "manual",
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := repo.AddSourceSnapshot(ctx, product.SnapshotInput{
		ProductID:  productID.ID,
		Provider:   "manual",
		SourceType: "product",
		Metrics:    map[string]float64{"views": 1500, "sales": 25},
	}); err != nil {
		t.Fatalf("add snapshot: %v", err)
	}
	got, err := repo.GetProduct(ctx, productID.ID)
	if err != nil {
		t.Fatalf("get product: %v", err)
	}
	if got.LastCapturedAt == nil || *got.LastCapturedAt == "" {
		t.Fatalf("expected last_captured_at to be populated")
	}
}

func TestLatestDetailProjection(t *testing.T) {
	repo := product.NewRepository(openTestDB(t))
	ctx := context.Background()

	productID, err := repo.UpsertProduct(ctx, product.ProductInput{
		Title:    "Phone Stand",
		Region:   "US",
		Provider: "manual",
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	price := 19.99
	image := "https://img.example/stand.jpg"
	url := "https://item.example/stand"
	detailID, err := repo.AddDetailSnapshot(ctx, product.DetailInput{
		ProductID:     productID.ID,
		Provider:      "manual-csv",
		ProductURL:    &url,
		ImageURL:      &image,
		Price:         &price,
		Currency:      stringPtr("USD"),
		SellingPoints: []string{"Adjustable", "Foldable"},
		Specs:         map[string]any{"material": "aluminum"},
		ReviewSummary: stringPtr("Sturdy build"),
	})
	if err != nil {
		t.Fatalf("add detail: %v", err)
	}
	detail, ok, err := repo.LatestDetail(ctx, productID.ID)
	if err != nil {
		t.Fatalf("latest detail: %v", err)
	}
	if !ok || detail.ID != detailID {
		t.Fatalf("expected detail %q got %#v", detailID, detail)
	}
	if detail.Price == nil || *detail.Price != 19.99 {
		t.Fatalf("expected price 19.99 got %#v", detail.Price)
	}
	if detail.Completeness != "complete" {
		t.Fatalf("expected completeness=complete got %q", detail.Completeness)
	}
	if len(detail.SellingPoints) != 2 {
		t.Fatalf("expected 2 selling points got %d", len(detail.SellingPoints))
	}
}

func stringPtr(s string) *string { return &s }