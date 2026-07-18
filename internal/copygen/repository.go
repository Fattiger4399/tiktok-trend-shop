package copygen

import (
	"context"
	"database/sql"
	"encoding/json"
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

// SaveVariants persists a batch of drafts for a request, numbering
// variant_no from 1 upwards, and returns the saved rows.
func (r *Repository) SaveVariants(ctx context.Context, requestID string, drafts []VariantDraft, provider, model string) ([]CopyVariant, error) {
	if len(drafts) == 0 {
		return []CopyVariant{}, nil
	}
	now := utcNow()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for i, draft := range drafts {
		hashtags, err := marshalHashtags(draft.Hashtags)
		if err != nil {
			return nil, err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO copy_variants(
				id, request_id, variant_no, hook, body, caption, hashtags,
				provider, model, prompt_version, created_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, id.New("cv"), requestID, i+1, draft.Hook, draft.Body, draft.Caption,
			hashtags, provider, model, PromptVersion, now)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ListByRequest(ctx, requestID)
}

// Get returns one variant by id, or sql.ErrNoRows when it does not exist.
func (r *Repository) Get(ctx context.Context, variantID string) (CopyVariant, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, request_id, variant_no, hook, body, caption, hashtags,
			provider, model, prompt_version, created_at
		FROM copy_variants
		WHERE id = ?
	`, variantID)
	return scanVariant(row)
}

// ListByRequest returns all variants of a request ordered by variant number.
func (r *Repository) ListByRequest(ctx context.Context, requestID string) ([]CopyVariant, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, request_id, variant_no, hook, body, caption, hashtags,
			provider, model, prompt_version, created_at
		FROM copy_variants
		WHERE request_id = ?
		ORDER BY variant_no ASC, id ASC
	`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []CopyVariant{}
	for rows.Next() {
		variant, err := scanVariant(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, variant)
	}
	return results, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanVariant(row rowScanner) (CopyVariant, error) {
	var (
		variant  CopyVariant
		hashtags string
	)
	if err := row.Scan(
		&variant.ID,
		&variant.RequestID,
		&variant.VariantNo,
		&variant.Hook,
		&variant.Body,
		&variant.Caption,
		&hashtags,
		&variant.Provider,
		&variant.Model,
		&variant.PromptVersion,
		&variant.CreatedAt,
	); err != nil {
		return CopyVariant{}, err
	}
	variant.Hashtags = unmarshalHashtags(hashtags)
	return variant, nil
}

func marshalHashtags(tags []string) (string, error) {
	if tags == nil {
		tags = []string{}
	}
	data, err := json.Marshal(tags)
	return string(data), err
}

func unmarshalHashtags(value string) []string {
	if value == "" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		return []string{}
	}
	return out
}

func utcNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}
