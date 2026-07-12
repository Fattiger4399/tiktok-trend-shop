package ingest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"tiktok-trend-shop/internal/product"
	"tiktok-trend-shop/internal/store"
)

func TestImportCSVWithDetailColumns(t *testing.T) {
	db, err := store.Open("sqlite:///" + filepath.Join(t.TempDir(), "test.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	csvPath := filepath.Join(t.TempDir(), "products.csv")
	content := "title,region,category,product_url,platform,shop_name,brand,price,image_url,selling_points,specs,review_summary,views,sales\n" +
		"Desk Lamp,CN,home,https://item.example/lamp,taobao,Demo Shop,Demo Brand,29.9,https://img.example/lamp.jpg,Soft light|USB powered,color:warm|height:38cm,Looks good on desks,10000,200\n"
	if err := os.WriteFile(csvPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := product.NewRepository(db)

	result, err := NewImporter(repo).ImportCSV(context.Background(), csvPath, "CN")
	if err != nil {
		t.Fatal(err)
	}
	products, err := repo.ListProducts(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}

	if result.ImportedRows != 1 {
		t.Fatalf("imported rows = %d", result.ImportedRows)
	}
	if len(result.ProductIDs) != 1 {
		t.Fatalf("product ids = %d", len(result.ProductIDs))
	}
	if len(products) != 1 {
		t.Fatalf("products = %d", len(products))
	}
	got := products[0]
	if got.DetailCompleteness != "complete" {
		t.Fatalf("detail completeness = %s", got.DetailCompleteness)
	}
	if got.ProductURL == nil || *got.ProductURL != "https://item.example/lamp" {
		t.Fatalf("product URL = %#v", got.ProductURL)
	}
	if len(got.SellingPoints) != 2 {
		t.Fatalf("selling points = %#v", got.SellingPoints)
	}
}