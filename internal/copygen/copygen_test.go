package copygen_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"tiktok-trend-shop/internal/copygen"
	"tiktok-trend-shop/internal/product"
	"tiktok-trend-shop/internal/request"
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

func strPtr(value string) *string { return &value }

// mustProduct creates a product with a detail snapshot carrying brand,
// selling points and a review summary.
func mustProduct(t *testing.T, db *sql.DB) product.Product {
	t.Helper()
	repo := product.NewRepository(db)
	p, err := repo.UpsertProduct(context.Background(), product.ProductInput{
		Title: "护眼台灯", Region: "US", Provider: "manual",
	})
	if err != nil {
		t.Fatalf("upsert product: %v", err)
	}
	if _, err := repo.AddDetailSnapshot(context.Background(), product.DetailInput{
		ProductID:     p.ID,
		Provider:      "manual",
		Brand:         strPtr("Glow"),
		SellingPoints: []string{"三档调光", "无频闪护眼"},
		Specs:         map[string]any{"功率": "12W"},
		ReviewSummary: strPtr("买家普遍反馈光线柔和"),
	}); err != nil {
		t.Fatalf("add detail snapshot: %v", err)
	}
	return p
}

func mustRequest(t *testing.T, repo *request.Repository, productID string) request.MaterialRequest {
	t.Helper()
	req, err := repo.Create(context.Background(), request.RequestInput{
		ProductID: productID,
		Usage:     "短视频带货",
		Style:     "真实测评",
		Focus:     "护眼",
	})
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	return req
}

func newService(db *sql.DB, provider copygen.Provider) (*copygen.Service, *request.Repository) {
	requests := request.NewRepository(db)
	return copygen.NewService(requests, product.NewRepository(db), copygen.NewRepository(db), provider), requests
}

func sampleGenInput() copygen.GenInput {
	return copygen.GenInput{
		ProductTitle:  "护眼台灯",
		Brand:         "Glow",
		SellingPoints: []string{"三档调光", "无频闪护眼"},
		Specs:         map[string]any{"功率": "12W"},
		ReviewSummary: "买家普遍反馈光线柔和",
		Usage:         "短视频带货",
		Style:         "真实测评",
		Focus:         "护眼",
		VariantCount:  3,
	}
}

func TestMockProviderDeterministic(t *testing.T) {
	provider := copygen.MockProvider{}
	input := sampleGenInput()
	first, err := provider.Generate(context.Background(), input)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	second, err := provider.Generate(context.Background(), input)
	if err != nil {
		t.Fatalf("generate again: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("expected deterministic output\nfirst=%#v\nsecond=%#v", first, second)
	}
	if len(first) != 3 {
		t.Fatalf("expected 3 variants got %d", len(first))
	}
	for i, draft := range first {
		if draft.Hook == "" || draft.Body == "" || draft.Caption == "" || len(draft.Hashtags) == 0 {
			t.Fatalf("variant %d has empty fields: %#v", i, draft)
		}
		if !strings.Contains(draft.Body, "护眼") && !strings.Contains(draft.Body, "三档调光") {
			t.Fatalf("variant %d body should reference the brief/profile: %q", i, draft.Body)
		}
	}
	if first[0].Hook == first[1].Hook && first[1].Hook == first[2].Hook {
		t.Fatalf("expected distinct angles across variants")
	}
}

func TestMockProviderPrefill(t *testing.T) {
	provider := copygen.MockProvider{}
	input := copygen.PrefillInput{
		ProductTitle:  "护眼台灯",
		Brand:         "Glow",
		SellingPoints: []string{"三档调光", "无频闪护眼"},
		ReviewSummary: "买家普遍反馈光线柔和",
	}
	first, err := provider.Prefill(context.Background(), input)
	if err != nil {
		t.Fatalf("prefill: %v", err)
	}
	second, err := provider.Prefill(context.Background(), input)
	if err != nil {
		t.Fatalf("prefill again: %v", err)
	}
	if first != second {
		t.Fatalf("expected deterministic prefill got %#v vs %#v", first, second)
	}
	if first.Usage == "" || first.Style == "" || first.Focus == "" {
		t.Fatalf("expected populated suggestion got %#v", first)
	}
	if first.Focus != "三档调光" {
		t.Fatalf("expected focus derived from first selling point got %q", first.Focus)
	}
	if !strings.Contains(first.Notes, "Glow") {
		t.Fatalf("expected notes to mention the brand got %q", first.Notes)
	}
}

func TestGenerateForRequestLifecycle(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db)
	service, requests := newService(db, copygen.MockProvider{})
	req := mustRequest(t, requests, p.ID)

	variants, err := service.GenerateForRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(variants) != 3 {
		t.Fatalf("expected 3 variants got %d", len(variants))
	}
	for i, variant := range variants {
		if !strings.HasPrefix(variant.ID, "cv_") {
			t.Fatalf("expected cv_ prefixed id got %q", variant.ID)
		}
		if variant.VariantNo != i+1 {
			t.Fatalf("expected variant_no %d got %d", i+1, variant.VariantNo)
		}
		if variant.RequestID != req.ID || variant.Provider != "mock" ||
			variant.PromptVersion != copygen.PromptVersion || variant.CreatedAt == "" {
			t.Fatalf("unexpected variant %#v", variant)
		}
		if len(variant.Hashtags) == 0 {
			t.Fatalf("expected hashtags on variant %#v", variant)
		}
	}
	if !strings.Contains(variants[0].Body, "Glow") {
		t.Fatalf("expected copy to reference the brand got %q", variants[0].Body)
	}

	updated, err := requests.Get(ctx, req.ID)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	if updated.Status != request.StatusGenerated {
		t.Fatalf("expected status generated got %q", updated.Status)
	}

	listed, err := service.ListVariants(ctx, req.ID)
	if err != nil {
		t.Fatalf("list variants: %v", err)
	}
	if !reflect.DeepEqual(listed, variants) {
		t.Fatalf("listed variants mismatch\nlisted=%#v\nsaved=%#v", listed, variants)
	}
}

func TestGenerateForRequestRejectsInvalidStatus(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db)
	service, requests := newService(db, copygen.MockProvider{})
	req := mustRequest(t, requests, p.ID)

	if _, err := service.GenerateForRequest(ctx, req.ID); err != nil {
		t.Fatalf("first generate: %v", err)
	}
	_, err := service.GenerateForRequest(ctx, req.ID)
	var statusErr *copygen.StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected StatusError for generated request got %v", err)
	}
	if statusErr.Status != request.StatusGenerated {
		t.Fatalf("expected status generated in error got %q", statusErr.Status)
	}

	if _, err := requests.UpdateStatus(ctx, req.ID, request.StatusApproved); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := service.GenerateForRequest(ctx, req.ID); !errors.As(err, &statusErr) {
		t.Fatalf("expected StatusError for approved request got %v", err)
	}
}

func TestGenerateForRequestNotFound(t *testing.T) {
	service, _ := newService(openTestDB(t), copygen.MockProvider{})
	if _, err := service.GenerateForRequest(context.Background(), "req_missing"); err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows got %v", err)
	}
}

func TestGenerateForRequestRejectedCanRegenerate(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db)
	service, requests := newService(db, copygen.MockProvider{})
	req := mustRequest(t, requests, p.ID)

	for _, next := range []string{request.StatusGenerating, request.StatusGenerated, request.StatusRejected} {
		if _, err := requests.UpdateStatus(ctx, req.ID, next); err != nil {
			t.Fatalf("transition to %q: %v", next, err)
		}
	}
	variants, err := service.GenerateForRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("regenerate after rejection: %v", err)
	}
	if len(variants) != 3 {
		t.Fatalf("expected 3 variants got %d", len(variants))
	}
	updated, err := requests.Get(ctx, req.ID)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	if updated.Status != request.StatusGenerated {
		t.Fatalf("expected status generated got %q", updated.Status)
	}
}

// failingProvider always fails generation to exercise the rollback path.
type failingProvider struct{}

func (failingProvider) Name() string { return "failing" }

func (failingProvider) Generate(context.Context, copygen.GenInput) ([]copygen.VariantDraft, error) {
	return nil, errors.New("provider unavailable")
}

func (failingProvider) Prefill(context.Context, copygen.PrefillInput) (copygen.PrefillSuggestion, error) {
	return copygen.PrefillSuggestion{}, errors.New("provider unavailable")
}

func TestGenerateForRequestRevertsToSubmittedOnFailure(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db)
	service, requests := newService(db, failingProvider{})
	req := mustRequest(t, requests, p.ID)

	if _, err := service.GenerateForRequest(ctx, req.ID); err == nil {
		t.Fatalf("expected generation error")
	}
	updated, err := requests.Get(ctx, req.ID)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	if updated.Status != request.StatusSubmitted {
		t.Fatalf("expected status reverted to submitted got %q", updated.Status)
	}
	variants, err := service.ListVariants(ctx, req.ID)
	if err != nil {
		t.Fatalf("list variants: %v", err)
	}
	if len(variants) != 0 {
		t.Fatalf("expected no persisted variants got %d", len(variants))
	}
}

func TestPrefillForProduct(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db)
	service, _ := newService(db, copygen.MockProvider{})

	suggestion, err := service.PrefillForProduct(ctx, p.ID)
	if err != nil {
		t.Fatalf("prefill: %v", err)
	}
	if suggestion.Usage == "" || suggestion.Style == "" || suggestion.Focus == "" {
		t.Fatalf("expected populated suggestion got %#v", suggestion)
	}
	if suggestion.Focus != "三档调光" {
		t.Fatalf("expected focus from selling points got %q", suggestion.Focus)
	}
	if _, err := service.PrefillForProduct(ctx, "prod_missing"); err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows got %v", err)
	}
}
