package dossier

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

// Add inserts a new asset after validating the per-kind required fields.
func (r *Repository) Add(ctx context.Context, input AssetInput) (Asset, error) {
	if strings.TrimSpace(input.ProductID) == "" {
		return Asset{}, fmt.Errorf("product_id is required")
	}
	if _, message, ok := Validate(input); !ok {
		return Asset{}, fmt.Errorf("%s", message)
	}
	now := utcNow()
	assetID := id.New("da")
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO dossier_assets(
			id, product_id, kind, url, content, source, note, created_by, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, assetID, input.ProductID, strings.TrimSpace(input.Kind), strings.TrimSpace(input.URL),
		strings.TrimSpace(input.Content), strings.TrimSpace(input.Source),
		strings.TrimSpace(input.Note), strings.TrimSpace(input.CreatedBy), now)
	if err != nil {
		return Asset{}, err
	}
	return r.Get(ctx, assetID)
}

func (r *Repository) Get(ctx context.Context, assetID string) (Asset, error) {
	row := r.db.QueryRowContext(ctx, assetSelectColumns+` WHERE id = ?`, assetID)
	return scanAsset(row)
}

// List returns the assets of a product in chronological order, optionally
// filtered by kind (empty kind returns all kinds).
func (r *Repository) List(ctx context.Context, productID, kind string) ([]Asset, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if strings.TrimSpace(kind) != "" {
		rows, err = r.db.QueryContext(ctx, assetSelectColumns+`
			WHERE product_id = ? AND kind = ?
			ORDER BY created_at ASC, id ASC
		`, productID, strings.TrimSpace(kind))
	} else {
		rows, err = r.db.QueryContext(ctx, assetSelectColumns+`
			WHERE product_id = ?
			ORDER BY created_at ASC, id ASC
		`, productID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []Asset{}
	for rows.Next() {
		asset, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, asset)
	}
	return results, rows.Err()
}

// Delete removes an asset of the given product. An asset that does not exist
// or belongs to another product yields sql.ErrNoRows.
func (r *Repository) Delete(ctx context.Context, productID, assetID string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM dossier_assets WHERE id = ? AND product_id = ?
	`, assetID, productID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

const assetSelectColumns = `
	SELECT id, product_id, kind, url, content, source, note, created_by, created_at
	FROM dossier_assets
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAsset(row rowScanner) (Asset, error) {
	var asset Asset
	if err := row.Scan(
		&asset.ID,
		&asset.ProductID,
		&asset.Kind,
		&asset.URL,
		&asset.Content,
		&asset.Source,
		&asset.Note,
		&asset.CreatedBy,
		&asset.CreatedAt,
	); err != nil {
		return Asset{}, err
	}
	return asset, nil
}

func utcNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}
