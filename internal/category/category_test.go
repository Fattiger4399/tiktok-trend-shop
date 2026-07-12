package category_test

import (
	"context"
	"path/filepath"
	"testing"

	"tiktok-trend-shop/internal/category"
	"tiktok-trend-shop/internal/product"
	"tiktok-trend-shop/internal/store"
)

func openDB(t *testing.T) (context.Context, *category.Repository, *product.Repository) {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open("sqlite:///" + filepath.Join(t.TempDir(), "test.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return ctx, category.NewRepository(db), product.NewRepository(db)
}

func mustProduct(t *testing.T, repo *product.Repository, ctx context.Context, title string) string {
	t.Helper()
	p, err := repo.UpsertProduct(ctx, product.ProductInput{
		Title:    title,
		Region:   "US",
		Provider: "manual",
	})
	if err != nil {
		t.Fatal(err)
	}
	return p.ID
}

func TestSeedCanonicalIsIdempotent(t *testing.T) {
	ctx, repo, _ := openDB(t)
	if err := repo.SeedCanonical(ctx); err != nil {
		t.Fatal(err)
	}
	if err := repo.SeedCanonical(ctx); err != nil {
		t.Fatal(err)
	}
	cats, err := repo.ListCanonical(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) < 12 {
		t.Fatalf("expected at least 12 canonical categories, got %d", len(cats))
	}
}

func TestExactMappingAppliesAndRecordsMethod(t *testing.T) {
	ctx, repo, _ := openDB(t)
	if err := repo.SeedCanonical(ctx); err != nil {
		t.Fatal(err)
	}
	mappingID, err := repo.UpsertMapping(ctx, category.Mapping{
		Provider:            "amazon-sp-api",
		SourceCategoryID:    "281052",
		SourceCategoryName:  stringPtr("Electronics"),
		CanonicalCategoryID: "cat-electronics",
		Confidence:          1.0,
		Method:              "exact",
		Active:              true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if mappingID == "" {
		t.Fatalf("expected mapping id")
	}
	got, ok, err := repo.LookupExactMapping(ctx, "amazon-sp-api", "281052")
	if err != nil || !ok {
		t.Fatalf("lookup: ok=%v err=%v", ok, err)
	}
	if got.CanonicalCategoryID != "cat-electronics" {
		t.Fatalf("expected cat-electronics got %s", got.CanonicalCategoryID)
	}
}

func TestKeywordRuleFallback(t *testing.T) {
	ctx, repo, productRepo := openDB(t)
	if err := repo.SeedCanonical(ctx); err != nil {
		t.Fatal(err)
	}
	productID := mustProduct(t, productRepo, ctx, "Premium Lipstick Set")
	a, err := repo.Classify(ctx, category.ClassifyInput{
		ProductID:          productID,
		Provider:           "amazon-sp-api",
		SourceCategoryID:   "unknown-cat",
		SourceCategoryName: "Health & Beauty",
		Title:              "Premium Lipstick Set",
	}, category.DefaultAutoAssignThreshold)
	if err != nil {
		t.Fatal(err)
	}
	if a.Method != "keyword_rule" {
		t.Fatalf("expected keyword_rule method, got %q", a.Method)
	}
	if a.CanonicalID == nil || *a.CanonicalID != "cat-beauty" {
		t.Fatalf("expected cat-beauty, got %#v", a.CanonicalID)
	}
}

func TestLowConfidenceRoutesToReview(t *testing.T) {
	ctx, repo, productRepo := openDB(t)
	if err := repo.SeedCanonical(ctx); err != nil {
		t.Fatal(err)
	}
	productID := mustProduct(t, productRepo, ctx, "Mystery Item")
	a, err := repo.Classify(ctx, category.ClassifyInput{
		ProductID:          productID,
		Provider:           "manual",
		SourceCategoryID:   "unknown-1",
		SourceCategoryName: "Generic Gadget",
		Title:              "Mystery Item",
	}, category.DefaultAutoAssignThreshold)
	if err != nil {
		t.Fatal(err)
	}
	if !a.NeedsReview {
		t.Fatalf("expected needs_review=true for low confidence")
	}
	unresolved, err := repo.ListUnresolved(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(unresolved) == 0 {
		t.Fatalf("expected unresolved list to include product")
	}
}

func TestManualAssignmentIsLocked(t *testing.T) {
	ctx, repo, productRepo := openDB(t)
	if err := repo.SeedCanonical(ctx); err != nil {
		t.Fatal(err)
	}
	productID := mustProduct(t, productRepo, ctx, "Phone Stand")
	if _, err := repo.AssignManually(ctx, productID, "cat-home", "tester"); err != nil {
		t.Fatal(err)
	}
	_, err := repo.Classify(ctx, category.ClassifyInput{
		ProductID:          productID,
		Provider:           "amazon-sp-api",
		SourceCategoryID:   "281052",
		SourceCategoryName: "Electronics",
		Title:              "Phone Stand",
	}, category.DefaultAutoAssignThreshold)
	if err != category.ErrManualAssignmentLocked {
		t.Fatalf("expected ErrManualAssignmentLocked, got %v", err)
	}
	a, _, _ := repo.GetAssignment(ctx, productID)
	if a.CanonicalID == nil || *a.CanonicalID != "cat-home" {
		t.Fatalf("manual assignment was overwritten")
	}
}

func stringPtr(s string) *string { return &s }