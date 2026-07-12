package product

type Product struct {
	ID                string         `json:"id"`
	CanonicalURL      *string        `json:"canonical_url,omitempty"`
	Title             string         `json:"title"`
	Region            string         `json:"region"`
	Marketplace       *string        `json:"marketplace,omitempty"`
	ASIN              *string        `json:"asin,omitempty"`
	SourceCategory    *string        `json:"source_category,omitempty"`
	SourceCategoryID  *string        `json:"source_category_id,omitempty"`
	LastCapturedAt    *string        `json:"last_captured_at,omitempty"`
	Category          *string        `json:"category,omitempty"`
	WorkflowState     string         `json:"workflow_state"`
	Metadata          map[string]any `json:"metadata"`
	CreatedAt         string         `json:"created_at"`
	UpdatedAt         string         `json:"updated_at"`
}

type ProductInput struct {
	Title            string
	Region           string
	Marketplace      *string
	ASIN             *string
	SourceCategory   *string
	SourceCategoryID *string
	Category         *string
	CanonicalURL     *string
	Provider         string
	SourceID         *string
	Metadata         map[string]any
}

type SnapshotInput struct {
	ProductID   string
	Provider    string
	SourceType  string
	SourceID    *string
	SourceURL   *string
	Metrics     map[string]float64
	MetricKind  string
	Metadata    map[string]any
}

type DetailInput struct {
	ProductID        string
	Provider         string
	Platform         *string
	SourceID         *string
	SourceURL        *string
	ProductURL       *string
	Title            *string
	ShopName         *string
	Brand            *string
	Price            *float64
	Currency         *string
	ImageURL         *string
	SourceCategory   *string
	Specs            map[string]any
	SellingPoints    []string
	ReviewSummary    *string
	ReviewHighlights []string
	Warnings         []string
	Raw              map[string]any
	Metadata         map[string]any
}

type ProductResult struct {
	ID                 string             `json:"id"`
	Title              string             `json:"title"`
	Region             string             `json:"region"`
	Marketplace        *string            `json:"marketplace,omitempty"`
	ASIN               *string            `json:"asin,omitempty"`
	SourceCategory     *string            `json:"source_category,omitempty"`
	SourceCategoryID   *string            `json:"source_category_id,omitempty"`
	Category           *string            `json:"category,omitempty"`
	CanonicalCategory  *CanonicalRef      `json:"canonical_category,omitempty"`
	WorkflowState      string             `json:"workflow_state"`
	CanonicalURL       *string            `json:"canonical_url,omitempty"`
	Metrics            map[string]float64 `json:"metrics"`
	MetricKind         string             `json:"metric_kind"`
	Estimated          bool               `json:"estimated"`
	Provenance         Provenance         `json:"provenance"`
	Freshness          Freshness          `json:"freshness"`
	Score              *ScoreSummary      `json:"score,omitempty"`
	DetailSnapshotID   *string            `json:"detail_snapshot_id,omitempty"`
	DetailCompleteness string             `json:"detail_completeness"`
	DetailMissing      []string           `json:"detail_missing_fields"`
	ProductURL         *string            `json:"product_url,omitempty"`
	ImageURL           *string            `json:"image_url,omitempty"`
	Platform           *string            `json:"platform,omitempty"`
	ShopName           *string            `json:"shop_name,omitempty"`
	Brand              *string            `json:"brand,omitempty"`
	Price              *float64           `json:"price,omitempty"`
	Currency           *string            `json:"currency,omitempty"`
	SellingPoints      []string           `json:"selling_points,omitempty"`
	Specs              map[string]any     `json:"specs,omitempty"`
	ReviewSummary      *string            `json:"review_summary,omitempty"`
	ReviewHighlights   []string           `json:"review_highlights,omitempty"`
	Metadata           map[string]any     `json:"metadata,omitempty"`
	DetailMetadata     map[string]any     `json:"detail_metadata,omitempty"`
}

// CanonicalRef describes the canonical classification of a product.
type CanonicalRef struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Slug       string  `json:"slug"`
	Confidence float64 `json:"confidence"`
	Method     string  `json:"method"`
	Manual     bool    `json:"manual"`
}

// Provenance indicates how a metric was observed.
type Provenance struct {
	Provider   string  `json:"provider"`
	SourceType string  `json:"source_type"`
	SourceID   *string `json:"source_id,omitempty"`
	SourceURL  *string `json:"source_url,omitempty"`
	CapturedAt string  `json:"captured_at"`
}

// Freshness describes how recent a product's data is.
type Freshness struct {
	LastCapturedAt *string `json:"last_captured_at,omitempty"`
	Stale          bool    `json:"stale"`
	ThresholdHours int     `json:"threshold_hours"`
}

// ScoreSummary summarizes the latest hotspot score snapshot.
type ScoreSummary struct {
	ID                  string             `json:"id"`
	TotalScore          float64            `json:"total_score"`
	Confidence          string             `json:"confidence"`
	ModelVersion        string             `json:"model_version"`
	TimeWindow          string             `json:"time_window"`
	ComparisonGroup     string             `json:"comparison_group,omitempty"`
	Components          map[string]float64 `json:"components"`
	MissingComponents   []string           `json:"missing_components"`
	MissingDetailFields []string           `json:"missing_detail_fields,omitempty"`
	CreatedAt           string             `json:"created_at"`
}