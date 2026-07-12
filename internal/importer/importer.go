// Package importer implements the shared CSV import workflow used by the
// CLI and the web workbench.
//
// The importer is bounded by MaxBytes and MaxRows, supports idempotency keys
// for retry safety, deduplicates Amazon rows by marketplace+ASIN and other
// sources by provider+source_id, and persists row-level outcomes in
// import_job_rows so the UI can show exact reasons for any rejected rows.
//
// Unsafe values such as spreadsheet formulas and HTML markup are escaped in
// the stored raw_json to keep them harmless if later rendered as text.
package importer

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"tiktok-trend-shop/internal/id"
	"tiktok-trend-shop/internal/product"
)

// MaxBytes is the default upper bound for uploaded CSV files.
const MaxBytes int64 = 10 * 1024 * 1024

// MaxRows is the default upper bound for parsed CSV rows.
const MaxRows = 5000

// Options controls import behavior.
type Options struct {
	Source         string
	Filename       string
	IdempotencyKey string
	MaxBytes       int64
	MaxRows        int
}

// Result is the summary returned to the API and CLI.
type Result struct {
	JobID              string   `json:"job_id"`
	Source             string   `json:"source"`
	Filename           string   `json:"filename"`
	TotalRows          int      `json:"total_rows"`
	ImportedRows       int      `json:"imported_rows"`
	UpdatedRows        int      `json:"updated_rows"`
	DuplicateRows      int      `json:"duplicate_rows"`
	RejectedRows       int      `json:"rejected_rows"`
	Status             string   `json:"status"`
	StartedAt          string   `json:"started_at"`
	CompletedAt        string   `json:"completed_at,omitempty"`
	ProductIDs         []string `json:"product_ids"`
	UpdatedProductIDs  []string `json:"updated_product_ids"`
	DuplicateProductIDs []string `json:"duplicate_product_ids"`
	IdempotencyKey     string   `json:"idempotency_key,omitempty"`
}

// RowOutcome describes a single row's import outcome.
type RowOutcome struct {
	RowNumber  int                    `json:"row_number"`
	Status     string                 `json:"status"`
	ProductID  string                 `json:"product_id,omitempty"`
	Reason     string                 `json:"reason,omitempty"`
	Field      string                 `json:"field,omitempty"`
	Raw        map[string]string      `json:"raw,omitempty"`
}

// Importer centralizes CSV ingestion for both the CLI and HTTP endpoints.
type Importer struct {
	db  *sql.DB
	pr  *product.Repository
}

// New returns a new Importer that uses the given repository.
func New(db *sql.DB, pr *product.Repository) *Importer {
	return &Importer{db: db, pr: pr}
}

// ImportFromPath imports a CSV file from the filesystem. This is used by the
// CLI command.
func (i *Importer) ImportFromPath(ctx context.Context, path, defaultRegion, source string) (Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return Result{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return Result{}, err
	}
	opts := Options{
		Source:   source,
		Filename: filepath.Base(path),
		MaxBytes: info.Size(),
		MaxRows:  MaxRows,
	}
	return i.Import(ctx, f, opts, defaultRegion)
}

// Import reads a CSV from r and runs the import job. The idempotency key is
// optional; when present and matching a previously completed job, that job's
// result is returned without writing again.
func (i *Importer) Import(ctx context.Context, r io.Reader, opts Options, defaultRegion string) (Result, error) {
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = MaxBytes
	}
	if opts.MaxRows <= 0 {
		opts.MaxRows = MaxRows
	}
	if opts.Source == "" {
		opts.Source = "manual-csv"
	}
	if opts.IdempotencyKey != "" {
		if jobID, result, ok, err := i.findIdempotent(ctx, opts.IdempotencyKey); err != nil {
			return Result{}, err
		} else if ok {
			result.IdempotencyKey = opts.IdempotencyKey
			result.JobID = jobID
			return result, nil
		}
	}

	// Pre-compute the idempotency hash when not provided.
	if opts.IdempotencyKey == "" {
		opts.IdempotencyKey = i.computeIdempotency(opts.Filename)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	jobID := id.New("job")
	if _, err := i.db.ExecContext(ctx, `
		INSERT INTO import_jobs(
			id, source, filename, status, total_rows, imported_rows, updated_rows,
			duplicate_rows, rejected_rows, idempotency_key, started_at, created_at, updated_at
		)
		VALUES (?, ?, ?, 'running', 0, 0, 0, 0, 0, ?, ?, ?, ?)
	`, jobID, opts.Source, opts.Filename, opts.IdempotencyKey, now, now, now); err != nil {
		return Result{}, err
	}

	reader := csv.NewReader(io.LimitReader(r, opts.MaxBytes+1))
	reader.FieldsPerRecord = -1
	headers, err := reader.Read()
	if err != nil {
		_, _ = i.db.ExecContext(ctx, `UPDATE import_jobs SET status='failed', completed_at=?, summary_json=? WHERE id=?`,
			now, errorSummary("invalid_headers", err.Error()), jobID)
		return Result{}, fmt.Errorf("read headers: %w", err)
	}
	if len(headers) > 0 {
		headers[0] = strings.TrimPrefix(headers[0], "\ufeff")
	}

	if err := requireHeaders(headers); err != nil {
		_, _ = i.db.ExecContext(ctx, `UPDATE import_jobs SET status='failed', completed_at=?, summary_json=? WHERE id=?`,
			now, errorSummary("missing_required_headers", err.Error()), jobID)
		return Result{}, err
	}

	result := Result{
		JobID:          jobID,
		Source:         opts.Source,
		Filename:       opts.Filename,
		Status:         "running",
		StartedAt:      now,
		IdempotencyKey: opts.IdempotencyKey,
	}
	outcomes := make([]RowOutcome, 0, 16)

	rowNumber := 0
	for {
		rowNumber++
		if rowNumber > opts.MaxRows {
			outcomes = append(outcomes, RowOutcome{
				RowNumber: rowNumber,
				Status:    "rejected",
				Reason:    "row_limit_exceeded",
				Field:     "row",
			})
			break
		}
		values, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			outcomes = append(outcomes, RowOutcome{
				RowNumber: rowNumber,
				Status:    "rejected",
				Reason:    "parse_error: " + err.Error(),
				Field:     "row",
			})
			result.RejectedRows++
			continue
		}
		row := MapRow(headers, values)
		outcome, err := i.importRow(ctx, rowNumber, row, defaultRegion)
		if err != nil {
			outcomes = append(outcomes, RowOutcome{
				RowNumber: rowNumber,
				Status:    "rejected",
				Reason:    err.Error(),
				Field:     "row",
				Raw:       sanitizeRaw(row),
			})
			result.RejectedRows++
			continue
		}
		outcomes = append(outcomes, outcome)
		switch outcome.Status {
		case "imported":
			result.ImportedRows++
			result.ProductIDs = append(result.ProductIDs, outcome.ProductID)
		case "updated":
			result.UpdatedRows++
			result.UpdatedProductIDs = append(result.UpdatedProductIDs, outcome.ProductID)
		case "duplicate":
			result.DuplicateRows++
			result.DuplicateProductIDs = append(result.DuplicateProductIDs, outcome.ProductID)
		}
	}
	result.TotalRows = rowNumber - 1
	if result.RejectedRows == 0 {
		result.Status = "completed"
	} else if result.ImportedRows > 0 || result.UpdatedRows > 0 {
		result.Status = "partially_completed"
	} else {
		result.Status = "failed"
	}
	result.CompletedAt = time.Now().UTC().Format(time.RFC3339)

	if err := i.persistOutcomes(ctx, jobID, outcomes); err != nil {
		return result, err
	}
	if err := i.persistSummary(ctx, jobID, result); err != nil {
		return result, err
	}
	return result, nil
}

// JobRecord describes an import job row.
type JobRecord struct {
	ID              string  `json:"id"`
	Source          string  `json:"source"`
	Filename        *string `json:"filename,omitempty"`
	Status          string  `json:"status"`
	TotalRows       int     `json:"total_rows"`
	ImportedRows    int     `json:"imported_rows"`
	UpdatedRows     int     `json:"updated_rows"`
	DuplicateRows   int     `json:"duplicate_rows"`
	RejectedRows    int     `json:"rejected_rows"`
	StartedAt       string  `json:"started_at"`
	CompletedAt     *string `json:"completed_at,omitempty"`
	IdempotencyKey  *string `json:"idempotency_key,omitempty"`
}

// ListJobs returns recent import jobs.
func (i *Importer) ListJobs(ctx context.Context, limit int) ([]JobRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := i.db.QueryContext(ctx, `
		SELECT id, source, filename, status, total_rows, imported_rows, updated_rows,
			duplicate_rows, rejected_rows, started_at, completed_at, idempotency_key
		FROM import_jobs
		ORDER BY created_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []JobRecord
	for rows.Next() {
		var j JobRecord
		var filename, completed, key sql.NullString
		if err := rows.Scan(&j.ID, &j.Source, &filename, &j.Status, &j.TotalRows, &j.ImportedRows,
			&j.UpdatedRows, &j.DuplicateRows, &j.RejectedRows, &j.StartedAt, &completed, &key); err != nil {
			return nil, err
		}
		j.Filename = nullStringPtr(filename)
		j.CompletedAt = nullStringPtr(completed)
		j.IdempotencyKey = nullStringPtr(key)
		out = append(out, j)
	}
	return out, rows.Err()
}

// GetJob returns a single import job plus its row outcomes.
func (i *Importer) GetJob(ctx context.Context, jobID string) (JobRecord, []RowOutcome, error) {
	row := i.db.QueryRowContext(ctx, `
		SELECT id, source, filename, status, total_rows, imported_rows, updated_rows,
			duplicate_rows, rejected_rows, started_at, completed_at, idempotency_key
		FROM import_jobs WHERE id = ?
	`, jobID)
	var j JobRecord
	var filename, completed, key sql.NullString
	if err := row.Scan(&j.ID, &j.Source, &filename, &j.Status, &j.TotalRows, &j.ImportedRows,
		&j.UpdatedRows, &j.DuplicateRows, &j.RejectedRows, &j.StartedAt, &completed, &key); err != nil {
		return j, nil, err
	}
	j.Filename = nullStringPtr(filename)
	j.CompletedAt = nullStringPtr(completed)
	j.IdempotencyKey = nullStringPtr(key)
	rows, err := i.db.QueryContext(ctx, `
		SELECT row_number, status, COALESCE(product_id, ''), COALESCE(reason, ''), COALESCE(field, ''), raw_json
		FROM import_job_rows WHERE import_job_id = ? ORDER BY row_number ASC
	`, jobID)
	if err != nil {
		return j, nil, err
	}
	defer rows.Close()
	var outcomes []RowOutcome
	for rows.Next() {
		var o RowOutcome
		var raw string
		if err := rows.Scan(&o.RowNumber, &o.Status, &o.ProductID, &o.Reason, &o.Field, &raw); err != nil {
			return j, nil, err
		}
		if raw != "" {
			var m map[string]string
			if err := json.Unmarshal([]byte(raw), &m); err == nil {
				o.Raw = m
			}
		}
		outcomes = append(outcomes, o)
	}
	return j, outcomes, rows.Err()
}

// ImportRowResult is the per-row outcome returned by importRow.
type ImportRowResult struct {
	Outcome RowOutcome
	Product product.Product
}

func (i *Importer) importRow(ctx context.Context, rowNumber int, row map[string]string, defaultRegion string) (RowOutcome, error) {
	title := strings.TrimSpace(row["title"])
	if title == "" {
		if alt := strings.TrimSpace(row["product_name"]); alt != "" {
			title = alt
		}
	}
	if title == "" {
		return RowOutcome{}, errors.New("missing required field: title")
	}
	region := strings.TrimSpace(row["region"])
	if region == "" {
		region = defaultRegion
	}
	if region == "" {
		region = "CN"
	}
	marketplace := optional(row["marketplace"])
	asin := optional(row["asin"])
	sourceCategoryName := optional(row["category"])
	if marketplace == nil {
		m := optional(row["market"])
		if m != nil && *m != "" {
			marketplace = m
		}
	}
	canonicalURL := optional(row["canonical_url"])
	if canonicalURL == nil {
		canonicalURL = optional(row["product_url"])
	}
	sourceID := optional(row["source_id"])
	if sourceID == nil {
		sourceID = optional(row["item_id"])
	}
	if sourceID == nil {
		sourceID = optional(row["goods_id"])
	}
	if sourceID == nil {
		fallback := strings.ToLower(strings.ReplaceAll(title, " ", "-"))
		sourceID = &fallback
	}

	productInput := product.ProductInput{
		Title:            title,
		Region:           region,
		Marketplace:      marketplace,
		ASIN:             asin,
		SourceCategory:   sourceCategoryName,
		SourceCategoryID: sourceID,
		CanonicalURL:     canonicalURL,
		Provider:         "manual",
		SourceID:         sourceID,
		Metadata: map[string]any{
			"csv_source": true,
			"csv_row":    sanitizeRaw(row),
		},
	}

	existing, existed, err := i.findExisting(ctx, productInput)
	if err != nil {
		return RowOutcome{}, err
	}
	status := "imported"
	if existed {
		status = "updated"
	}
	rec, err := i.pr.UpsertProduct(ctx, productInput)
	if err != nil {
		return RowOutcome{}, err
	}
	if existed && existing.ID != "" {
		rec.ID = existing.ID
	}
	if marketplace != nil && asin != nil && *marketplace != "" && *asin != "" {
		if err := i.pr.UpdateAmazonIdentity(ctx, rec.ID, *marketplace, *asin, ""); err != nil {
			return RowOutcome{}, err
		}
	}
	if _, err := i.pr.AddSourceSnapshot(ctx, product.SnapshotInput{
		ProductID:  rec.ID,
		Provider:   "manual",
		SourceType: "product",
		SourceID:   sourceID,
		SourceURL:  canonicalURL,
		Metrics: map[string]float64{
			"views":            parseMetric(row["views"]),
			"sales":            parseMetric(row["sales"]),
			"engagement":       parseMetric(row["engagement"]),
			"growth_rate":      parsePercent(row["growth_rate"]),
			"commission_rate":  parsePercent(row["commission_rate"]),
			"margin":           parseMetric(row["margin"]),
			"competitor_count": parseMetric(row["competitor_count"]),
			"reviews":          parseMetric(row["reviews"]),
		},
		MetricKind: "observed",
		Metadata:   map[string]any{"import": true},
	}); err != nil {
		return RowOutcome{}, err
	}
	price := parsePrice(row["price"])
	detailInput := product.DetailInput{
		ProductID:        rec.ID,
		Provider:         "manual-csv",
		Platform:         optional(row["platform"]),
		SourceID:         sourceID,
		SourceURL:        canonicalURL,
		ProductURL:       canonicalURL,
		Title:            &title,
		ShopName:         optional(row["shop_name"]),
		Brand:            optional(row["brand"]),
		Price:            price,
		Currency:         optionalOr(row["currency"], "CNY"),
		ImageURL:         optional(row["image_url"]),
		SourceCategory:   sourceCategoryName,
		SellingPoints:    parseList(row["selling_points"]),
		Specs:            parseSpecs(row["specs"]),
		ReviewSummary:    optional(row["review_summary"]),
		ReviewHighlights: parseList(row["review_highlights"]),
		Warnings:         parseList(row["warnings"]),
		Raw:              rowToAny(sanitizeRaw(row)),
		Metadata:         map[string]any{"csv_detail": true},
	}
	if _, err := i.pr.AddDetailSnapshot(ctx, detailInput); err != nil {
		return RowOutcome{}, err
	}
	return RowOutcome{
		RowNumber: rowNumber,
		Status:    status,
		ProductID: rec.ID,
		Raw:       sanitizeRaw(row),
	}, nil
}

func (i *Importer) findExisting(ctx context.Context, in product.ProductInput) (product.Product, bool, error) {
	if in.Marketplace != nil && in.ASIN != nil && *in.Marketplace != "" && *in.ASIN != "" {
		if p, err := i.pr.DB().QueryContext(ctx,
			`SELECT id FROM products WHERE marketplace = ? AND asin = ? LIMIT 1`,
			*in.Marketplace, *in.ASIN); err == nil {
			defer p.Close()
			if p.Next() {
				var id string
				_ = p.Scan(&id)
				existing, err := i.pr.GetProduct(ctx, id)
				if err == nil {
					return existing, true, nil
				}
			}
		}
	}
	if in.Provider != "" && in.SourceID != nil && *in.SourceID != "" {
		if p, err := i.pr.DB().QueryContext(ctx,
			`SELECT product_id FROM source_snapshots WHERE provider = ? AND source_id = ? ORDER BY created_at DESC LIMIT 1`,
			in.Provider, *in.SourceID); err == nil {
			defer p.Close()
			if p.Next() {
				var id string
				_ = p.Scan(&id)
				existing, err := i.pr.GetProduct(ctx, id)
				if err == nil {
					return existing, true, nil
				}
			}
		}
	}
	return product.Product{}, false, nil
}

func (i *Importer) findIdempotent(ctx context.Context, key string) (string, Result, bool, error) {
	row := i.db.QueryRowContext(ctx, `
		SELECT id, source, COALESCE(filename, ''), status, total_rows, imported_rows, updated_rows,
			duplicate_rows, rejected_rows, started_at, COALESCE(completed_at, ''), COALESCE(summary_json, '{}')
		FROM import_jobs WHERE idempotency_key = ? AND status IN ('completed','partially_completed')
		ORDER BY created_at DESC LIMIT 1
	`, key)
	var (
		jobID, source, filename, status, started, completed, summary string
		total, imported, updated, duplicate, rejected int
	)
	if err := row.Scan(&jobID, &source, &filename, &status, &total, &imported, &updated, &duplicate,
		&rejected, &started, &completed, &summary); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", Result{}, false, nil
		}
		return "", Result{}, false, err
	}
	var summaryObj map[string]any
	_ = json.Unmarshal([]byte(summary), &summaryObj)
	res := Result{
		JobID:          jobID,
		Source:         source,
		Filename:       filename,
		TotalRows:      total,
		ImportedRows:   imported,
		UpdatedRows:    updated,
		DuplicateRows:  duplicate,
		RejectedRows:   rejected,
		Status:         status,
		StartedAt:      started,
		CompletedAt:    completed,
	}
	return jobID, res, true, nil
}

func (i *Importer) computeIdempotency(filename string) string {
	h := sha256.Sum256([]byte(time.Now().UTC().Format("2006-01-02") + ":" + filename))
	return hex.EncodeToString(h[:8])
}

func (i *Importer) persistOutcomes(ctx context.Context, jobID string, outcomes []RowOutcome) error {
	tx, err := i.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO import_job_rows(
			id, import_job_id, row_number, status, product_id, reason, field, raw_json, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, o := range outcomes {
		var productID any
		if o.ProductID != "" {
			productID = o.ProductID
		}
		var reason, field any
		if o.Reason != "" {
			reason = o.Reason
		}
		if o.Field != "" {
			field = o.Field
		}
		rawJSON := "{}"
		if o.Raw != nil {
			b, err := json.Marshal(o.Raw)
			if err == nil {
				rawJSON = string(b)
			}
		}
		if _, err := stmt.ExecContext(ctx, id.New("row"), jobID, o.RowNumber, o.Status, productID, reason, field, rawJSON, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (i *Importer) persistSummary(ctx context.Context, jobID string, result Result) error {
	summary := map[string]any{
		"imported_product_ids":  result.ProductIDs,
		"updated_product_ids":   result.UpdatedProductIDs,
		"duplicate_product_ids": result.DuplicateProductIDs,
	}
	summaryJSON, _ := json.Marshal(summary)
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := i.db.ExecContext(ctx, `
		UPDATE import_jobs
		SET status = ?, total_rows = ?, imported_rows = ?, updated_rows = ?,
			duplicate_rows = ?, rejected_rows = ?,
			completed_at = ?, summary_json = ?, errors_json = '{}', updated_at = ?
		WHERE id = ?
	`, result.Status, result.TotalRows, result.ImportedRows, result.UpdatedRows,
		result.DuplicateRows, result.RejectedRows, result.CompletedAt, string(summaryJSON), now, jobID)
	return err
}

func errorSummary(code, message string) string {
	b, _ := json.Marshal(map[string]string{"code": code, "message": message})
	return string(b)
}

// RequiredHeaders lists the minimum set of headers for an import to be
// considered valid.
var RequiredHeaders = []string{"title"}

// OptionalHeaders lists the optional but recommended headers.
var OptionalHeaders = []string{
	"marketplace", "asin", "category", "canonical_url", "product_url", "url",
	"platform", "shop_name", "brand", "price", "currency", "image_url",
	"selling_points", "specs", "review_summary", "review_highlights",
	"warnings", "views", "sales", "engagement", "growth_rate",
	"commission_rate", "margin", "competitor_count", "reviews",
}

func requireHeaders(headers []string) error {
	set := make(map[string]bool, len(headers))
	for _, h := range headers {
		set[strings.ToLower(strings.TrimSpace(h))] = true
	}
	for _, required := range RequiredHeaders {
		if !set[required] {
			return fmt.Errorf("missing required header: %s", required)
		}
	}
	return nil
}

// MapRow maps a CSV row to a normalized header->value map.
func MapRow(headers, values []string) map[string]string {
	row := make(map[string]string, len(headers))
	for idx, header := range headers {
		if idx >= len(values) {
			continue
		}
		row[normalizeKey(header)] = strings.TrimSpace(values[idx])
	}
	return row
}

func normalizeKey(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(strings.TrimSpace(value)), "-", "_"), " ", "_")
}

func optional(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func optionalOr(value, fallback string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return &fallback
	}
	return &value
}

func parseMetric(value string) float64 {
	text := cleanNumber(value)
	if text == "" || text == "-" {
		return 0
	}
	if strings.Contains(text, "~") {
		parts := strings.SplitN(text, "~", 2)
		return (parseMetric(parts[0]) + parseMetric(parts[1])) / 2
	}
	multiplier := 1.0
	text = strings.TrimSuffix(text, "+")
	if strings.HasSuffix(text, "w") || strings.HasSuffix(text, "万") {
		multiplier = 10000
		text = strings.TrimSuffix(strings.TrimSuffix(text, "w"), "万")
	}
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0
	}
	return parsed * multiplier
}

func parsePercent(value string) float64 {
	text := cleanNumber(value)
	if text == "" || text == "-" {
		return 0
	}
	if strings.Contains(text, "蝉选") || strings.Contains(text, "公开") {
		text = strings.ReplaceAll(strings.ReplaceAll(text, "蝉选", " "), "公开", " ")
		best := 0.0
		for _, part := range strings.Fields(text) {
			if value := parsePercent(part); value > best {
				best = value
			}
		}
		return best
	}
	if strings.Contains(text, "~") {
		parts := strings.SplitN(text, "~", 2)
		return (parsePercent(parts[0]) + parsePercent(parts[1])) / 2
	}
	text = strings.TrimSuffix(strings.TrimSuffix(text, "+"), "%")
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0
	}
	if parsed <= 1 {
		return parsed
	}
	return parsed / 100
}

func parsePrice(value string) *float64 {
	text := cleanNumber(value)
	text = strings.ReplaceAll(text, "¥", "")
	text = strings.ReplaceAll(text, "￥", "")
	text = strings.ReplaceAll(text, "元", "")
	text = strings.ReplaceAll(text, "cny", "")
	if text == "" {
		return nil
	}
	if strings.Contains(text, "~") {
		parts := strings.SplitN(text, "~", 2)
		left := parsePrice(parts[0])
		right := parsePrice(parts[1])
		if left != nil && right != nil {
			avg := (*left + *right) / 2
			return &avg
		}
	}
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return nil
	}
	return &parsed
}

func cleanNumber(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(value), ",", ""), "，", ""))
}

func parseList(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.ReplaceAll(value, "；", ";")
	for _, sep := range []string{";", "|", "、", "\n"} {
		value = strings.ReplaceAll(value, sep, ",")
	}
	var out []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func parseSpecs(value string) map[string]any {
	specs := map[string]any{}
	var loose []string
	for _, item := range parseList(value) {
		if key, val, ok := strings.Cut(item, ":"); ok {
			specs[strings.TrimSpace(key)] = strings.TrimSpace(val)
			continue
		}
		if key, val, ok := strings.Cut(item, "："); ok {
			specs[strings.TrimSpace(key)] = strings.TrimSpace(val)
			continue
		}
		loose = append(loose, item)
	}
	if len(loose) > 0 {
		specs["items"] = loose
	}
	return specs
}

// sanitizeRaw escapes spreadsheet formulas and HTML to keep uploaded content
// from being treated as code when later rendered in the UI.
func sanitizeRaw(row map[string]string) map[string]string {
	out := make(map[string]string, len(row))
	for k, v := range row {
		out[k] = neutralize(v)
	}
	return out
}

// neutralize removes dangerous spreadsheet prefixes and HTML angle brackets.
func neutralize(value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	for _, prefix := range []string{"=", "+", "-", "@"} {
		if strings.HasPrefix(lower, prefix) {
			if len(value) > 0 {
				value = "'" + value[1:]
			}
			break
		}
	}
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	return value
}

func nullStringPtr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

func rowToAny(row map[string]string) map[string]any {
	out := make(map[string]any, len(row))
	for k, v := range row {
		out[k] = v
	}
	return out
}