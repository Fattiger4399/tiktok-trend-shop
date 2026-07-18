package review_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"tiktok-trend-shop/internal/copygen"
	"tiktok-trend-shop/internal/product"
	"tiktok-trend-shop/internal/request"
	"tiktok-trend-shop/internal/review"
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

// mustProduct creates a product with a detail snapshot and Amazon identity so
// delivery packages carry asin/marketplace.
func mustProduct(t *testing.T, db *sql.DB) product.Product {
	t.Helper()
	repo := product.NewRepository(db)
	p, err := repo.UpsertProduct(context.Background(), product.ProductInput{
		Title: "护眼台灯", Region: "US", Provider: "manual",
	})
	if err != nil {
		t.Fatalf("upsert product: %v", err)
	}
	if err := repo.UpdateAmazonIdentity(context.Background(), p.ID, "US", "B0ABCDEFGH", "2026-07-17T00:00:00Z"); err != nil {
		t.Fatalf("update amazon identity: %v", err)
	}
	if _, err := repo.AddDetailSnapshot(context.Background(), product.DetailInput{
		ProductID:     p.ID,
		Provider:      "manual",
		Brand:         strPtr("Glow"),
		SellingPoints: []string{"三档调光", "无频闪护眼"},
		ReviewSummary: strPtr("买家普遍反馈光线柔和"),
	}); err != nil {
		t.Fatalf("add detail snapshot: %v", err)
	}
	return p
}

type fixture struct {
	service  *review.Service
	requests *request.Repository
	copygen  *copygen.Service
	repo     *review.Repository
}

func newFixture(db *sql.DB) fixture {
	requests := request.NewRepository(db)
	variants := copygen.NewRepository(db)
	products := product.NewRepository(db)
	return fixture{
		service:  review.NewService(requests, products, variants, review.NewRepository(db)),
		requests: requests,
		copygen:  copygen.NewService(requests, products, variants, copygen.MockProvider{}),
		repo:     review.NewRepository(db),
	}
}

// mustGenerated creates a request and generates copy variants for it.
func mustGenerated(t *testing.T, f fixture, productID string) (request.MaterialRequest, []copygen.CopyVariant) {
	t.Helper()
	req, err := f.requests.Create(context.Background(), request.RequestInput{
		ProductID: productID,
		Usage:     "短视频带货",
		Style:     "真实测评",
		Focus:     "护眼",
	})
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	variants, err := f.copygen.GenerateForRequest(context.Background(), req.ID)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(variants) == 0 {
		t.Fatalf("expected variants")
	}
	return req, variants
}

func mustApproved(t *testing.T, f fixture, productID string) (request.MaterialRequest, copygen.CopyVariant) {
	t.Helper()
	req, variants := mustGenerated(t, f, productID)
	if _, err := f.service.Approve(context.Background(), req.ID, variants[0].ID, "reviewer-1", "ok"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	return req, variants[0]
}

func TestApproveLifecycle(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db)
	f := newFixture(db)
	req, variants := mustGenerated(t, f, p.ID)

	event, err := f.service.Approve(ctx, req.ID, variants[1].ID, "reviewer-1", "第二版更有代入感")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if !strings.HasPrefix(event.ID, "rev_") {
		t.Fatalf("expected rev_ prefixed id got %q", event.ID)
	}
	if event.RequestID != req.ID || event.Action != review.ActionApproved ||
		event.Actor != "reviewer-1" || event.Note != "第二版更有代入感" {
		t.Fatalf("unexpected event %#v", event)
	}
	if event.VariantID == nil || *event.VariantID != variants[1].ID {
		t.Fatalf("expected variant_id %q got %#v", variants[1].ID, event.VariantID)
	}

	updated, err := f.requests.Get(ctx, req.ID)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	if updated.Status != request.StatusApproved {
		t.Fatalf("expected status approved got %q", updated.Status)
	}

	events, err := f.service.ListEvents(ctx, req.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 || events[0].ID != event.ID {
		t.Fatalf("unexpected events %#v", events)
	}
}

func TestApproveRequiresGeneratedStatus(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db)
	f := newFixture(db)
	req, variants := mustGenerated(t, f, p.ID)

	var statusErr *review.StatusError
	if _, err := f.service.Approve(ctx, req.ID, variants[0].ID, "", ""); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := f.service.Approve(ctx, req.ID, variants[0].ID, "", ""); !errors.As(err, &statusErr) {
		t.Fatalf("expected StatusError for approved request got %v", err)
	}
	if statusErr.Status != request.StatusApproved {
		t.Fatalf("expected status approved in error got %q", statusErr.Status)
	}

	// A fresh submitted request cannot be approved either.
	fresh, err := f.requests.Create(ctx, request.RequestInput{ProductID: p.ID})
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if _, err := f.service.Approve(ctx, fresh.ID, variants[0].ID, "", ""); !errors.As(err, &statusErr) {
		t.Fatalf("expected StatusError for submitted request got %v", err)
	}
	if _, err := f.service.Approve(ctx, "req_missing", variants[0].ID, "", ""); err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows got %v", err)
	}
}

func TestApproveVariantMismatch(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db)
	f := newFixture(db)
	req, _ := mustGenerated(t, f, p.ID)
	other, otherVariants := mustGenerated(t, f, p.ID)

	var variantErr *review.VariantError
	if _, err := f.service.Approve(ctx, req.ID, otherVariants[0].ID, "", ""); !errors.As(err, &variantErr) {
		t.Fatalf("expected VariantError for foreign variant got %v", err)
	}
	if _, err := f.service.Approve(ctx, req.ID, "cv_missing", "", ""); !errors.As(err, &variantErr) {
		t.Fatalf("expected VariantError for missing variant got %v", err)
	}

	updated, err := f.requests.Get(ctx, req.ID)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	if updated.Status != request.StatusGenerated {
		t.Fatalf("expected status unchanged got %q", updated.Status)
	}
	events, err := f.service.ListEvents(ctx, other.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no events got %#v", events)
	}
}

func TestRejectLifecycle(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db)
	f := newFixture(db)
	req, _ := mustGenerated(t, f, p.ID)

	event, err := f.service.Reject(ctx, req.ID, "reviewer-2", "卖点不突出，重新生成")
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if event.Action != review.ActionRejected || event.Note != "卖点不突出，重新生成" ||
		event.Actor != "reviewer-2" {
		t.Fatalf("unexpected event %#v", event)
	}
	if event.VariantID != nil {
		t.Fatalf("expected nil variant_id on rejection got %#v", event.VariantID)
	}

	updated, err := f.requests.Get(ctx, req.ID)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	if updated.Status != request.StatusRejected {
		t.Fatalf("expected status rejected got %q", updated.Status)
	}

	// A rejected request may enter another generation round. The second batch
	// is appended, so the request now lists both rounds.
	variants, err := f.copygen.GenerateForRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if len(variants) != 6 {
		t.Fatalf("expected 6 variants across two rounds got %d", len(variants))
	}
}

func TestRejectRequiresNote(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db)
	f := newFixture(db)
	req, _ := mustGenerated(t, f, p.ID)

	if _, err := f.service.Reject(ctx, req.ID, "reviewer-2", "  "); err == nil {
		t.Fatalf("expected error for empty note")
	}
	updated, err := f.requests.Get(ctx, req.ID)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	if updated.Status != request.StatusGenerated {
		t.Fatalf("expected status unchanged got %q", updated.Status)
	}
	events, err := f.service.ListEvents(ctx, req.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no events got %#v", events)
	}
}

func TestRejectRequiresGeneratedStatus(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db)
	f := newFixture(db)
	req, _ := mustApproved(t, f, p.ID)

	var statusErr *review.StatusError
	if _, err := f.service.Reject(ctx, req.ID, "", "打回"); !errors.As(err, &statusErr) {
		t.Fatalf("expected StatusError for approved request got %v", err)
	}
	if statusErr.Status != request.StatusApproved {
		t.Fatalf("expected status approved in error got %q", statusErr.Status)
	}
}

func TestDeliverLifecycle(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db)
	f := newFixture(db)
	req, variant := mustApproved(t, f, p.ID)

	delivery, err := f.service.Deliver(ctx, req.ID, variant.ID, "ops-1")
	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if !strings.HasPrefix(delivery.ID, "dlv_") {
		t.Fatalf("expected dlv_ prefixed id got %q", delivery.ID)
	}
	if delivery.RequestID != req.ID || delivery.VariantID != variant.ID ||
		delivery.Actor != "ops-1" || delivery.CreatedAt == "" {
		t.Fatalf("unexpected delivery %#v", delivery)
	}

	var pkg review.DeliveryPackage
	if err := json.Unmarshal(delivery.Package, &pkg); err != nil {
		t.Fatalf("unmarshal package: %v", err)
	}
	if pkg.Request.ID != req.ID || pkg.Variant.ID != variant.ID {
		t.Fatalf("unexpected package %#v", pkg)
	}
	if pkg.Product.ID != p.ID || pkg.Product.Title != "护眼台灯" {
		t.Fatalf("unexpected product summary %#v", pkg.Product)
	}
	if pkg.Product.ASIN == nil || *pkg.Product.ASIN != "B0ABCDEFGH" ||
		pkg.Product.Marketplace == nil || *pkg.Product.Marketplace != "US" {
		t.Fatalf("expected asin/marketplace in package got %#v", pkg.Product)
	}

	updated, err := f.requests.Get(ctx, req.ID)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	if updated.Status != request.StatusDelivered {
		t.Fatalf("expected status delivered got %q", updated.Status)
	}

	stored, err := f.service.GetDelivery(ctx, req.ID)
	if err != nil {
		t.Fatalf("get delivery: %v", err)
	}
	if stored.ID != delivery.ID {
		t.Fatalf("expected delivery %q got %q", delivery.ID, stored.ID)
	}
	byID, err := f.service.GetDeliveryByID(ctx, delivery.ID)
	if err != nil {
		t.Fatalf("get delivery by id: %v", err)
	}
	if byID.RequestID != req.ID {
		t.Fatalf("unexpected delivery %#v", byID)
	}
}

func TestDeliverRequiresApprovedStatus(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db)
	f := newFixture(db)
	req, variants := mustGenerated(t, f, p.ID)

	var statusErr *review.StatusError
	if _, err := f.service.Deliver(ctx, req.ID, variants[0].ID, ""); !errors.As(err, &statusErr) {
		t.Fatalf("expected StatusError for generated request got %v", err)
	}
	if statusErr.Status != request.StatusGenerated {
		t.Fatalf("expected status generated in error got %q", statusErr.Status)
	}
}

func TestDeliverDuplicateConflict(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db)
	f := newFixture(db)
	req, variant := mustApproved(t, f, p.ID)

	if _, err := f.service.Deliver(ctx, req.ID, variant.ID, "ops-1"); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	// The request is now delivered, so a second deliver hits the status gate.
	var statusErr *review.StatusError
	if _, err := f.service.Deliver(ctx, req.ID, variant.ID, "ops-1"); !errors.As(err, &statusErr) {
		t.Fatalf("expected StatusError for delivered request got %v", err)
	}

	// The UNIQUE constraint still guards direct duplicate inserts.
	_, err := f.repo.CreateDelivery(ctx, review.DeliveryInput{
		RequestID: req.ID, VariantID: variant.ID, PackageJSON: `{}`,
	})
	if !errors.Is(err, review.ErrAlreadyDelivered) {
		t.Fatalf("expected ErrAlreadyDelivered got %v", err)
	}

	items, total, err := f.service.ListDeliveries(ctx, 1, 20)
	if err != nil {
		t.Fatalf("list deliveries: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected 1 delivery got total=%d len=%d", total, len(items))
	}
}

func TestReviewHistoryChronological(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db)
	f := newFixture(db)
	req, _ := mustGenerated(t, f, p.ID)

	if _, err := f.service.Reject(ctx, req.ID, "reviewer-2", "第一轮不行"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	variants, err := f.copygen.GenerateForRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if _, err := f.service.Approve(ctx, req.ID, variants[0].ID, "reviewer-1", "第二轮通过"); err != nil {
		t.Fatalf("approve: %v", err)
	}

	events, err := f.service.ListEvents(ctx, req.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events got %#v", events)
	}
	if events[0].Action != review.ActionRejected || events[1].Action != review.ActionApproved {
		t.Fatalf("expected chronological reject-then-approve got %#v", events)
	}
}
