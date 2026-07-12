package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"tiktok-trend-shop/internal/product"
	"tiktok-trend-shop/internal/store"
)

func TestHealthAndProductsEndpoints(t *testing.T) {
	db, err := store.Open("sqlite:///" + filepath.Join(t.TempDir(), "test.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	repo := product.NewRepository(db)
	created, err := repo.UpsertProduct(context.Background(), product.ProductInput{
		Title:    "Mini Fan",
		Region:   "CN",
		Provider: "manual",
		Metadata: map[string]any{"test": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddSourceSnapshot(context.Background(), product.SnapshotInput{
		ProductID:  created.ID,
		Provider:   "manual",
		SourceType: "product",
		Metrics:    map[string]float64{"views": 1200},
	}); err != nil {
		t.Fatal(err)
	}
	handler := New(repo)

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health code = %d", health.Code)
	}

	products := httptest.NewRecorder()
	handler.ServeHTTP(products, httptest.NewRequest(http.MethodGet, "/products?limit=1", nil))
	if products.Code != http.StatusOK {
		t.Fatalf("products code = %d body=%s", products.Code, products.Body.String())
	}
	var payload struct {
		Count   int `json:"count"`
		Results []struct {
			Title   string             `json:"title"`
			Metrics map[string]float64 `json:"metrics"`
		} `json:"results"`
	}
	if err := json.Unmarshal(products.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Count != 1 || payload.Results[0].Title != "Mini Fan" {
		t.Fatalf("payload = %#v", payload)
	}
	if payload.Results[0].Metrics["views"] != 1200 {
		t.Fatalf("metrics = %#v", payload.Results[0].Metrics)
	}
}
