package score_test

import (
	"context"
	"path/filepath"
	"testing"

	"tiktok-trend-shop/internal/product"
	"tiktok-trend-shop/internal/score"
	"tiktok-trend-shop/internal/store"
)

func setup(t *testing.T) (context.Context, *product.Repository, *score.Repository) {
	t.Helper()
	db, err := store.Open("sqlite:///" + filepath.Join(t.TempDir(), "test.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	pr := product.NewRepository(db)
	return context.Background(), pr, score.NewRepository(db, pr)
}

func TestScoreWithCompleteHistory(t *testing.T) {
	ctx, pr, repo := setup(t)
	p, err := pr.UpsertProduct(ctx, product.ProductInput{
		Title:    "Wireless Earbuds",
		Region:   "US",
		Provider: "manual",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []map[string]float64{
		{"views": 1000, "sales": 50, "price": 29.99},
		{"views": 1500, "sales": 80, "price": 27.99, "reviews": 12},
		{"views": 2200, "sales": 130, "price": 27.99, "reviews": 24},
	} {
		if _, err := pr.AddSourceSnapshot(ctx, product.SnapshotInput{
			ProductID:  p.ID,
			Provider:   "manual",
			SourceType: "product",
			Metrics:    m,
		}); err != nil {
			t.Fatal(err)
		}
	}
	snap, err := repo.ComputeAndStore(ctx, p.ID, score.Inputs{TimeWindow: "30d"})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Confidence != "high" && snap.Confidence != "medium" {
		t.Fatalf("expected confidence >= medium, got %q", snap.Confidence)
	}
	if snap.Components["demand"] <= 0 {
		t.Fatalf("expected demand component populated, got %#v", snap.Components)
	}
}

func TestScoreWithSingleSnapshot(t *testing.T) {
	ctx, pr, repo := setup(t)
	p, err := pr.UpsertProduct(ctx, product.ProductInput{
		Title:    "Phone Stand",
		Region:   "US",
		Provider: "manual",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pr.AddSourceSnapshot(ctx, product.SnapshotInput{
		ProductID:  p.ID,
		Provider:   "manual",
		SourceType: "product",
		Metrics:    map[string]float64{"views": 500, "sales": 5},
	}); err != nil {
		t.Fatal(err)
	}
	snap, err := repo.ComputeAndStore(ctx, p.ID, score.Inputs{})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Confidence == "high" {
		t.Fatalf("expected lower confidence for single snapshot, got %q", snap.Confidence)
	}
	if len(snap.MissingComponents) < 2 {
		t.Fatalf("expected several missing components, got %v", snap.MissingComponents)
	}
}

func TestScoreStaleData(t *testing.T) {
	ctx, pr, repo := setup(t)
	p, err := pr.UpsertProduct(ctx, product.ProductInput{
		Title:    "Stale Item",
		Region:   "US",
		Provider: "manual",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pr.AddSourceSnapshot(ctx, product.SnapshotInput{
		ProductID:  p.ID,
		Provider:   "manual",
		SourceType: "product",
		Metrics:    map[string]float64{"views": 200},
	}); err != nil {
		t.Fatal(err)
	}
	// Rewind captured_at to simulate stale data.
	if _, err := pr.DB().ExecContext(ctx,
		`UPDATE source_snapshots SET captured_at = '2000-01-01T00:00:00Z' WHERE product_id = ?`, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pr.DB().ExecContext(ctx,
		`UPDATE products SET last_captured_at = '2000-01-01T00:00:00Z' WHERE id = ?`, p.ID); err != nil {
		t.Fatal(err)
	}
	snap, err := repo.ComputeAndStore(ctx, p.ID, score.Inputs{StaleHours: 1})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Components["freshness"] > 0.1 {
		t.Fatalf("expected near-zero freshness, got %v", snap.Components["freshness"])
	}
}

func TestCrossCategoryNormalization(t *testing.T) {
	ctx, pr, repo := setup(t)
	// Create two products with high and low demand in the same marketplace.
	for _, name := range []string{"High Volume", "Low Volume"} {
		p, err := pr.UpsertProduct(ctx, product.ProductInput{
			Title: name, Region: "US", Provider: "manual",
		})
		if err != nil {
			t.Fatal(err)
		}
		var views float64 = 100
		if name == "High Volume" {
			views = 10000
		}
		if _, err := pr.AddSourceSnapshot(ctx, product.SnapshotInput{
			ProductID: p.ID, Provider: "manual", SourceType: "product",
			Metrics: map[string]float64{"views": views},
		}); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := pr.DB().QueryContext(ctx, `SELECT id, title FROM products ORDER BY title`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type idTitle struct {
		id, title string
	}
	var ids []idTitle
	for rows.Next() {
		var it idTitle
		if err := rows.Scan(&it.id, &it.title); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, it)
	}
	// "High Volume" sorts before "Low Volume"
	highID := ids[0].id
	lowID := ids[1].id
	for _, it := range ids {
		if _, err := repo.ComputeAndStore(ctx, it.id, score.Inputs{Marketplace: "US"}); err != nil {
			t.Fatal(err)
		}
	}
	highSnap, _, _ := repo.LatestForProduct(ctx, highID)
	lowSnap, _, _ := repo.LatestForProduct(ctx, lowID)
	if highSnap.Components["demand"] <= lowSnap.Components["demand"] {
		t.Fatalf("expected high volume demand component greater than low volume, got %v vs %v",
			highSnap.Components["demand"], lowSnap.Components["demand"])
	}
}