// Package category implements the canonical category hierarchy, source-category
// mappings, and product category assignment pipeline.
//
// Classification follows a deterministic order:
//
//  1. Manual assignment (manual=true) is treated as authoritative and is never
//     overwritten by automatic classification.
//  2. Exact source-category mapping (provider + source_category_id) when active.
//  3. Deterministic keyword rule against the source category name and product
//     title.
//  4. Otherwise the assignment is recorded with needs_review=true and a low
//     confidence value.
//
// Confidence is exposed as a 0.0-1.0 value where >= autoAssignThreshold
// (default 0.75) is treated as a confident automatic assignment.
package category

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"tiktok-trend-shop/internal/id"
)

// DefaultAutoAssignThreshold is the minimum confidence for an automatic
// classification to be considered a confident assignment.
const DefaultAutoAssignThreshold = 0.75

// Seed represents one canonical category in the initial hierarchy.
type Seed struct {
	ID           string
	ParentID     string
	Name         string
	Slug         string
	DisplayOrder int
}

// DefaultSeeds returns the initial canonical category hierarchy. Slugs and
// identifiers are stable and platform-owned.
func DefaultSeeds() []Seed {
	roots := []Seed{
		{ID: "cat-beauty", Name: "Beauty & Personal Care", Slug: "beauty", DisplayOrder: 10},
		{ID: "cat-home", Name: "Home & Kitchen", Slug: "home", DisplayOrder: 20},
		{ID: "cat-electronics", Name: "Electronics", Slug: "electronics", DisplayOrder: 30},
		{ID: "cat-fashion", Name: "Fashion", Slug: "fashion", DisplayOrder: 40},
		{ID: "cat-toys", Name: "Toys & Games", Slug: "toys", DisplayOrder: 50},
		{ID: "cat-sports", Name: "Sports & Outdoors", Slug: "sports", DisplayOrder: 60},
		{ID: "cat-grocery", Name: "Grocery & Gourmet", Slug: "grocery", DisplayOrder: 70},
		{ID: "cat-pet", Name: "Pet Supplies", Slug: "pet", DisplayOrder: 80},
		{ID: "cat-office", Name: "Office Products", Slug: "office", DisplayOrder: 90},
		{ID: "cat-automotive", Name: "Automotive", Slug: "automotive", DisplayOrder: 100},
		{ID: "cat-tools", Name: "Tools & Home Improvement", Slug: "tools", DisplayOrder: 110},
		{ID: "cat-baby", Name: "Baby Products", Slug: "baby", DisplayOrder: 120},
	}
	children := map[string][]Seed{
		"cat-beauty": {
			{ID: "cat-beauty-skin", ParentID: "cat-beauty", Name: "Skin Care", Slug: "skin-care", DisplayOrder: 11},
			{ID: "cat-beauty-hair", ParentID: "cat-beauty", Name: "Hair Care", Slug: "hair-care", DisplayOrder: 12},
		},
		"cat-home": {
			{ID: "cat-home-decor", ParentID: "cat-home", Name: "Home Decor", Slug: "home-decor", DisplayOrder: 21},
			{ID: "cat-home-kitchen", ParentID: "cat-home", Name: "Kitchen & Dining", Slug: "kitchen", DisplayOrder: 22},
		},
		"cat-electronics": {
			{ID: "cat-electronics-audio", ParentID: "cat-electronics", Name: "Audio", Slug: "audio", DisplayOrder: 31},
			{ID: "cat-electronics-wearable", ParentID: "cat-electronics", Name: "Wearables", Slug: "wearables", DisplayOrder: 32},
		},
		"cat-fashion": {
			{ID: "cat-fashion-women", ParentID: "cat-fashion", Name: "Women", Slug: "women", DisplayOrder: 41},
			{ID: "cat-fashion-men", ParentID: "cat-fashion", Name: "Men", Slug: "men", DisplayOrder: 42},
		},
	}
	out := make([]Seed, 0, len(roots)+8)
	out = append(out, roots...)
	for _, list := range children {
		out = append(out, list...)
	}
	return out
}

// Canonical is the projection of a canonical_categories row.
type Canonical struct {
	ID           string  `json:"id"`
	ParentID     *string `json:"parent_id,omitempty"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	DisplayOrder int     `json:"display_order"`
	Active       bool    `json:"active"`
}

// Mapping is the projection of a source_category_mappings row.
type Mapping struct {
	ID                  string  `json:"id"`
	Provider            string  `json:"provider"`
	SourceCategoryID    string  `json:"source_category_id"`
	SourceCategoryName  *string `json:"source_category_name,omitempty"`
	CanonicalCategoryID string  `json:"canonical_category_id"`
	Confidence          float64 `json:"confidence"`
	Method              string  `json:"method"`
	Active              bool    `json:"active"`
}

// Assignment is the projection of a product_category_assignments row.
type Assignment struct {
	ID                 string  `json:"id"`
	ProductID          string  `json:"product_id"`
	CanonicalID        *string `json:"canonical_category_id,omitempty"`
	Method             string  `json:"method"`
	Confidence         float64 `json:"confidence"`
	SourceCategoryID   *string `json:"source_category_id,omitempty"`
	SourceCategoryName *string `json:"source_category_name,omitempty"`
	Manual             bool    `json:"manual"`
	NeedsReview        bool    `json:"needs_review"`
	AssignedAt         string  `json:"assigned_at"`
	UpdatedAt          string  `json:"updated_at"`
}

// Repository centralizes category persistence.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// SeedCanonical inserts the default canonical category hierarchy. Existing
// rows with the same ID are left untouched so the seed is idempotent.
func (r *Repository) SeedCanonical(ctx context.Context) error {
	now := time.Now().UTC().Format(time.RFC3339)
	for _, seed := range DefaultSeeds() {
		_, err := r.db.ExecContext(ctx, `
			INSERT OR IGNORE INTO canonical_categories(
				id, parent_id, name, slug, display_order, active, created_at, updated_at
			)
			VALUES (?, ?, ?, ?, ?, 1, ?, ?)
		`, seed.ID, nullableString(seed.ParentID), seed.Name, seed.Slug, seed.DisplayOrder, now, now)
		if err != nil {
			return err
		}
	}
	return nil
}

// UpsertMapping creates or updates a source-category mapping.
func (r *Repository) UpsertMapping(ctx context.Context, m Mapping) (string, error) {
	if m.Provider == "" || m.SourceCategoryID == "" || m.CanonicalCategoryID == "" {
		return "", fmt.Errorf("mapping requires provider, source_category_id, and canonical_category_id")
	}
	if m.Method == "" {
		m.Method = "exact"
	}
	if m.Confidence == 0 {
		m.Confidence = 1.0
	}
	now := time.Now().UTC().Format(time.RFC3339)
	row := r.db.QueryRowContext(ctx, `
		SELECT id FROM source_category_mappings
		WHERE provider = ? AND source_category_id = ?
	`, m.Provider, m.SourceCategoryID)
	var existing string
	switch err := row.Scan(&existing); {
	case err == nil:
		_, err := r.db.ExecContext(ctx, `
			UPDATE source_category_mappings
			SET source_category_name = ?, canonical_category_id = ?,
				confidence = ?, method = ?, active = ?, updated_at = ?
			WHERE id = ?
		`, nullableStringPtr(m.SourceCategoryName), m.CanonicalCategoryID, m.Confidence, m.Method,
			boolToInt(m.Active), now, existing)
		if err != nil {
			return "", err
		}
		return existing, nil
	case errors.Is(err, sql.ErrNoRows):
		newID := id.New("map")
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO source_category_mappings(
				id, provider, source_category_id, source_category_name,
				canonical_category_id, confidence, method, active, created_at, updated_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?)
		`, newID, m.Provider, m.SourceCategoryID, nullableStringPtr(m.SourceCategoryName),
			m.CanonicalCategoryID, m.Confidence, m.Method, now, now)
		if err != nil {
			return "", err
		}
		return newID, nil
	default:
		return "", err
	}
}

// ListCanonical returns the active canonical categories ordered for display.
func (r *Repository) ListCanonical(ctx context.Context) ([]Canonical, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, parent_id, name, slug, display_order, active
		FROM canonical_categories
		WHERE active = 1
		ORDER BY display_order, name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Canonical
	for rows.Next() {
		var c Canonical
		var parent sql.NullString
		var active int
		if err := rows.Scan(&c.ID, &parent, &c.Name, &c.Slug, &c.DisplayOrder, &active); err != nil {
			return nil, err
		}
		c.ParentID = nullStringPtr(parent)
		c.Active = active == 1
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListMappings returns all active source-category mappings.
func (r *Repository) ListMappings(ctx context.Context) ([]Mapping, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, provider, source_category_id, source_category_name,
			canonical_category_id, confidence, method, active
		FROM source_category_mappings
		WHERE active = 1
		ORDER BY provider, source_category_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Mapping
	for rows.Next() {
		var m Mapping
		var name sql.NullString
		var active int
		if err := rows.Scan(&m.ID, &m.Provider, &m.SourceCategoryID, &name,
			&m.CanonicalCategoryID, &m.Confidence, &m.Method, &active); err != nil {
			return nil, err
		}
		m.SourceCategoryName = nullStringPtr(name)
		m.Active = active == 1
		out = append(out, m)
	}
	return out, rows.Err()
}

// LookupExactMapping finds the active exact mapping for a provider + source category.
func (r *Repository) LookupExactMapping(ctx context.Context, provider, sourceCategoryID string) (Mapping, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, provider, source_category_id, source_category_name,
			canonical_category_id, confidence, method, active
		FROM source_category_mappings
		WHERE provider = ? AND source_category_id = ? AND active = 1
	`, provider, sourceCategoryID)
	var m Mapping
	var name sql.NullString
	var active int
	err := row.Scan(&m.ID, &m.Provider, &m.SourceCategoryID, &name,
		&m.CanonicalCategoryID, &m.Confidence, &m.Method, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return Mapping{}, false, nil
	}
	if err != nil {
		return Mapping{}, false, err
	}
	m.SourceCategoryName = nullStringPtr(name)
	m.Active = active == 1
	return m, true, nil
}

// GetAssignment returns the current assignment for a product.
func (r *Repository) GetAssignment(ctx context.Context, productID string) (Assignment, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, product_id, canonical_category_id, method, confidence,
			source_category_id, source_category_name, manual, needs_review,
			assigned_at, updated_at
		FROM product_category_assignments
		WHERE product_id = ?
	`, productID)
	var a Assignment
	var canonical sql.NullString
	var sourceID sql.NullString
	var sourceName sql.NullString
	var manual, needsReview int
	err := row.Scan(&a.ID, &a.ProductID, &canonical, &a.Method, &a.Confidence,
		&sourceID, &sourceName, &manual, &needsReview, &a.AssignedAt, &a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Assignment{}, false, nil
	}
	if err != nil {
		return Assignment{}, false, err
	}
	a.CanonicalID = nullStringPtr(canonical)
	a.SourceCategoryID = nullStringPtr(sourceID)
	a.SourceCategoryName = nullStringPtr(sourceName)
	a.Manual = manual == 1
	a.NeedsReview = needsReview == 1
	return a, true, nil
}

// SaveAssignment upserts the current assignment for a product. Manual=true
// overrides any automatic classification.
func (r *Repository) SaveAssignment(ctx context.Context, a Assignment) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if a.AssignedAt == "" {
		a.AssignedAt = now
	}
	a.UpdatedAt = now
	existing, ok, err := r.GetAssignment(ctx, a.ProductID)
	if err != nil {
		return err
	}
	if ok && existing.Manual && !a.Manual {
		// Refuse to overwrite a manual assignment.
		return ErrManualAssignmentLocked
	}
	if ok {
		_, err := r.db.ExecContext(ctx, `
			UPDATE product_category_assignments
			SET canonical_category_id = ?, method = ?, confidence = ?,
				source_category_id = ?, source_category_name = ?,
				manual = ?, needs_review = ?,
				assigned_at = ?, updated_at = ?
			WHERE id = ?
		`, nullableStringPtr(a.CanonicalID), a.Method, a.Confidence,
			nullableStringPtr(a.SourceCategoryID), nullableStringPtr(a.SourceCategoryName),
			boolToInt(a.Manual), boolToInt(a.NeedsReview), a.AssignedAt, now, existing.ID)
		return err
	}
	newID := id.New("asgn")
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO product_category_assignments(
			id, product_id, canonical_category_id, method, confidence,
			source_category_id, source_category_name, manual, needs_review,
			assigned_at, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, newID, a.ProductID, nullableStringPtr(a.CanonicalID), a.Method, a.Confidence,
		nullableStringPtr(a.SourceCategoryID), nullableStringPtr(a.SourceCategoryName),
		boolToInt(a.Manual), boolToInt(a.NeedsReview), a.AssignedAt, now, now)
	return err
}

// ListUnresolved returns assignments marked as needs_review ordered by recency.
func (r *Repository) ListUnresolved(ctx context.Context, limit int) ([]Assignment, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, product_id, canonical_category_id, method, confidence,
			source_category_id, source_category_name, manual, needs_review,
			assigned_at, updated_at
		FROM product_category_assignments
		WHERE needs_review = 1
		ORDER BY updated_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Assignment
	for rows.Next() {
		var a Assignment
		var canonical sql.NullString
		var sourceID sql.NullString
		var sourceName sql.NullString
		var manual, needsReview int
		if err := rows.Scan(&a.ID, &a.ProductID, &canonical, &a.Method, &a.Confidence,
			&sourceID, &sourceName, &manual, &needsReview, &a.AssignedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.CanonicalID = nullStringPtr(canonical)
		a.SourceCategoryID = nullStringPtr(sourceID)
		a.SourceCategoryName = nullStringPtr(sourceName)
		a.Manual = manual == 1
		a.NeedsReview = needsReview == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

// ErrManualAssignmentLocked is returned when an automatic classification
// attempts to overwrite a manual assignment.
var ErrManualAssignmentLocked = errors.New("manual category assignment is locked")

// KeywordRule is one deterministic keyword-to-canonical mapping.
type KeywordRule struct {
	Keyword    string
	CanonicalID string
	Confidence float64
}

// DefaultKeywordRules are the deterministic fallback rules applied when no
// exact source mapping exists. Each rule's confidence determines whether the
// assignment is treated as automatic or routed to manual review.
func DefaultKeywordRules() []KeywordRule {
	return []KeywordRule{
		{Keyword: "phone", CanonicalID: "cat-electronics-wearable", Confidence: 0.8},
		{Keyword: "earbud", CanonicalID: "cat-electronics-audio", Confidence: 0.85},
		{Keyword: "headphone", CanonicalID: "cat-electronics-audio", Confidence: 0.85},
		{Keyword: "speaker", CanonicalID: "cat-electronics-audio", Confidence: 0.8},
		{Keyword: "watch", CanonicalID: "cat-electronics-wearable", Confidence: 0.7},
		{Keyword: "lamp", CanonicalID: "cat-home-decor", Confidence: 0.85},
		{Keyword: "kitchen", CanonicalID: "cat-home-kitchen", Confidence: 0.85},
		{Keyword: "mug", CanonicalID: "cat-home-kitchen", Confidence: 0.85},
		{Keyword: "skincare", CanonicalID: "cat-beauty-skin", Confidence: 0.9},
		{Keyword: "skin care", CanonicalID: "cat-beauty-skin", Confidence: 0.9},
		{Keyword: "shampoo", CanonicalID: "cat-beauty-hair", Confidence: 0.9},
		{Keyword: "hair", CanonicalID: "cat-beauty-hair", Confidence: 0.7},
		{Keyword: "lipstick", CanonicalID: "cat-beauty", Confidence: 0.75},
		{Keyword: "makeup", CanonicalID: "cat-beauty", Confidence: 0.7},
		{Keyword: "pet", CanonicalID: "cat-pet", Confidence: 0.8},
		{Keyword: "dog", CanonicalID: "cat-pet", Confidence: 0.8},
		{Keyword: "cat food", CanonicalID: "cat-pet", Confidence: 0.85},
		{Keyword: "toy", CanonicalID: "cat-toys", Confidence: 0.7},
		{Keyword: "yoga", CanonicalID: "cat-sports", Confidence: 0.8},
		{Keyword: "fitness", CanonicalID: "cat-sports", Confidence: 0.8},
		{Keyword: "running", CanonicalID: "cat-sports", Confidence: 0.75},
		{Keyword: "office", CanonicalID: "cat-office", Confidence: 0.8},
		{Keyword: "notebook", CanonicalID: "cat-office", Confidence: 0.6},
		{Keyword: "car", CanonicalID: "cat-automotive", Confidence: 0.8},
		{Keyword: "auto", CanonicalID: "cat-automotive", Confidence: 0.7},
		{Keyword: "tool", CanonicalID: "cat-tools", Confidence: 0.8},
		{Keyword: "baby", CanonicalID: "cat-baby", Confidence: 0.85},
		{Keyword: "shirt", CanonicalID: "cat-fashion", Confidence: 0.7},
		{Keyword: "dress", CanonicalID: "cat-fashion-women", Confidence: 0.8},
	}
}

// ClassifyInput describes the inputs needed for automatic classification.
type ClassifyInput struct {
	ProductID          string
	Provider           string
	SourceCategoryID   string
	SourceCategoryName string
	Title              string
}

// Classify runs the classification order and persists the resulting assignment.
func (r *Repository) Classify(ctx context.Context, in ClassifyInput, threshold float64) (Assignment, error) {
	if threshold <= 0 {
		threshold = DefaultAutoAssignThreshold
	}
	assignment := Assignment{
		ProductID:          in.ProductID,
		Method:             "unresolved",
		SourceCategoryID:   stringPtr(in.SourceCategoryID),
		SourceCategoryName: stringPtr(in.SourceCategoryName),
	}
	if in.SourceCategoryID != "" {
		if mapping, ok, err := r.LookupExactMapping(ctx, in.Provider, in.SourceCategoryID); err != nil {
			return assignment, err
		} else if ok {
			assignment.CanonicalID = stringPtr(mapping.CanonicalCategoryID)
			assignment.Method = "exact_mapping"
			assignment.Confidence = mapping.Confidence
			assignment.NeedsReview = mapping.Confidence < threshold
			return assignment, r.SaveAssignment(ctx, assignment)
		}
	}
	text := strings.ToLower(strings.TrimSpace(in.SourceCategoryName + " " + in.Title))
	bestConfidence := 0.0
	var bestCanonical string
	for _, rule := range DefaultKeywordRules() {
		if strings.Contains(text, rule.Keyword) && rule.Confidence > bestConfidence {
			bestConfidence = rule.Confidence
			bestCanonical = rule.CanonicalID
		}
	}
	if bestCanonical != "" {
		assignment.CanonicalID = stringPtr(bestCanonical)
		assignment.Method = "keyword_rule"
		assignment.Confidence = bestConfidence
		assignment.NeedsReview = bestConfidence < threshold
		return assignment, r.SaveAssignment(ctx, assignment)
	}
	assignment.NeedsReview = true
	assignment.Confidence = 0.0
	return assignment, r.SaveAssignment(ctx, assignment)
}

// AssignManually records a manual assignment for a product.
func (r *Repository) AssignManually(ctx context.Context, productID, canonicalID, reviewer string) (Assignment, error) {
	if _, ok, err := r.canonicalExists(ctx, canonicalID); err != nil {
		return Assignment{}, err
	} else if !ok {
		return Assignment{}, fmt.Errorf("canonical category %q not found", canonicalID)
	}
	a := Assignment{
		ProductID:   productID,
		CanonicalID: stringPtr(canonicalID),
		Method:      "manual",
		Confidence:  1.0,
		Manual:      true,
		NeedsReview: false,
	}
	if err := r.SaveAssignment(ctx, a); err != nil {
		return Assignment{}, err
	}
	a2, _, err := r.GetAssignment(ctx, productID)
	return a2, err
}

func (r *Repository) canonicalExists(ctx context.Context, id string) (string, bool, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id FROM canonical_categories WHERE id = ? AND active = 1`, id)
	var got string
	err := row.Scan(&got)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return got, true, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableStringPtr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullStringPtr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}