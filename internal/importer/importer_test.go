package importer_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tiktok-trend-shop/internal/importer"
	"tiktok-trend-shop/internal/product"
	"tiktok-trend-shop/internal/store"
)

func openDB(t *testing.T) (context.Context, *importer.Importer, *product.Repository) {
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
	return context.Background(), importer.New(db, pr), pr
}

const validCSV = "title,region,category,product_url,platform,shop_name,brand,price,image_url,selling_points,specs,review_summary,views,sales,marketplace,asin\n" +
	"Desk Lamp,CN,home,https://item.example/lamp,taobao,Demo Shop,Demo Brand,29.9,https://img.example/lamp.jpg,Soft light|USB powered,color:warm|height:38cm,Looks good on desks,10000,200,CN,B0EXAMPLE1\n"

func TestImportValidCSV(t *testing.T) {
	ctx, imp, _ := openDB(t)
	result, err := imp.ImportFromPath(ctx, writeTempCSV(t, validCSV), "CN", "manual-csv")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" {
		t.Fatalf("expected completed, got %s", result.Status)
	}
	if result.ImportedRows != 1 {
		t.Fatalf("expected imported rows 1, got %d", result.ImportedRows)
	}
}

func TestImportInvalidHeaders(t *testing.T) {
	ctx, imp, _ := openDB(t)
	body := "region,category\nCN,home\n"
	_, err := imp.ImportFromPath(ctx, writeTempCSV(t, body), "CN", "manual-csv")
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestImportMixedRows(t *testing.T) {
	ctx, imp, _ := openDB(t)
	body := "title,region,views,sales\n" +
		"Good Item,CN,1000,50\n" +
		",US,500,10\n" + // missing title
		"Another,US,800,20\n"
	result, err := imp.ImportFromPath(ctx, writeTempCSV(t, body), "CN", "manual-csv")
	if err != nil {
		t.Fatal(err)
	}
	if result.ImportedRows != 2 {
		t.Fatalf("expected 2 imported, got %d", result.ImportedRows)
	}
	if result.RejectedRows != 1 {
		t.Fatalf("expected 1 rejected, got %d", result.RejectedRows)
	}
}

func TestImportDeduplicatesByMarketplaceASIN(t *testing.T) {
	ctx, imp, _ := openDB(t)
	body := "title,region,marketplace,asin,views\n" +
		"First Title,US,US,B0DUPE1,100\n" +
		"Renamed Title,US,US,B0DUPE1,500\n"
	first, err := imp.ImportFromPath(ctx, writeTempCSV(t, body), "US", "manual-csv")
	if err != nil {
		t.Fatal(err)
	}
	if first.ImportedRows != 1 {
		t.Fatalf("expected 1 imported on first run, got %d", first.ImportedRows)
	}
	if first.UpdatedRows != 1 {
		t.Fatalf("expected 1 updated on first run, got %d", first.UpdatedRows)
	}
	if len(first.ProductIDs) != 1 || first.ProductIDs[0] != first.UpdatedProductIDs[0] {
		t.Fatalf("expected updated product id to match first imported, got %v / %v",
			first.ProductIDs, first.UpdatedProductIDs)
	}
}

func TestImportIdempotency(t *testing.T) {
	ctx, imp, _ := openDB(t)
	opts := importer.Options{Source: "manual-csv", Filename: "data.csv", IdempotencyKey: "fixture-1"}
	r1, err := imp.Import(ctx, strings.NewReader(validCSV), opts, "CN")
	if err != nil {
		t.Fatal(err)
	}
	r2, err := imp.Import(ctx, strings.NewReader(validCSV), opts, "CN")
	if err != nil {
		t.Fatal(err)
	}
	if r1.JobID != r2.JobID {
		t.Fatalf("expected idempotent job ids to match, got %s vs %s", r1.JobID, r2.JobID)
	}
}

func TestImportEscapesUnsafeValues(t *testing.T) {
	ctx, imp, pr := openDB(t)
	body := "title,region,review_summary\n=BADFORMULA(),CN,<script>alert(1)</script>\n"
	if _, err := imp.ImportFromPath(ctx, writeTempCSV(t, body), "CN", "manual-csv"); err != nil {
		t.Fatal(err)
	}
	row := pr.DB().QueryRowContext(ctx, `SELECT raw_json FROM import_job_rows LIMIT 1`)
	var raw string
	if err := row.Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "<script>") {
		t.Fatalf("expected html markup escaped in raw_json, got %s", raw)
	}
}

func TestImportRowLimit(t *testing.T) {
	ctx, imp, _ := openDB(t)
	body := "title,region\n"
	for i := 0; i < 50; i++ {
		body += "Item-" + strings.Repeat("x", i) + ",CN\n"
	}
	result, err := imp.ImportFromPath(ctx, writeTempCSV(t, body), "CN", "manual-csv")
	if err != nil {
		t.Fatal(err)
	}
	if result.ImportedRows+result.UpdatedRows != 50 {
		t.Fatalf("expected 50 rows imported or updated, got imported=%d updated=%d",
			result.ImportedRows, result.UpdatedRows)
	}
}

func writeTempCSV(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.csv")
	if err := writeFile(path, body); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFile(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o644)
}