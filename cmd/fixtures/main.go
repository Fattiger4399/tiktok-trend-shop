// Command fixtures seeds the development database with representative
// multi-snapshot products for browser verification of the workbench.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"tiktok-trend-shop/internal/product"
	"tiktok-trend-shop/internal/score"
	"tiktok-trend-shop/internal/store"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	db, err := store.Open(envDefault("TTS_DATABASE_URL", "sqlite:///./data/web-fixture.sqlite3"))
	if err != nil {
		return err
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		return err
	}
	pr := product.NewRepository(db)
	sc := score.NewRepository(db, pr)
	ctx := context.Background()

	type seed struct {
		title       string
		marketplace string
		asin        string
		category    string
		views       []float64
		sales       []float64
		price       []float64
		reviews     []float64
	}
	seeds := []seed{
		{
			title: "LED Desk Lamp", marketplace: "US", asin: "B0FIX0001", category: "Home & Kitchen",
			views: []float64{900, 1500, 2400, 3600, 5800, 9000, 12000},
			sales: []float64{8, 14, 22, 38, 70, 130, 210},
			price: []float64{34.99, 32.99, 31.99, 30.99, 29.99, 29.99, 29.99},
			reviews: []float64{0, 1, 5, 12, 25, 41, 60},
		},
		{
			title: "Wireless Earbuds", marketplace: "US", asin: "B0FIX0002", category: "Electronics",
			views: []float64{300, 700, 1500, 2800, 5000, 6500, 8500},
			sales: []float64{3, 7, 12, 28, 55, 80, 95},
			price: []float64{59.99, 57.99, 55.99, 54.99, 52.99, 51.99, 49.99},
			reviews: []float64{0, 2, 4, 9, 22, 38, 47},
		},
	}
	for _, s := range seeds {
		marketplace := s.marketplace
		asin := s.asin
		p, err := pr.UpsertProduct(ctx, product.ProductInput{
			Title: s.title, Region: "US", Marketplace: &marketplace, ASIN: &asin,
			SourceCategory: stringPtr(s.category), Provider: "amazon-sp-api",
		})
		if err != nil {
			return fmt.Errorf("upsert %s: %w", s.title, err)
		}
		base := time.Now().Add(-time.Duration(len(s.views)) * 24 * time.Hour)
		for idx := range s.views {
			captured := base.Add(time.Duration(idx) * 24 * time.Hour).UTC().Format(time.RFC3339)
			if _, err := db.ExecContext(ctx, `
				INSERT INTO source_snapshots(
					id, product_id, provider, source_type, source_id, source_url,
					captured_at, metrics_json, metric_kind, raw_ref, metadata_json, created_at
				)
				VALUES (?, ?, 'amazon-sp-api', 'product', ?, NULL, ?, ?, 'observed', NULL, '{}', ?)
			`, idAt(ctx, db, "snap"), p.ID, s.asin, captured,
				metricsJSON(s.views[idx], s.sales[idx], s.price[idx], s.reviews[idx]), captured); err != nil {
				return err
			}
		}
	}
	count, err := sc.ComputeAll(ctx, score.Inputs{TimeWindow: "90d"})
	if err != nil {
		return err
	}
	fmt.Printf("seeded %d hotspot snapshots\n", count)
	return nil
}

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func stringPtr(s string) *string { return &s }

func metricsJSON(views, sales, price, reviews float64) string {
	body := map[string]float64{
		"views":   views,
		"sales":   sales,
		"price":   price,
		"reviews": reviews,
	}
	b, _ := jsonMarshal(body)
	return string(b)
}

// jsonMarshal is a tiny helper to avoid pulling encoding/json into the
// fixture program just for one map. The fixture runs locally so we keep the
// dependency footprint small.
func jsonMarshal(v map[string]float64) ([]byte, error) {
	out := []byte{'{'}
	first := true
	for k, val := range v {
		if !first {
			out = append(out, ',')
		}
		first = false
		out = append(out, '"')
		out = append(out, k...)
		out = append(out, '"', ':')
		out = append(out, []byte(fmt.Sprintf("%g", val))...)
	}
	out = append(out, '}')
	return out, nil
}

func idAt(ctx context.Context, db *sql.DB, prefix string) string {
	// We rely on the repository's id package via the public API path, but
	// the fixture is a tiny program so we inline a minimal id generator.
	var b [16]byte
	for i := range b {
		b[i] = byte(time.Now().UnixNano() >> (i % 8))
	}
	const hex = "0123456789abcdef"
	out := []byte(prefix)
	out = append(out, '_')
	for _, v := range b {
		out = append(out, hex[v>>4])
		out = append(out, hex[v&0x0f])
	}
	return string(out)
}