package mediagen

import (
	"context"
	"database/sql"

	"tiktok-trend-shop/internal/id"
)

// Repository persists image generation tasks.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// DB exposes the underlying handle.
func (r *Repository) DB() *sql.DB { return r.db }

// CreateTask inserts a queued task and returns the persisted row.
func (r *Repository) CreateTask(ctx context.Context, task ImageGenTask) (ImageGenTask, error) {
	now := utcNow()
	taskID := id.New("imgtask")
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO image_gen_tasks(
			id, product_id, status, usage, style, quality, prompt,
			seed, width, height, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, taskID, task.ProductID, StatusQueued, task.Usage, task.Style, task.Quality,
		task.Prompt, task.Seed, task.Width, task.Height, now, now)
	if err != nil {
		return ImageGenTask{}, err
	}
	return r.GetTask(ctx, taskID)
}

// GetTask returns one task by id, or sql.ErrNoRows when it does not exist.
func (r *Repository) GetTask(ctx context.Context, taskID string) (ImageGenTask, error) {
	row := r.db.QueryRowContext(ctx, taskSelectColumns+` WHERE id = ?`, taskID)
	return scanTask(row)
}

// MarkRunning moves a task to running and records the provider prompt id.
func (r *Repository) MarkRunning(ctx context.Context, taskID, comfyPromptID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE image_gen_tasks SET status = ?, comfy_prompt_id = ?, updated_at = ?
		WHERE id = ?
	`, StatusRunning, comfyPromptID, utcNow(), taskID)
	return err
}

// MarkSucceeded completes a task with the id of the first produced asset.
func (r *Repository) MarkSucceeded(ctx context.Context, taskID, assetID string) error {
	now := utcNow()
	_, err := r.db.ExecContext(ctx, `
		UPDATE image_gen_tasks SET status = ?, asset_id = ?, updated_at = ?, completed_at = ?
		WHERE id = ?
	`, StatusSucceeded, assetID, now, now, taskID)
	return err
}

// MarkFailed completes a task with the failure cause.
func (r *Repository) MarkFailed(ctx context.Context, taskID string, cause error) error {
	now := utcNow()
	_, err := r.db.ExecContext(ctx, `
		UPDATE image_gen_tasks SET status = ?, last_error = ?, updated_at = ?, completed_at = ?
		WHERE id = ?
	`, StatusFailed, cause.Error(), now, now, taskID)
	return err
}

// ListTasksByProduct returns the newest tasks of a product capped by limit.
func (r *Repository) ListTasksByProduct(ctx context.Context, productID string, limit int) ([]ImageGenTask, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, taskSelectColumns+`
		WHERE product_id = ?
		ORDER BY created_at DESC, rowid DESC
		LIMIT ?
	`, productID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []ImageGenTask{}
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, task)
	}
	return results, rows.Err()
}

const taskSelectColumns = `
	SELECT id, product_id, status, usage, style, quality, prompt,
		seed, width, height, comfy_prompt_id, asset_id, last_error,
		created_at, updated_at, completed_at
	FROM image_gen_tasks
`

func scanTask(row rowScanner) (ImageGenTask, error) {
	var (
		task          ImageGenTask
		comfyPromptID sql.NullString
		assetID       sql.NullString
		lastError     sql.NullString
		completedAt   sql.NullString
	)
	if err := row.Scan(
		&task.ID,
		&task.ProductID,
		&task.Status,
		&task.Usage,
		&task.Style,
		&task.Quality,
		&task.Prompt,
		&task.Seed,
		&task.Width,
		&task.Height,
		&comfyPromptID,
		&assetID,
		&lastError,
		&task.CreatedAt,
		&task.UpdatedAt,
		&completedAt,
	); err != nil {
		return ImageGenTask{}, err
	}
	task.ComfyPromptID = nullStringPtr(comfyPromptID)
	task.AssetID = nullStringPtr(assetID)
	task.LastError = nullStringPtr(lastError)
	task.CompletedAt = nullStringPtr(completedAt)
	return task, nil
}
