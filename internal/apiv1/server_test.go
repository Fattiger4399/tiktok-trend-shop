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
	"tiktok-trend-shop/internal/auth"
	"tiktok-trend-shop/internal/category"
	"tiktok-trend-shop/internal/copygen"
	"tiktok-trend-shop/internal/dossier"
	"tiktok-trend-shop/internal/importer"
	"tiktok-trend-shop/internal/mediagen"
	"tiktok-trend-shop/internal/product"
	"tiktok-trend-shop/internal/request"
	"tiktok-trend-shop/internal/review"
	"tiktok-trend-shop/internal/score"
	"tiktok-trend-shop/internal/store"
)

type harness struct {
	t        *testing.T
	db       *sql.DB
	server   *apiv1.Server
	products *product.Repository
	auth     *auth.Service
	token    string
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
	rq := request.NewRepository(db)
	cgRepo := copygen.NewRepository(db)
	cg := copygen.NewService(rq, pr, cgRepo, copygen.MockProvider{})
	rv := review.NewService(rq, pr, cgRepo, review.NewRepository(db))
	authSvc := auth.NewService(auth.NewRepository(db))
	if _, err := authSvc.CreateUser(context.Background(), "admin", "admin123", auth.RoleOperator, "Admin"); err != nil {
		t.Fatal(err)
	}
	token, _, err := authSvc.Login(context.Background(), "admin", "admin123")
	if err != nil {
		t.Fatal(err)
	}
	ds := dossier.NewRepository(db)
	mg := mediagen.NewService(pr, mediagen.NewAssetStore(db, filepath.Join(t.TempDir(), "assets")), mediagen.NewRepository(db), &mediagen.MockProvider{})
	return &harness{
		t: t, db: db, products: pr, auth: authSvc, token: token,
		server: apiv1.NewServer(db, pr, cat, imp, sc, rq, cg, rv, authSvc, ds, mg),
	}
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
	if req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}
	rr := httptest.NewRecorder()
	h.server.Handler().ServeHTTP(rr, req)
	return rr
}

// doAs performs a request authenticated as the given token holder.
func (h *harness) doAs(token, method, path string, body string, headers map[string]string) *httptest.ResponseRecorder {
	h.t.Helper()
	if headers == nil {
		headers = map[string]string{}
	}
	headers["Authorization"] = "Bearer " + token
	return h.do(method, path, body, headers)
}

// mustUser creates a user with the given role and returns it.
func (h *harness) mustUser(username, password, role string) auth.User {
	h.t.Helper()
	user, err := h.auth.CreateUser(context.Background(), username, password, role, username+" display")
	if err != nil {
		h.t.Fatal(err)
	}
	return user
}

// mustLogin logs the user in and returns the bearer token.
func (h *harness) mustLogin(username, password string) string {
	h.t.Helper()
	token, _, err := h.auth.Login(context.Background(), username, password)
	if err != nil {
		h.t.Fatal(err)
	}
	return token
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
	req.Header.Set("Authorization", "Bearer "+h.token)
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
	req.Header.Set("Authorization", "Bearer "+h.token)
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
func TestListMarketplaces(t *testing.T) {
	h := newHarness(t)
	p := h.mustProduct("Lamp")
	if err := h.products.UpdateAmazonIdentity(context.Background(), p.ID, "US", "B0ABCDEFGH", "2026-07-17T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.products.UpsertProduct(context.Background(), product.ProductInput{
		Title: "Bottle", Region: "JP", Provider: "manual",
	}); err != nil {
		t.Fatal(err)
	}
	rr := h.do(http.MethodGet, "/api/v1/marketplaces", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload struct {
		Items []string `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Items) != 2 || payload.Items[0] != "JP" || payload.Items[1] != "US" {
		t.Fatalf("unexpected marketplaces %#v", payload.Items)
	}
}

func (h *harness) mustRequest(productID, usage string) request.MaterialRequest {
	h.t.Helper()
	rr := h.do(http.MethodPost, "/api/v1/requests",
		`{"product_id":"`+productID+`","usage":"`+usage+`"}`,
		map[string]string{"Content-Type": "application/json"})
	if rr.Code != http.StatusCreated {
		h.t.Fatalf("create request: expected 201 got %d body=%s", rr.Code, rr.Body.String())
	}
	var created request.MaterialRequest
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		h.t.Fatal(err)
	}
	return created
}

func TestRequestCreateRoundTrip(t *testing.T) {
	h := newHarness(t)
	p := h.mustProduct("Lamp")
	rr := h.do(http.MethodPost, "/api/v1/requests",
		`{"product_id":"`+p.ID+`","usage":"listing","style":"minimal","focus":"durability","notes":"highlight warranty","client_id":"client-1"}`,
		map[string]string{"Content-Type": "application/json"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d body=%s", rr.Code, rr.Body.String())
	}
	var created request.MaterialRequest
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(created.ID, "req_") {
		t.Fatalf("expected req_ prefixed id got %q", created.ID)
	}
	if created.ProductID != p.ID || created.Status != "submitted" ||
		created.Usage != "listing" || created.ClientID != "client-1" {
		t.Fatalf("unexpected request %#v", created)
	}
	rr2 := h.do(http.MethodGet, "/api/v1/requests/"+created.ID, "", nil)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr2.Code, rr2.Body.String())
	}
	var fetched request.MaterialRequest
	if err := json.Unmarshal(rr2.Body.Bytes(), &fetched); err != nil {
		t.Fatal(err)
	}
	if fetched.ID != created.ID || fetched.Notes != "highlight warranty" {
		t.Fatalf("unexpected request %#v", fetched)
	}
}

func TestCreateRequestRequiresProductID(t *testing.T) {
	h := newHarness(t)
	rr := h.do(http.MethodPost, "/api/v1/requests", `{"usage":"listing"}`,
		map[string]string{"Content-Type": "application/json"})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload struct {
		Error struct {
			Code  string `json:"code"`
			Field string `json:"field"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != "validation_error" || payload.Error.Field != "product_id" {
		t.Fatalf("unexpected error envelope %#v", payload.Error)
	}
}

func TestCreateRequestUnknownProduct(t *testing.T) {
	h := newHarness(t)
	rr := h.do(http.MethodPost, "/api/v1/requests", `{"product_id":"prod_missing"}`,
		map[string]string{"Content-Type": "application/json"})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestGetRequestNotFound(t *testing.T) {
	h := newHarness(t)
	rr := h.do(http.MethodGet, "/api/v1/requests/req_missing", "", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestListRequestsFilters(t *testing.T) {
	h := newHarness(t)
	p1 := h.mustProduct("Lamp")
	p2 := h.mustProduct("Bottle")
	h.mustRequest(p1.ID, "listing")
	h.mustRequest(p1.ID, "social")
	h.mustRequest(p2.ID, "ad")

	var payload struct {
		Items      []request.MaterialRequest `json:"items"`
		Pagination apiv1.Pagination          `json:"pagination"`
	}
	rr := h.do(http.MethodGet, "/api/v1/requests?product_id="+p1.ID, "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Pagination.TotalItems != 2 || len(payload.Items) != 2 {
		t.Fatalf("expected 2 requests for p1 got total=%d len=%d",
			payload.Pagination.TotalItems, len(payload.Items))
	}
	for _, item := range payload.Items {
		if item.ProductID != p1.ID {
			t.Fatalf("unexpected product_id %q", item.ProductID)
		}
	}

	rr = h.do(http.MethodGet, "/api/v1/requests?status=submitted", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Pagination.TotalItems != 3 || len(payload.Items) != 3 {
		t.Fatalf("expected 3 submitted requests got total=%d len=%d",
			payload.Pagination.TotalItems, len(payload.Items))
	}

	rr = h.do(http.MethodGet, "/api/v1/requests?page=2&page_size=2", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Pagination.TotalItems != 3 || len(payload.Items) != 1 {
		t.Fatalf("expected page 2 with 1 of 3 got total=%d len=%d",
			payload.Pagination.TotalItems, len(payload.Items))
	}
}

func strPtr(value string) *string { return &value }

func (h *harness) mustProductWithDetail(title string) *product.Product {
	h.t.Helper()
	p := h.mustProduct(title)
	if _, err := h.products.AddDetailSnapshot(context.Background(), product.DetailInput{
		ProductID:     p.ID,
		Provider:      "manual",
		Brand:         strPtr("Glow"),
		SellingPoints: []string{"三档调光", "无频闪护眼"},
		ReviewSummary: strPtr("买家普遍反馈光线柔和"),
	}); err != nil {
		h.t.Fatal(err)
	}
	return p
}

func TestPrefillNotFound(t *testing.T) {
	h := newHarness(t)
	rr := h.do(http.MethodPost, "/api/v1/products/prod_missing/prefill", "", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestPrefillReturnsSuggestion(t *testing.T) {
	h := newHarness(t)
	p := h.mustProductWithDetail("护眼台灯")
	rr := h.do(http.MethodPost, "/api/v1/products/"+p.ID+"/prefill", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload struct {
		ProductID  string `json:"product_id"`
		Suggestion struct {
			Usage string `json:"usage"`
			Style string `json:"style"`
			Focus string `json:"focus"`
			Notes string `json:"notes"`
		} `json:"suggestion"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ProductID != p.ID {
		t.Fatalf("unexpected product_id %q", payload.ProductID)
	}
	if payload.Suggestion.Usage == "" || payload.Suggestion.Style == "" ||
		payload.Suggestion.Focus == "" {
		t.Fatalf("expected populated suggestion got %#v", payload.Suggestion)
	}
	if payload.Suggestion.Focus != "三档调光" {
		t.Fatalf("expected focus from first selling point got %q", payload.Suggestion.Focus)
	}
}

func TestGenerateVariantsNotFound(t *testing.T) {
	h := newHarness(t)
	rr := h.do(http.MethodPost, "/api/v1/requests/req_missing/generate", "", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestGenerateVariantsRoundTrip(t *testing.T) {
	h := newHarness(t)
	p := h.mustProductWithDetail("护眼台灯")
	req := h.mustRequest(p.ID, "短视频带货")

	rr := h.do(http.MethodPost, "/api/v1/requests/"+req.ID+"/generate", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload struct {
		RequestID string                `json:"request_id"`
		Items     []copygen.CopyVariant `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.RequestID != req.ID || len(payload.Items) != 3 {
		t.Fatalf("unexpected payload %#v", payload)
	}
	for i, variant := range payload.Items {
		if variant.VariantNo != i+1 || variant.Provider != "mock" || variant.Hook == "" {
			t.Fatalf("unexpected variant %#v", variant)
		}
	}

	rr = h.do(http.MethodGet, "/api/v1/requests/"+req.ID, "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var fetched request.MaterialRequest
	if err := json.Unmarshal(rr.Body.Bytes(), &fetched); err != nil {
		t.Fatal(err)
	}
	if fetched.Status != "generated" {
		t.Fatalf("expected status generated got %q", fetched.Status)
	}

	rr = h.do(http.MethodGet, "/api/v1/requests/"+req.ID+"/variants", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var list struct {
		Items []copygen.CopyVariant `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 3 {
		t.Fatalf("expected 3 variants got %d", len(list.Items))
	}
	for i, variant := range list.Items {
		if variant.VariantNo != i+1 {
			t.Fatalf("expected ascending variant_no got %#v", list.Items)
		}
	}
}

func TestGenerateVariantsConflict(t *testing.T) {
	h := newHarness(t)
	p := h.mustProductWithDetail("护眼台灯")
	req := h.mustRequest(p.ID, "短视频带货")

	rr := h.do(http.MethodPost, "/api/v1/requests/"+req.ID+"/generate", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	rr = h.do(http.MethodPost, "/api/v1/requests/"+req.ID+"/generate", "", nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != "invalid_status" {
		t.Fatalf("expected invalid_status error got %#v", payload.Error)
	}
}

func TestListVariantsEmptyAndNotFound(t *testing.T) {
	h := newHarness(t)
	p := h.mustProductWithDetail("护眼台灯")
	req := h.mustRequest(p.ID, "短视频带货")

	rr := h.do(http.MethodGet, "/api/v1/requests/"+req.ID+"/variants", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload struct {
		Items []copygen.CopyVariant `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Items == nil || len(payload.Items) != 0 {
		t.Fatalf("expected empty items array got %#v", payload.Items)
	}

	rr = h.do(http.MethodGet, "/api/v1/requests/req_missing/variants", "", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func (h *harness) mustGenerate(requestID string) []copygen.CopyVariant {
	h.t.Helper()
	rr := h.do(http.MethodPost, "/api/v1/requests/"+requestID+"/generate", "", nil)
	if rr.Code != http.StatusOK {
		h.t.Fatalf("generate: expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload struct {
		Items []copygen.CopyVariant `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		h.t.Fatal(err)
	}
	if len(payload.Items) == 0 {
		h.t.Fatalf("expected variants")
	}
	return payload.Items
}

func errorEnvelopeOf(t *testing.T, rr *httptest.ResponseRecorder) struct {
	Code  string `json:"code"`
	Field string `json:"field"`
} {
	t.Helper()
	var payload struct {
		Error struct {
			Code  string `json:"code"`
			Field string `json:"field"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload.Error
}

func TestApproveDeliverRoutes(t *testing.T) {
	h := newHarness(t)
	p := h.mustProductWithDetail("护眼台灯")
	req := h.mustRequest(p.ID, "短视频带货")
	jsonHeaders := map[string]string{"Content-Type": "application/json"}

	// Approve before generation conflicts with the submitted status.
	rr := h.do(http.MethodPost, "/api/v1/requests/"+req.ID+"/approve",
		`{"variant_id":"cv_x"}`, jsonHeaders)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 got %d body=%s", rr.Code, rr.Body.String())
	}
	if env := errorEnvelopeOf(t, rr); env.Code != "invalid_status" {
		t.Fatalf("expected invalid_status got %#v", env)
	}

	variants := h.mustGenerate(req.ID)
	variantID := variants[0].ID

	// variant_id is required.
	rr = h.do(http.MethodPost, "/api/v1/requests/"+req.ID+"/approve", `{"actor":"ops"}`, jsonHeaders)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d body=%s", rr.Code, rr.Body.String())
	}
	if env := errorEnvelopeOf(t, rr); env.Field != "variant_id" {
		t.Fatalf("expected field variant_id got %#v", env)
	}

	// A variant of another request is rejected as a variant_id field error.
	other := h.mustRequest(p.ID, "短视频带货")
	otherVariants := h.mustGenerate(other.ID)
	rr = h.do(http.MethodPost, "/api/v1/requests/"+req.ID+"/approve",
		`{"variant_id":"`+otherVariants[0].ID+`"}`, jsonHeaders)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d body=%s", rr.Code, rr.Body.String())
	}
	if env := errorEnvelopeOf(t, rr); env.Field != "variant_id" {
		t.Fatalf("expected field variant_id got %#v", env)
	}

	// Approve with a valid variant.
	rr = h.do(http.MethodPost, "/api/v1/requests/"+req.ID+"/approve",
		`{"variant_id":"`+variantID+`","actor":"reviewer-1","note":"通过"}`, jsonHeaders)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var approveResp struct {
		Request request.MaterialRequest `json:"request"`
		Event   review.ReviewEvent      `json:"event"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &approveResp); err != nil {
		t.Fatal(err)
	}
	if approveResp.Request.Status != "approved" {
		t.Fatalf("expected status approved got %q", approveResp.Request.Status)
	}
	if approveResp.Event.Action != "approved" || approveResp.Event.Actor != "reviewer-1" ||
		approveResp.Event.VariantID == nil || *approveResp.Event.VariantID != variantID {
		t.Fatalf("unexpected event %#v", approveResp.Event)
	}

	// Approving an approved request conflicts.
	rr = h.do(http.MethodPost, "/api/v1/requests/"+req.ID+"/approve",
		`{"variant_id":"`+variantID+`"}`, jsonHeaders)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 got %d body=%s", rr.Code, rr.Body.String())
	}

	// Deliver requires variant_id.
	rr = h.do(http.MethodPost, "/api/v1/requests/"+req.ID+"/deliver", `{"actor":"ops"}`, jsonHeaders)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d body=%s", rr.Code, rr.Body.String())
	}

	// Deliver the approved request.
	rr = h.do(http.MethodPost, "/api/v1/requests/"+req.ID+"/deliver",
		`{"variant_id":"`+variantID+`","actor":"ops-1"}`, jsonHeaders)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var deliverResp struct {
		Request  request.MaterialRequest `json:"request"`
		Delivery review.Delivery         `json:"delivery"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &deliverResp); err != nil {
		t.Fatal(err)
	}
	if deliverResp.Request.Status != "delivered" {
		t.Fatalf("expected status delivered got %q", deliverResp.Request.Status)
	}
	if !strings.HasPrefix(deliverResp.Delivery.ID, "dlv_") {
		t.Fatalf("expected dlv_ prefixed id got %q", deliverResp.Delivery.ID)
	}
	var pkg review.DeliveryPackage
	if err := json.Unmarshal(deliverResp.Delivery.Package, &pkg); err != nil {
		t.Fatalf("unmarshal package: %v", err)
	}
	if pkg.Request.ID != req.ID || pkg.Variant.ID != variantID ||
		pkg.Product.ID != p.ID || pkg.Product.Title != "护眼台灯" {
		t.Fatalf("unexpected package %#v", pkg)
	}

	// Repeated delivery conflicts.
	rr = h.do(http.MethodPost, "/api/v1/requests/"+req.ID+"/deliver",
		`{"variant_id":"`+variantID+`"}`, jsonHeaders)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 got %d body=%s", rr.Code, rr.Body.String())
	}

	// The review aggregate returns request, events and delivery together.
	rr = h.do(http.MethodGet, "/api/v1/requests/"+req.ID+"/review", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var reviewResp struct {
		Request  request.MaterialRequest `json:"request"`
		Events   []review.ReviewEvent    `json:"events"`
		Delivery *review.Delivery        `json:"delivery"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &reviewResp); err != nil {
		t.Fatal(err)
	}
	if reviewResp.Request.Status != "delivered" || len(reviewResp.Events) != 1 ||
		reviewResp.Delivery == nil || reviewResp.Delivery.ID != deliverResp.Delivery.ID {
		t.Fatalf("unexpected review aggregate %#v", reviewResp)
	}

	// Delivery list and detail for operations export.
	rr = h.do(http.MethodGet, "/api/v1/deliveries", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var listResp struct {
		Items      []review.Delivery `json:"items"`
		Pagination apiv1.Pagination  `json:"pagination"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if listResp.Pagination.TotalItems != 1 || len(listResp.Items) != 1 ||
		listResp.Items[0].ID != deliverResp.Delivery.ID {
		t.Fatalf("unexpected deliveries list %#v", listResp)
	}

	rr = h.do(http.MethodGet, "/api/v1/deliveries/"+deliverResp.Delivery.ID, "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	rr = h.do(http.MethodGet, "/api/v1/deliveries/dlv_missing", "", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d body=%s", rr.Code, rr.Body.String())
	}

	// Review aggregate of an unknown request is a 404.
	rr = h.do(http.MethodGet, "/api/v1/requests/req_missing/review", "", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestRejectRoute(t *testing.T) {
	h := newHarness(t)
	p := h.mustProductWithDetail("护眼台灯")
	req := h.mustRequest(p.ID, "短视频带货")
	jsonHeaders := map[string]string{"Content-Type": "application/json"}

	// reason is required and maps to a 400 field error.
	rr := h.do(http.MethodPost, "/api/v1/requests/"+req.ID+"/reject", `{"actor":"ops"}`, jsonHeaders)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d body=%s", rr.Code, rr.Body.String())
	}
	if env := errorEnvelopeOf(t, rr); env.Code != "validation_error" || env.Field != "reason" {
		t.Fatalf("expected reason field error got %#v", env)
	}

	h.mustGenerate(req.ID)

	rr = h.do(http.MethodPost, "/api/v1/requests/"+req.ID+"/reject",
		`{"reason":"卖点不突出","actor":"reviewer-2"}`, jsonHeaders)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var rejectResp struct {
		Request request.MaterialRequest `json:"request"`
		Event   review.ReviewEvent      `json:"event"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &rejectResp); err != nil {
		t.Fatal(err)
	}
	if rejectResp.Request.Status != "rejected" {
		t.Fatalf("expected status rejected got %q", rejectResp.Request.Status)
	}
	if rejectResp.Event.Action != "rejected" || rejectResp.Event.Note != "卖点不突出" ||
		rejectResp.Event.VariantID != nil {
		t.Fatalf("unexpected event %#v", rejectResp.Event)
	}

	// Rejecting a rejected request conflicts.
	rr = h.do(http.MethodPost, "/api/v1/requests/"+req.ID+"/reject", `{"reason":"again"}`, jsonHeaders)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 got %d body=%s", rr.Code, rr.Body.String())
	}

	// The aggregate shows the event and a null delivery.
	rr = h.do(http.MethodGet, "/api/v1/requests/"+req.ID+"/review", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var reviewResp struct {
		Request  request.MaterialRequest `json:"request"`
		Events   []review.ReviewEvent    `json:"events"`
		Delivery *review.Delivery        `json:"delivery"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &reviewResp); err != nil {
		t.Fatal(err)
	}
	if len(reviewResp.Events) != 1 || reviewResp.Delivery != nil {
		t.Fatalf("unexpected review aggregate %#v", reviewResp)
	}

	// A rejected request can enter another generation round.
	rr = h.do(http.MethodPost, "/api/v1/requests/"+req.ID+"/generate", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestGetProductDetailNullables(t *testing.T) {
	h := newHarness(t)
	p := h.mustProduct("Plain Lamp")
	rr := h.do(http.MethodGet, "/api/v1/products/"+p.ID, "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"detail", "latest_snapshot", "score", "assignment"} {
		if string(payload[key]) != "null" {
			t.Fatalf("expected %s to be null, got %s", key, payload[key])
		}
	}
}

func TestGetProductDetailSnakeCaseKeys(t *testing.T) {
	h := newHarness(t)
	p := h.mustProduct("Lamp")
	price := 19.99
	if _, err := h.products.AddDetailSnapshot(context.Background(), product.DetailInput{
		ProductID: p.ID, Provider: "manual", Price: &price,
	}); err != nil {
		t.Fatal(err)
	}
	rr := h.do(http.MethodGet, "/api/v1/products/"+p.ID, "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload struct {
		Detail map[string]json.RawMessage `json:"detail"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Detail == nil {
		t.Fatalf("expected detail present, body=%s", rr.Body.String())
	}
	if _, ok := payload.Detail["price"]; !ok {
		t.Fatalf("expected snake_case key price, got keys %v", payload.Detail)
	}
	if _, ok := payload.Detail["Price"]; ok {
		t.Fatalf("unexpected PascalCase key leaked into response")
	}
}

func TestDossierAssetRoundTrip(t *testing.T) {
	h := newHarness(t)
	p := h.mustProduct("Desk Lamp")
	jsonHeaders := map[string]string{"Content-Type": "application/json"}

	rr := h.do(http.MethodPost, "/api/v1/products/"+p.ID+"/assets",
		`{"kind":"image","url":"https://example.com/lamp.jpg","source":"amazon.com","note":"主图"}`,
		jsonHeaders)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d body=%s", rr.Code, rr.Body.String())
	}
	var created dossier.Asset
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(created.ID, "da_") {
		t.Fatalf("expected da_ prefixed id got %q", created.ID)
	}
	if created.Kind != "image" || created.URL != "https://example.com/lamp.jpg" ||
		created.Source != "amazon.com" || created.CreatedBy != "admin" {
		t.Fatalf("unexpected asset %#v", created)
	}

	rr = h.do(http.MethodGet, "/api/v1/products/"+p.ID+"/assets", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var list struct {
		Items []dossier.Asset `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].ID != created.ID {
		t.Fatalf("unexpected list %#v", list.Items)
	}

	// kind filter narrows the list.
	rr = h.do(http.MethodGet, "/api/v1/products/"+p.ID+"/assets?kind=text", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("expected empty text filter result got %#v", list.Items)
	}

	rr = h.do(http.MethodDelete, "/api/v1/products/"+p.ID+"/assets/"+created.ID, "", nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 got %d body=%s", rr.Code, rr.Body.String())
	}
	rr = h.do(http.MethodGet, "/api/v1/products/"+p.ID+"/assets", "", nil)
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("expected asset removed got %#v", list.Items)
	}
}

func TestCreateDossierAssetValidation(t *testing.T) {
	h := newHarness(t)
	p := h.mustProduct("Desk Lamp")
	jsonHeaders := map[string]string{"Content-Type": "application/json"}

	cases := []struct {
		body  string
		field string
	}{
		{`{"url":"https://a/1.jpg"}`, "kind"},
		{`{"kind":"video","url":"https://a/1.jpg"}`, "kind"},
		{`{"kind":"image"}`, "url"},
		{`{"kind":"link"}`, "url"},
		{`{"kind":"text"}`, "content"},
	}
	for _, c := range cases {
		rr := h.do(http.MethodPost, "/api/v1/products/"+p.ID+"/assets", c.body, jsonHeaders)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for %s got %d body=%s", c.body, rr.Code, rr.Body.String())
		}
		if env := errorEnvelopeOf(t, rr); env.Code != "validation_error" || env.Field != c.field {
			t.Fatalf("expected field %q for %s got %#v", c.field, c.body, env)
		}
	}
}

func TestDossierAssetNotFound(t *testing.T) {
	h := newHarness(t)
	p := h.mustProduct("Desk Lamp")
	jsonHeaders := map[string]string{"Content-Type": "application/json"}

	rr := h.do(http.MethodPost, "/api/v1/products/prod_missing/assets",
		`{"kind":"text","content":"x"}`, jsonHeaders)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d body=%s", rr.Code, rr.Body.String())
	}
	rr = h.do(http.MethodGet, "/api/v1/products/prod_missing/assets", "", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d body=%s", rr.Code, rr.Body.String())
	}
	rr = h.do(http.MethodDelete, "/api/v1/products/"+p.ID+"/assets/da_missing", "", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d body=%s", rr.Code, rr.Body.String())
	}
}
