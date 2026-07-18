package request_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

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

func mustRequest(t *testing.T, repo *request.Repository, input request.RequestInput) request.MaterialRequest {
	t.Helper()
	req, err := repo.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	return req
}

func TestCreateAndGet(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db, "Desk Lamp")
	repo := request.NewRepository(db)

	created, err := repo.Create(ctx, request.RequestInput{
		ProductID: p.ID,
		ClientID:  "client-1",
		Usage:     "listing",
		Style:     "minimal",
		Focus:     "durability",
		Notes:     "highlight warranty",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !strings.HasPrefix(created.ID, "req_") {
		t.Fatalf("expected req_ prefixed id got %q", created.ID)
	}
	if created.Status != request.StatusSubmitted {
		t.Fatalf("expected status submitted got %q", created.Status)
	}
	if created.CreatedAt == "" || created.UpdatedAt == "" {
		t.Fatalf("expected timestamps to be populated")
	}
	got, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ProductID != p.ID || got.ClientID != "client-1" || got.Usage != "listing" ||
		got.Style != "minimal" || got.Focus != "durability" || got.Notes != "highlight warranty" {
		t.Fatalf("unexpected request %#v", got)
	}
}

func TestCreateRequiresProductID(t *testing.T) {
	repo := request.NewRepository(openTestDB(t))
	if _, err := repo.Create(context.Background(), request.RequestInput{}); err == nil {
		t.Fatalf("expected error for empty product_id")
	}
}

func TestGetNotFound(t *testing.T) {
	repo := request.NewRepository(openTestDB(t))
	if _, err := repo.Get(context.Background(), "req_missing"); err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows got %v", err)
	}
}

func TestListFiltersByStatusAndProduct(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p1 := mustProduct(t, db, "Lamp")
	p2 := mustProduct(t, db, "Bottle")
	repo := request.NewRepository(db)

	mustRequest(t, repo, request.RequestInput{ProductID: p1.ID, Usage: "listing"})
	second := mustRequest(t, repo, request.RequestInput{ProductID: p1.ID, Usage: "social"})
	mustRequest(t, repo, request.RequestInput{ProductID: p2.ID, Usage: "ad"})
	if _, err := repo.UpdateStatus(ctx, second.ID, request.StatusGenerating); err != nil {
		t.Fatalf("update status: %v", err)
	}

	all, total, err := repo.List(ctx, request.ListFilter{})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if total != 3 || len(all) != 3 {
		t.Fatalf("expected 3 requests got total=%d len=%d", total, len(all))
	}

	byProduct, total, err := repo.List(ctx, request.ListFilter{ProductID: p1.ID})
	if err != nil {
		t.Fatalf("list by product: %v", err)
	}
	if total != 2 || len(byProduct) != 2 {
		t.Fatalf("expected 2 requests for p1 got total=%d len=%d", total, len(byProduct))
	}
	for _, req := range byProduct {
		if req.ProductID != p1.ID {
			t.Fatalf("unexpected product_id %q", req.ProductID)
		}
	}

	byStatus, total, err := repo.List(ctx, request.ListFilter{Status: request.StatusGenerating})
	if err != nil {
		t.Fatalf("list by status: %v", err)
	}
	if total != 1 || len(byStatus) != 1 || byStatus[0].ID != second.ID {
		t.Fatalf("expected the generating request got total=%d items=%#v", total, byStatus)
	}

	combined, total, err := repo.List(ctx, request.ListFilter{
		ProductID: p1.ID,
		Status:    request.StatusSubmitted,
	})
	if err != nil {
		t.Fatalf("list combined: %v", err)
	}
	if total != 1 || len(combined) != 1 {
		t.Fatalf("expected 1 submitted request for p1 got total=%d len=%d", total, len(combined))
	}
}

func TestListPagination(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db, "Lamp")
	repo := request.NewRepository(db)
	for i := 0; i < 3; i++ {
		mustRequest(t, repo, request.RequestInput{ProductID: p.ID})
	}

	page1, total, err := repo.List(ctx, request.ListFilter{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("list page 1: %v", err)
	}
	if total != 3 || len(page1) != 2 {
		t.Fatalf("expected page 1 with 2 of 3 got total=%d len=%d", total, len(page1))
	}
	page2, total, err := repo.List(ctx, request.ListFilter{Page: 2, PageSize: 2})
	if err != nil {
		t.Fatalf("list page 2: %v", err)
	}
	if total != 3 || len(page2) != 1 {
		t.Fatalf("expected page 2 with 1 of 3 got total=%d len=%d", total, len(page2))
	}
	if page1[0].ID == page2[0].ID || page1[1].ID == page2[0].ID {
		t.Fatalf("expected distinct pages")
	}
}

func TestCanTransition(t *testing.T) {
	cases := []struct {
		from, to string
		ok       bool
	}{
		{request.StatusSubmitted, request.StatusGenerating, true},
		{request.StatusGenerating, request.StatusGenerated, true},
		{request.StatusGenerated, request.StatusApproved, true},
		{request.StatusGenerated, request.StatusRejected, true},
		{request.StatusApproved, request.StatusDelivered, true},
		{request.StatusRejected, request.StatusGenerating, true},
		{request.StatusSubmitted, request.StatusApproved, false},
		{request.StatusSubmitted, request.StatusDelivered, false},
		{request.StatusApproved, request.StatusSubmitted, false},
		{request.StatusDelivered, request.StatusGenerating, false},
	}
	for _, c := range cases {
		if got := request.CanTransition(c.from, c.to); got != c.ok {
			t.Fatalf("CanTransition(%q, %q) = %v, want %v", c.from, c.to, got, c.ok)
		}
	}
}

func TestUpdateStatusWalksLifecycle(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db, "Lamp")
	repo := request.NewRepository(db)
	req := mustRequest(t, repo, request.RequestInput{ProductID: p.ID})

	steps := []string{
		request.StatusGenerating,
		request.StatusGenerated,
		request.StatusApproved,
		request.StatusDelivered,
	}
	for _, next := range steps {
		updated, err := repo.UpdateStatus(ctx, req.ID, next)
		if err != nil {
			t.Fatalf("transition to %q: %v", next, err)
		}
		if updated.Status != next {
			t.Fatalf("expected status %q got %q", next, updated.Status)
		}
	}
	if updated, err := repo.Get(ctx, req.ID); err != nil || updated.UpdatedAt < req.UpdatedAt {
		t.Fatalf("expected updated_at to move forward, before=%q after=%q err=%v",
			req.UpdatedAt, updated.UpdatedAt, err)
	}
}

func TestUpdateStatusRejectedCanRegenerate(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db, "Lamp")
	repo := request.NewRepository(db)
	req := mustRequest(t, repo, request.RequestInput{ProductID: p.ID})

	for _, next := range []string{request.StatusGenerating, request.StatusGenerated, request.StatusRejected} {
		if _, err := repo.UpdateStatus(ctx, req.ID, next); err != nil {
			t.Fatalf("transition to %q: %v", next, err)
		}
	}
	updated, err := repo.UpdateStatus(ctx, req.ID, request.StatusGenerating)
	if err != nil {
		t.Fatalf("rejected -> generating: %v", err)
	}
	if updated.Status != request.StatusGenerating {
		t.Fatalf("expected status generating got %q", updated.Status)
	}
}

func TestUpdateStatusRejectsInvalidTransition(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db, "Lamp")
	repo := request.NewRepository(db)
	req := mustRequest(t, repo, request.RequestInput{ProductID: p.ID})

	_, err := repo.UpdateStatus(ctx, req.ID, request.StatusApproved)
	if err == nil {
		t.Fatalf("expected error for submitted -> approved")
	}
	if !strings.Contains(err.Error(), "invalid status transition") {
		t.Fatalf("expected descriptive transition error got %v", err)
	}
	got, getErr := repo.Get(ctx, req.ID)
	if getErr != nil {
		t.Fatalf("get: %v", getErr)
	}
	if got.Status != request.StatusSubmitted {
		t.Fatalf("expected status to stay submitted got %q", got.Status)
	}
}
