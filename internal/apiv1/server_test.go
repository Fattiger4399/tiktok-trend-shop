package apiv1_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"tiktok-trend-shop/internal/apiv1"
	"tiktok-trend-shop/internal/category"
	"tiktok-trend-shop/internal/importer"
	"tiktok-trend-shop/internal/product"
	"tiktok-trend-shop/internal/score"
	"tiktok-trend-shop/internal/store"
)

type harness struct {
	t        *testing.T
	db       *sql.DB
	server   *apiv1.Server
	products *product.Repository
}

func newHarness(t *testing.T) *harness {
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
	cat := category.NewRepository(db)
	if err := cat.SeedCanonical(context.Background()); err != nil {
		t.Fatal(err)
	}
	imp := importer.New(db, pr)
	sc := score.NewRepository(db, pr)
	return &harness{t: t, db: db, server: apiv1.NewServer(db, pr, cat, imp, sc), products: pr}
}

func (h *harness) mustProduct(title string) *product.Product {
	h.t.Helper()
	p, err := h.products.UpsertProduct(context.Background(), product.ProductInput{
		Title: title, Region: "US", Provider: "manual",
	})
	if err != nil {
		h.t.Fatal(err)
	}
	return &p
}

func (h *harness) do(method, path string, body string, headers map[string]string) *httptest.ResponseRecorder {
	h.t.Helper()
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	var req *http.Request
	if reader != nil {
		req = httptest.NewRequest(method, path, reader)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	h.server.Handler().ServeHTTP(rr, req)
	return rr
}

func TestListTrendsDefaults(t *testing.T) {
	h := newHarness(t)
	h.mustProduct("Lamp")
	rr := h.do(http.MethodGet, "/api/v1/trends", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload apiv1.ListResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Pagination.PageSize == 0 {
		t.Fatalf("expected pagination page_size")
	}
}

func TestGetProductDetail(t *testing.T) {
	h := newHarness(t)
	p := h.mustProduct("Desk Lamp")
	rr := h.do(http.MethodGet, "/api/v1/products/"+p.ID, "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestGetProductNotFound(t *testing.T) {
	h := newHarness(t)
	rr := h.do(http.MethodGet, "/api/v1/products/does-not-exist", "", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestImportCSVRoundTrip(t *testing.T) {
	h := newHarness(t)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fw, err := writer.CreateFormFile("file", "products.csv")
	if err != nil {
		t.Fatal(err)
	}
	csv := "title,region,marketplace,asin,views,sales\nLamp,US,US,B0ABCDEFGH,1000,15\n"
	if _, err := fw.Write([]byte(csv)); err != nil {
		t.Fatal(err)
	}
	_ = writer.WriteField("region", "US")
	_ = writer.WriteField("idempotency_key", "test-key-1")
	_ = writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/imports/csv", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	h.server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		JobID        string `json:"job_id"`
		ImportedRows int    `json:"imported_rows"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.JobID == "" || resp.ImportedRows != 1 {
		t.Fatalf("unexpected payload %#v", resp)
	}
	rr2 := h.do(http.MethodGet, "/api/v1/imports/"+resp.JobID, "", nil)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rr2.Code)
	}
}

func TestImportCSVRejectsInvalidFile(t *testing.T) {
	h := newHarness(t)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fw, err := writer.CreateFormFile("file", "products.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte("region\nUS\n")); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/imports/csv", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	h.server.Handler().ServeHTTP(rr, req)
	if rr.Code == http.StatusCreated {
		t.Fatalf("expected rejection, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCategoryListAndAssignment(t *testing.T) {
	h := newHarness(t)
	rr := h.do(http.MethodGet, "/api/v1/categories", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rr.Code)
	}
	p := h.mustProduct("Lamp")
	rr2 := h.do(http.MethodPatch, "/api/v1/products/"+p.ID+"/category",
		`{"canonical_category_id":"cat-home","reviewer":"tester"}`,
		map[string]string{"Content-Type": "application/json"})
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr2.Code, rr2.Body.String())
	}
}

func TestGetProductMetrics(t *testing.T) {
	h := newHarness(t)
	p := h.mustProduct("Lamp")
	for i := 0; i < 3; i++ {
		if _, err := h.products.AddSourceSnapshot(context.Background(), product.SnapshotInput{
			ProductID: p.ID, Provider: "manual", SourceType: "product",
			Metrics: map[string]float64{"views": float64(100 * (i + 1))},
		}); err != nil {
			t.Fatal(err)
		}
	}
	rr := h.do(http.MethodGet, "/api/v1/products/"+p.ID+"/metrics?window=all", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
}