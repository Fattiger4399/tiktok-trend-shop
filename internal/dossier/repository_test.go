package dossier_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"tiktok-trend-shop/internal/dossier"
	"tiktok-trend-shop/internal/product"
	"tiktok-trend-shop/internal/store"
)

// openTestDB returns a fresh SQLite database with all migrations applied.
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

func mustProduct(t *testing.T, db *sql.DB, title string) product.Product {
	t.Helper()
	p, err := product.NewRepository(db).UpsertProduct(context.Background(), product.ProductInput{
		Title: title, Region: "US", Provider: "manual",
	})
	if err != nil {
		t.Fatalf("upsert product: %v", err)
	}
	return p
}

func mustAsset(t *testing.T, repo *dossier.Repository, input dossier.AssetInput) dossier.Asset {
	t.Helper()
	asset, err := repo.Add(context.Background(), input)
	if err != nil {
		t.Fatalf("add asset: %v", err)
	}
	return asset
}

func TestAddAndList(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db, "Desk Lamp")
	repo := dossier.NewRepository(db)

	created, err := repo.Add(ctx, dossier.AssetInput{
		ProductID: p.ID,
		Kind:      dossier.KindImage,
		URL:       "https://example.com/lamp.jpg",
		Source:    "amazon.com",
		Note:      "主图",
		CreatedBy: "admin",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if !strings.HasPrefix(created.ID, "da_") {
		t.Fatalf("expected da_ prefixed id got %q", created.ID)
	}
	if created.Kind != dossier.KindImage || created.URL != "https://example.com/lamp.jpg" ||
		created.Source != "amazon.com" || created.Note != "主图" || created.CreatedBy != "admin" {
		t.Fatalf("unexpected asset %#v", created)
	}
	if created.CreatedAt == "" {
		t.Fatalf("expected created_at to be populated")
	}

	items, err := repo.List(ctx, p.ID, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("unexpected list %#v", items)
	}
}

func TestAddValidatesKindAndRequiredFields(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db, "Desk Lamp")
	repo := dossier.NewRepository(db)

	if _, err := repo.Add(ctx, dossier.AssetInput{ProductID: p.ID, Kind: "video"}); err == nil {
		t.Fatalf("expected error for invalid kind")
	}
	if _, err := repo.Add(ctx, dossier.AssetInput{ProductID: p.ID, Kind: dossier.KindImage}); err == nil {
		t.Fatalf("expected error for image without url")
	}
	if _, err := repo.Add(ctx, dossier.AssetInput{ProductID: p.ID, Kind: dossier.KindLink}); err == nil {
		t.Fatalf("expected error for link without url")
	}
	if _, err := repo.Add(ctx, dossier.AssetInput{ProductID: p.ID, Kind: dossier.KindText}); err == nil {
		t.Fatalf("expected error for text without content")
	}
	if _, err := repo.Add(ctx, dossier.AssetInput{Kind: dossier.KindText, Content: "x"}); err == nil {
		t.Fatalf("expected error for empty product_id")
	}
}

func TestListFiltersByKind(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db, "Desk Lamp")
	other := mustProduct(t, db, "Bottle")
	repo := dossier.NewRepository(db)

	mustAsset(t, repo, dossier.AssetInput{ProductID: p.ID, Kind: dossier.KindImage, URL: "https://a/1.jpg"})
	mustAsset(t, repo, dossier.AssetInput{ProductID: p.ID, Kind: dossier.KindText, Content: "好评文案"})
	mustAsset(t, repo, dossier.AssetInput{ProductID: p.ID, Kind: dossier.KindLink, URL: "https://a/item"})
	mustAsset(t, repo, dossier.AssetInput{ProductID: other.ID, Kind: dossier.KindImage, URL: "https://a/2.jpg"})

	all, err := repo.List(ctx, p.ID, "")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 assets got %d", len(all))
	}
	images, err := repo.List(ctx, p.ID, dossier.KindImage)
	if err != nil {
		t.Fatalf("list images: %v", err)
	}
	if len(images) != 1 || images[0].Kind != dossier.KindImage {
		t.Fatalf("unexpected image filter result %#v", images)
	}
	for _, asset := range all {
		if asset.ProductID != p.ID {
			t.Fatalf("unexpected product_id %q", asset.ProductID)
		}
	}
}

func TestDeleteScopedToProduct(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p1 := mustProduct(t, db, "Lamp")
	p2 := mustProduct(t, db, "Bottle")
	repo := dossier.NewRepository(db)

	asset := mustAsset(t, repo, dossier.AssetInput{ProductID: p1.ID, Kind: dossier.KindText, Content: "x"})

	// An asset of another product is a not-found, not a delete.
	if err := repo.Delete(ctx, p2.ID, asset.ID); err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows deleting across products got %v", err)
	}
	if err := repo.Delete(ctx, p1.ID, asset.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	items, err := repo.List(ctx, p1.ID, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected asset removed got %#v", items)
	}
	if err := repo.Delete(ctx, p1.ID, asset.ID); err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows deleting twice got %v", err)
	}
}
