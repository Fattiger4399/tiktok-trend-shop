package product

import (
	"context"
	"database/sql"
	"encoding/json"
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

func (r *Repository) UpsertProduct(ctx context.Context, input ProductInput) (Product, error) {
	if strings.TrimSpace(input.Title) == "" {
		return Product{}, fmt.Errorf("product title is required")
	}
	if strings.TrimSpace(input.Region) == "" {
		return Product{}, fmt.Errorf("product region is required")
	}
	if input.Marketplace != nil && input.ASIN != nil && *input.Marketplace != "" && *input.ASIN != "" {
		if product, ok, err := r.findByMarketplaceASIN(ctx, *input.Marketplace, *input.ASIN); err != nil || ok {
			return product, err
		}
	}
	if input.Provider != "" && input.SourceID != nil && *input.SourceID != "" {
		if product, ok, err := r.findBySource(ctx, input.Provider, *input.SourceID); err != nil || ok {
			return product, err
		}
	}
	if input.CanonicalURL != nil && *input.CanonicalURL != "" {
		if product, ok, err := r.findByCanonicalURL(ctx, *input.CanonicalURL); err != nil || ok {
			return product, err
		}
	}
	if product, ok, err := r.findByTitleRegion(ctx, input.Title, input.Region); err != nil || ok {
		return product, err
	}
	now := utcNow()
	productID := id.New("prod")
	metadata, err := marshalObject(input.Metadata)
	if err != nil {
		return Product{}, err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO products(
			id, canonical_url, title, region, marketplace, asin,
			source_category, source_category_id, last_captured_at,
			category, workflow_state, metadata_json, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'candidate', ?, ?, ?)
	`, productID, stringValue(input.CanonicalURL), input.Title, input.Region,
		stringValue(input.Marketplace), stringValue(input.ASIN),
		stringValue(input.SourceCategory), stringValue(input.SourceCategoryID), now,
		stringValue(input.Category), metadata, now, now)
	if err != nil {
		return Product{}, err
	}
	return r.GetProduct(ctx, productID)
}

// UpdateAmazonIdentity sets marketplace/ASIN on an existing product and updates
// the last_captured_at timestamp.
func (r *Repository) UpdateAmazonIdentity(ctx context.Context, productID, marketplace, asin string, capturedAt string) error {
	if capturedAt == "" {
		capturedAt = utcNow()
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE products SET marketplace = ?, asin = ?, last_captured_at = ?, updated_at = ?
		WHERE id = ?
	`, nullableString(marketplace), nullableString(asin), capturedAt, capturedAt, productID)
	return err
}

// TouchProductCapturedAt updates last_captured_at when new observations arrive.
func (r *Repository) TouchProductCapturedAt(ctx context.Context, productID, capturedAt string) error {
	if capturedAt == "" {
		capturedAt = utcNow()
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE products SET last_captured_at = ?, updated_at = ?
		WHERE id = ?
	`, capturedAt, capturedAt, productID)
	return err
}

func (r *Repository) GetProduct(ctx context.Context, productID string) (Product, error) {
	row := r.db.QueryRowContext(ctx, productSelectColumns+` WHERE id = ?`, productID)
	return scanProduct(row)
}

func (r *Repository) AddSourceSnapshot(ctx context.Context, input SnapshotInput) (string, error) {
	if input.Provider == "" {
		input.Provider = "manual"
	}
	if input.SourceType == "" {
		input.SourceType = "product"
	}
	if input.MetricKind == "" {
		input.MetricKind = "observed"
	}
	now := utcNow()
	if input.Metrics == nil {
		input.Metrics = map[string]float64{}
	}
	snapshotID := id.New("snap")
	metrics, err := json.Marshal(input.Metrics)
	if err != nil {
		return "", err
	}
	metadata, err := marshalObject(input.Metadata)
	if err != nil {
		return "", err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO source_snapshots(
			id, product_id, provider, source_type, source_id, source_url,
			captured_at, metrics_json, metric_kind, raw_ref, metadata_json, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?)
	`, snapshotID, input.ProductID, input.Provider, input.SourceType, stringValue(input.SourceID),
		stringValue(input.SourceURL), now, string(metrics), input.MetricKind, metadata, now)
	if err != nil {
		return "", err
	}
	if err := r.TouchProductCapturedAt(ctx, input.ProductID, now); err != nil {
		return snapshotID, err
	}
	return snapshotID, nil
}

// SnapshotRecord exposes the data needed for metric history and trend views.
type SnapshotRecord struct {
	ID         string             `json:"id"`
	ProductID  string             `json:"product_id"`
	Provider   string             `json:"provider"`
	SourceType string             `json:"source_type"`
	SourceID   *string            `json:"source_id,omitempty"`
	SourceURL  *string            `json:"source_url,omitempty"`
	CapturedAt string             `json:"captured_at"`
	MetricKind string             `json:"metric_kind"`
	Metrics    map[string]float64 `json:"metrics"`
}

func (r *Repository) AddDetailSnapshot(ctx context.Context, input DetailInput) (string, error) {
	if input.Provider == "" {
		input.Provider = "manual-csv"
	}
	missing := missingDetailFields(input)
	status := "complete"
	if len(missing) > 0 {
		status = "incomplete"
	}
	now := utcNow()
	detailID := id.New("detail")
	specs, err := marshalObject(input.Specs)
	if err != nil {
		return "", err
	}
	sellingPoints, err := marshalItems(input.SellingPoints)
	if err != nil {
		return "", err
	}
	reviewHighlights, err := marshalItems(input.ReviewHighlights)
	if err != nil {
		return "", err
	}
	warnings, err := marshalItems(input.Warnings)
	if err != nil {
		return "", err
	}
	missingJSON, err := marshalItems(missing)
	if err != nil {
		return "", err
	}
	raw, err := marshalObject(input.Raw)
	if err != nil {
		return "", err
	}
	metadata, err := marshalObject(input.Metadata)
	if err != nil {
		return "", err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO product_detail_snapshots(
			id, product_id, provider, platform, source_id, source_url,
			product_url, title, shop_name, brand, price, currency, image_url,
			specs_json, selling_points_json, review_summary,
			review_highlights_json, warnings_json, missing_fields_json,
			completeness_status, raw_json, metadata_json, captured_at, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, detailID, input.ProductID, input.Provider, stringValue(input.Platform), stringValue(input.SourceID),
		stringValue(input.SourceURL), stringValue(input.ProductURL), stringValue(input.Title),
		stringValue(input.ShopName), stringValue(input.Brand), floatValue(input.Price),
		stringValue(input.Currency), stringValue(input.ImageURL), specs, sellingPoints, stringValue(input.ReviewSummary),
		reviewHighlights, warnings, missingJSON, status, raw, metadata, now, now)
	if err != nil {
		return "", err
	}
	if input.SourceCategory != nil && *input.SourceCategory != "" {
		_, _ = r.db.ExecContext(ctx, `
			UPDATE products SET source_category = COALESCE(source_category, ?), updated_at = ?
			WHERE id = ?
		`, *input.SourceCategory, now, input.ProductID)
	}
	if input.SourceID != nil && *input.SourceID != "" {
		_, _ = r.db.ExecContext(ctx, `
			UPDATE products SET source_category_id = COALESCE(source_category_id, ?), updated_at = ?
			WHERE id = ?
		`, *input.SourceID, now, input.ProductID)
	}
	return detailID, nil
}

// DetailSnapshot is the latest normalized detail projection.
type DetailSnapshot struct {
	ID               string         `json:"id"`
	ProductID        string         `json:"product_id"`
	ProductURL       *string        `json:"product_url"`
	Platform         *string        `json:"platform"`
	ShopName         *string        `json:"shop_name"`
	Brand            *string        `json:"brand"`
	Price            *float64       `json:"price"`
	Currency         *string        `json:"currency"`
	ImageURL         *string        `json:"image_url"`
	SellingPoints    []string       `json:"selling_points"`
	Specs            map[string]any `json:"specs"`
	ReviewSummary    *string        `json:"review_summary"`
	ReviewHighlights []string       `json:"review_highlights"`
	Completeness     string         `json:"completeness"`
	MissingFields    []string       `json:"missing_fields"`
	Metadata         map[string]any `json:"metadata"`
	CapturedAt       string         `json:"captured_at"`
}

// LatestDetail returns the most recent detail snapshot for a product.
func (r *Repository) LatestDetail(ctx context.Context, productID string) (DetailSnapshot, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, product_id, product_url, platform, shop_name, brand, price, currency, image_url,
			selling_points_json, specs_json, review_summary, review_highlights_json,
			completeness_status, missing_fields_json, metadata_json, captured_at
		FROM product_detail_snapshots
		WHERE product_id = ?
		ORDER BY captured_at DESC, created_at DESC
		LIMIT 1
	`, productID)
	var (
		detail          DetailSnapshot
		productURL      sql.NullString
		platform        sql.NullString
		shopName        sql.NullString
		brand           sql.NullString
		price           sql.NullFloat64
		currency        sql.NullString
		imageURL        sql.NullString
		sellingPoints   string
		specs           string
		reviewSummary   sql.NullString
		reviewHighlights string
		missingJSON     string
		metadataJSON    string
	)
	err := row.Scan(
		&detail.ID, &detail.ProductID, &productURL, &platform, &shopName, &brand, &price, &currency,
		&imageURL, &sellingPoints, &specs, &reviewSummary, &reviewHighlights,
		&detail.Completeness, &missingJSON, &metadataJSON, &detail.CapturedAt,
	)
	if err == sql.ErrNoRows {
		return DetailSnapshot{}, false, nil
	}
	if err != nil {
		return DetailSnapshot{}, false, err
	}
	detail.ProductURL = nullStringPtr(productURL)
	detail.Platform = nullStringPtr(platform)
	detail.ShopName = nullStringPtr(shopName)
	detail.Brand = nullStringPtr(brand)
	detail.Price = nullFloatPtr(price)
	detail.Currency = nullStringPtr(currency)
	detail.ImageURL = nullStringPtr(imageURL)
	detail.ReviewSummary = nullStringPtr(reviewSummary)
	detail.SellingPoints = unmarshalItems(sellingPoints)
	detail.Specs = unmarshalMap(specs)
	detail.ReviewHighlights = unmarshalItems(reviewHighlights)
	detail.MissingFields = unmarshalItems(missingJSON)
	detail.Metadata = unmarshalMap(metadataJSON)
	return detail, true, nil
}

// LatestSnapshot returns the latest source snapshot for a product, including provenance.
func (r *Repository) LatestSnapshot(ctx context.Context, productID string) (SnapshotRecord, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, product_id, provider, source_type, source_id, source_url,
			captured_at, metric_kind, metrics_json
		FROM source_snapshots
		WHERE product_id = ?
		ORDER BY captured_at DESC, created_at DESC
		LIMIT 1
	`, productID)
	var (
		rec        SnapshotRecord
		sourceID   sql.NullString
		sourceURL  sql.NullString
		metricsJSON string
	)
	err := row.Scan(
		&rec.ID, &rec.ProductID, &rec.Provider, &rec.SourceType, &sourceID, &sourceURL,
		&rec.CapturedAt, &rec.MetricKind, &metricsJSON,
	)
	if err == sql.ErrNoRows {
		return SnapshotRecord{}, false, nil
	}
	if err != nil {
		return SnapshotRecord{}, false, err
	}
	rec.SourceID = nullStringPtr(sourceID)
	rec.SourceURL = nullStringPtr(sourceURL)
	rec.Metrics = unmarshalFloatMap(metricsJSON)
	return rec, true, nil
}

// SnapshotHistory returns chronological observations for a product, optionally bounded by a time window.
func (r *Repository) SnapshotHistory(ctx context.Context, productID, window string, limit int) ([]SnapshotRecord, error) {
	if limit <= 0 {
		limit = 200
	}
	var since time.Time
	switch strings.ToLower(window) {
	case "", "all":
		since = time.Time{}
	case "24h":
		since = time.Now().Add(-24 * time.Hour)
	case "7d":
		since = time.Now().Add(-7 * 24 * time.Hour)
	case "30d":
		since = time.Now().Add(-30 * 24 * time.Hour)
	case "90d":
		since = time.Now().Add(-90 * 24 * time.Hour)
	default:
		since = time.Time{}
	}
	var (
		rows *sql.Rows
		err  error
	)
	if since.IsZero() {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, product_id, provider, source_type, source_id, source_url,
				captured_at, metric_kind, metrics_json
			FROM source_snapshots
			WHERE product_id = ?
			ORDER BY captured_at ASC, created_at ASC
			LIMIT ?
		`, productID, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, product_id, provider, source_type, source_id, source_url,
				captured_at, metric_kind, metrics_json
			FROM source_snapshots
			WHERE product_id = ? AND captured_at >= ?
			ORDER BY captured_at ASC, created_at ASC
			LIMIT ?
		`, productID, since.UTC().Format(time.RFC3339), limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SnapshotRecord
	for rows.Next() {
		var (
			rec         SnapshotRecord
			sourceID    sql.NullString
			sourceURL   sql.NullString
			metricsJSON string
		)
		if err := rows.Scan(
			&rec.ID, &rec.ProductID, &rec.Provider, &rec.SourceType, &sourceID, &sourceURL,
			&rec.CapturedAt, &rec.MetricKind, &metricsJSON,
		); err != nil {
			return nil, err
		}
		rec.SourceID = nullStringPtr(sourceID)
		rec.SourceURL = nullStringPtr(sourceURL)
		rec.Metrics = unmarshalFloatMap(metricsJSON)
		out = append(out, rec)
	}
	return out, rows.Err()
}

// ListMarketplaces returns the distinct marketplaces present in the catalog,
// falling back to region when a product has no marketplace set.
func (r *Repository) ListMarketplaces(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT COALESCE(NULLIF(marketplace, ''), region)
		FROM products
		WHERE COALESCE(NULLIF(marketplace, ''), region) != ''
		ORDER BY 1
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Repository) ListProducts(ctx context.Context, limit int) ([]ProductResult, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, canonical_url, title, region, marketplace, asin,
			source_category, source_category_id, category, workflow_state, metadata_json,
			last_captured_at
		FROM products
		ORDER BY created_at
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []ProductResult
	for rows.Next() {
		var result ProductResult
		var metadataJSON string
		var canonicalURL, marketplace, asin, sourceCategory, sourceCategoryID, category sql.NullString
		var lastCapturedAt sql.NullString
		if err := rows.Scan(
			&result.ID,
			&canonicalURL,
			&result.Title,
			&result.Region,
			&marketplace,
			&asin,
			&sourceCategory,
			&sourceCategoryID,
			&category,
			&result.WorkflowState,
			&metadataJSON,
			&lastCapturedAt,
		); err != nil {
			return nil, err
		}
		result.CanonicalURL = nullStringPtr(canonicalURL)
		result.Marketplace = nullStringPtr(marketplace)
		result.ASIN = nullStringPtr(asin)
		result.SourceCategory = nullStringPtr(sourceCategory)
		result.SourceCategoryID = nullStringPtr(sourceCategoryID)
		result.Category = nullStringPtr(category)
		result.Metadata = unmarshalMap(metadataJSON)
		if lastCapturedAt.Valid {
			ts := lastCapturedAt.String
			result.Freshness.LastCapturedAt = &ts
		}
		result.Freshness.ThresholdHours = DefaultStalenessHours
		result.Freshness.Stale = isStale(result.Freshness.LastCapturedAt, result.Freshness.ThresholdHours)
		result.Metrics = r.latestMetrics(ctx, result.ID)
		result.MetricKind, result.Estimated = r.latestMetricKind(ctx, result.ID)
		result.Provenance = r.latestProvenance(ctx, result.ID)
		r.attachLatestDetail(ctx, &result)
		results = append(results, result)
	}
	return results, rows.Err()
}

// findByMarketplaceASIN returns the existing product for a unique Amazon identity.
func (r *Repository) findByMarketplaceASIN(ctx context.Context, marketplace, asin string) (Product, bool, error) {
	row := r.db.QueryRowContext(ctx, productSelectColumns+`
		WHERE marketplace = ? AND asin = ?
		ORDER BY created_at LIMIT 1
	`, marketplace, asin)
	product, err := scanProduct(row)
	if err == sql.ErrNoRows {
		return Product{}, false, nil
	}
	return product, err == nil, err
}

func (r *Repository) findBySource(ctx context.Context, provider, sourceID string) (Product, bool, error) {
	row := r.db.QueryRowContext(ctx, productSelectColumns+`
		JOIN source_snapshots s ON s.product_id = p.id
		WHERE s.provider = ? AND s.source_id = ?
		ORDER BY s.created_at LIMIT 1
	`, provider, sourceID)
	product, err := scanProduct(row)
	if err == sql.ErrNoRows {
		return Product{}, false, nil
	}
	return product, err == nil, err
}

func (r *Repository) findByCanonicalURL(ctx context.Context, url string) (Product, bool, error) {
	row := r.db.QueryRowContext(ctx, productSelectColumns+`
		WHERE canonical_url = ? ORDER BY created_at LIMIT 1
	`, url)
	product, err := scanProduct(row)
	if err == sql.ErrNoRows {
		return Product{}, false, nil
	}
	return product, err == nil, err
}

func (r *Repository) findByTitleRegion(ctx context.Context, title, region string) (Product, bool, error) {
	row := r.db.QueryRowContext(ctx, productSelectColumns+`
		WHERE lower(title) = lower(?) AND region = ?
		ORDER BY created_at LIMIT 1
	`, strings.TrimSpace(title), region)
	product, err := scanProduct(row)
	if err == sql.ErrNoRows {
		return Product{}, false, nil
	}
	return product, err == nil, err
}

func (r *Repository) latestMetrics(ctx context.Context, productID string) map[string]float64 {
	row := r.db.QueryRowContext(ctx, `
		SELECT metrics_json FROM source_snapshots
		WHERE product_id = ?
		ORDER BY captured_at DESC, created_at DESC LIMIT 1
	`, productID)
	var metricsJSON string
	if err := row.Scan(&metricsJSON); err != nil {
		return map[string]float64{}
	}
	return unmarshalFloatMap(metricsJSON)
}

// LatestMetrics is the public form of latestMetrics.
func (r *Repository) LatestMetrics(ctx context.Context, productID string) map[string]float64 {
	return r.latestMetrics(ctx, productID)
}

func (r *Repository) latestMetricKind(ctx context.Context, productID string) (string, bool) {
	row := r.db.QueryRowContext(ctx, `
		SELECT metric_kind FROM source_snapshots
		WHERE product_id = ?
		ORDER BY captured_at DESC, created_at DESC LIMIT 1
	`, productID)
	var kind string
	if err := row.Scan(&kind); err != nil {
		return "observed", false
	}
	return kind, kind == "estimated"
}

// LatestMetricKind exposes the latest observation kind to external packages.
func (r *Repository) LatestMetricKind(ctx context.Context, productID string) (string, bool) {
	return r.latestMetricKind(ctx, productID)
}

func (r *Repository) latestProvenance(ctx context.Context, productID string) Provenance {
	row := r.db.QueryRowContext(ctx, `
		SELECT provider, source_type, source_id, source_url, captured_at
		FROM source_snapshots
		WHERE product_id = ?
		ORDER BY captured_at DESC, created_at DESC LIMIT 1
	`, productID)
	var prov Provenance
	var sourceID, sourceURL sql.NullString
	if err := row.Scan(&prov.Provider, &prov.SourceType, &sourceID, &sourceURL, &prov.CapturedAt); err != nil {
		return Provenance{}
	}
	prov.SourceID = nullStringPtr(sourceID)
	prov.SourceURL = nullStringPtr(sourceURL)
	return prov
}

// LatestProvenance returns provenance for the most recent snapshot.
func (r *Repository) LatestProvenance(ctx context.Context, productID string) Provenance {
	return r.latestProvenance(ctx, productID)
}

func (r *Repository) attachLatestDetail(ctx context.Context, result *ProductResult) {
	detail, ok, err := r.LatestDetail(ctx, result.ID)
	if err != nil || !ok {
		result.DetailCompleteness = "missing"
		result.DetailMissing = []string{"product_url", "image_url", "price", "selling_points", "review_summary"}
		return
	}
	result.DetailSnapshotID = &detail.ID
	result.ProductURL = detail.ProductURL
	result.ImageURL = detail.ImageURL
	result.Platform = detail.Platform
	result.ShopName = detail.ShopName
	result.Brand = detail.Brand
	result.Price = detail.Price
	result.Currency = detail.Currency
	result.SellingPoints = detail.SellingPoints
	result.Specs = detail.Specs
	result.ReviewSummary = detail.ReviewSummary
	result.ReviewHighlights = detail.ReviewHighlights
	result.DetailCompleteness = detail.Completeness
	result.DetailMissing = detail.MissingFields
	result.DetailMetadata = detail.Metadata
}

// AttachLatestDetail is the public form of attachLatestDetail.
func (r *Repository) AttachLatestDetail(ctx context.Context, result *ProductResult) {
	r.attachLatestDetail(ctx, result)
}

const productSelectColumns = `
	SELECT p.id, p.canonical_url, p.title, p.region, p.marketplace, p.asin,
		p.source_category, p.source_category_id, p.last_captured_at,
		p.category, p.workflow_state, p.metadata_json, p.created_at, p.updated_at
	FROM products p
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProduct(row rowScanner) (Product, error) {
	var product Product
	var metadataJSON string
	var canonicalURL, marketplace, asin, sourceCategory, sourceCategoryID sql.NullString
	var lastCapturedAt sql.NullString
	var category sql.NullString
	if err := row.Scan(
		&product.ID,
		&canonicalURL,
		&product.Title,
		&product.Region,
		&marketplace,
		&asin,
		&sourceCategory,
		&sourceCategoryID,
		&lastCapturedAt,
		&category,
		&product.WorkflowState,
		&metadataJSON,
		&product.CreatedAt,
		&product.UpdatedAt,
	); err != nil {
		return Product{}, err
	}
	product.CanonicalURL = nullStringPtr(canonicalURL)
	product.Marketplace = nullStringPtr(marketplace)
	product.ASIN = nullStringPtr(asin)
	product.SourceCategory = nullStringPtr(sourceCategory)
	product.SourceCategoryID = nullStringPtr(sourceCategoryID)
	if lastCapturedAt.Valid {
		ts := lastCapturedAt.String
		product.LastCapturedAt = &ts
	}
	product.Category = nullStringPtr(category)
	product.Metadata = unmarshalMap(metadataJSON)
	return product, nil
}

func missingDetailFields(input DetailInput) []string {
	var missing []string
	if input.ProductURL == nil || *input.ProductURL == "" {
		missing = append(missing, "product_url")
	}
	if input.ImageURL == nil || *input.ImageURL == "" {
		missing = append(missing, "image_url")
	}
	if input.Price == nil {
		missing = append(missing, "price")
	}
	if len(input.SellingPoints) == 0 {
		missing = append(missing, "selling_points")
	}
	if input.ReviewSummary == nil || *input.ReviewSummary == "" {
		missing = append(missing, "review_summary")
	}
	return missing
}

// DefaultStalenessHours is the threshold for marking a product as stale.
const DefaultStalenessHours = 48

func isStale(lastCapturedAt *string, thresholdHours int) bool {
	return IsStaleTime(deriveTime(lastCapturedAt), thresholdHours)
}

// IsStaleTime returns true when the supplied RFC3339 timestamp is older than
// thresholdHours or cannot be parsed.
func IsStaleTime(timestamp string, thresholdHours int) bool {
	if timestamp == "" {
		return true
	}
	ts, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return true
	}
	return time.Since(ts) > time.Duration(thresholdHours)*time.Hour
}

func deriveTime(lastCapturedAt *string) string {
	if lastCapturedAt == nil {
		return ""
	}
	return *lastCapturedAt
}

func marshalObject(value map[string]any) (string, error) {
	if value == nil {
		value = map[string]any{}
	}
	data, err := json.Marshal(value)
	return string(data), err
}

func marshalItems(items []string) (string, error) {
	if items == nil {
		items = []string{}
	}
	data, err := json.Marshal(map[string][]string{"items": items})
	return string(data), err
}

func unmarshalMap(value string) map[string]any {
	if value == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		return map[string]any{}
	}
	return out
}

func unmarshalItems(value string) []string {
	if value == "" {
		return nil
	}
	var out struct {
		Items []string `json:"items"`
	}
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		return nil
	}
	out.Items = sanitizeForLog(out.Items)
	return out.Items
}

func unmarshalFloatMap(value string) map[string]float64 {
	if value == "" {
		return map[string]float64{}
	}
	var out map[string]float64
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		return map[string]float64{}
	}
	return out
}

func utcNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func nullFloatPtr(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	return &value.Float64
}

func stringValue(value *string) any {
	if value == nil || *value == "" {
		return nil
	}
	return *value
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func floatValue(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func sanitizeForLog(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}