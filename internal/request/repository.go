package request

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"tiktok-trend-shop/internal/id"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DB() *sql.DB { return r.db }

// Create inserts a new material request in the submitted state.
func (r *Repository) Create(ctx context.Context, input RequestInput) (MaterialRequest, error) {
	if strings.TrimSpace(input.ProductID) == "" {
		return MaterialRequest{}, fmt.Errorf("product_id is required")
	}
	now := utcNow()
	requestID := id.New("req")
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO material_requests(
			id, product_id, client_id, usage, style, focus, notes, status, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, requestID, input.ProductID, input.ClientID, input.Usage, input.Style, input.Focus,
		input.Notes, StatusSubmitted, now, now)
	if err != nil {
		return MaterialRequest{}, err
	}
	return r.Get(ctx, requestID)
}

func (r *Repository) Get(ctx context.Context, requestID string) (MaterialRequest, error) {
	row := r.db.QueryRowContext(ctx, requestSelectColumns+` WHERE id = ?`, requestID)
	return scanRequest(row)
}

// ListFilter narrows the result set of List.
type ListFilter struct {
	Status    string
	ProductID string
	ClientID  string
	Page      int
	PageSize  int
}

// List returns the filtered page of requests plus the total count across all
// pages.
func (r *Repository) List(ctx context.Context, filter ListFilter) ([]MaterialRequest, int, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	where := []string{"1=1"}
	args := []any{}
	if filter.Status != "" {
		where = append(where, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.ProductID != "" {
		where = append(where, "product_id = ?")
		args = append(args, filter.ProductID)
	}
	if filter.ClientID != "" {
		where = append(where, "client_id = ?")
		args = append(args, filter.ClientID)
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM material_requests WHERE `+whereSQL, args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	pageArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := r.db.QueryContext(ctx, requestSelectColumns+`
		WHERE `+whereSQL+`
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	results := []MaterialRequest{}
	for rows.Next() {
		req, err := scanRequest(rows)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, req)
	}
	return results, total, rows.Err()
}

// UpdateStatus moves a request to the next status, rejecting illegal
// transitions with a descriptive error.
func (r *Repository) UpdateStatus(ctx context.Context, requestID, next string) (MaterialRequest, error) {
	req, err := r.Get(ctx, requestID)
	if err != nil {
		return MaterialRequest{}, err
	}
	if !CanTransition(req.Status, next) {
		return MaterialRequest{}, fmt.Errorf(
			"invalid status transition from %q to %q", req.Status, next)
	}
	if _, err := r.db.ExecContext(ctx, `
		UPDATE material_requests SET status = ?, updated_at = ?
		WHERE id = ?
	`, next, utcNow(), requestID); err != nil {
		return MaterialRequest{}, err
	}
	return r.Get(ctx, requestID)
}

const requestSelectColumns = `
	SELECT id, product_id, client_id, usage, style, focus, notes, status, created_at, updated_at
	FROM material_requests
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRequest(row rowScanner) (MaterialRequest, error) {
	var req MaterialRequest
	if err := row.Scan(
		&req.ID,
		&req.ProductID,
		&req.ClientID,
		&req.Usage,
		&req.Style,
		&req.Focus,
		&req.Notes,
		&req.Status,
		&req.CreatedAt,
		&req.UpdatedAt,
	); err != nil {
		return MaterialRequest{}, err
	}
	return req, nil
}

func utcNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}
