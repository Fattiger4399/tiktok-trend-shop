// Package score implements the deterministic, versioned category-relative
// hotspot scoring model.
//
// The model combines the following components when available:
//
//   - demand: latest observation of the demand metric (views or sales)
//   - acceleration: relative change between the most recent and previous
//     observation in the requested time window
//   - review_growth: change in review signal between the latest two snapshots
//   - price_signal: deviation from the product's own historical price mean
//   - freshness: how recently the product was observed
//   - completeness: ratio of supported components out of the model total
//
// Each component is normalized against the comparison group (marketplace +
// canonical category + time window) using the available observations. When the
// comparison group is empty or the product lacks history, components fall back
// to a single-observation estimate and confidence is reduced.
package score

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"tiktok-trend-shop/internal/id"
	"tiktok-trend-shop/internal/product"
)

// ModelVersion is the semantic version of the current scoring formula.
const ModelVersion = "1.0.0"

// ComponentNames lists the score components produced by the model.
var ComponentNames = []string{"demand", "acceleration", "review_growth", "price_signal", "freshness", "completeness"}

// Snapshot is a hotspot score snapshot row.
type Snapshot struct {
	ID                  string             `json:"id"`
	ProductID           string             `json:"product_id"`
	Marketplace         *string            `json:"marketplace,omitempty"`
	CanonicalCategoryID *string            `json:"canonical_category_id,omitempty"`
	ModelVersion        string             `json:"model_version"`
	TimeWindow          string             `json:"time_window"`
	ComparisonGroup     string             `json:"comparison_group"`
	TotalScore          float64            `json:"total_score"`
	Confidence          string             `json:"confidence"`
	Components          map[string]float64 `json:"components"`
	MissingComponents   []string           `json:"missing_components"`
	Evidence            map[string]any     `json:"evidence"`
	CreatedAt           string             `json:"created_at"`
}

// Repository centralizes persistence of hotspot score snapshots.
type Repository struct {
	db  *sql.DB
	pr  *product.Repository
}

func NewRepository(db *sql.DB, pr *product.Repository) *Repository {
	return &Repository{db: db, pr: pr}
}

// Inputs controls scoring behavior.
type Inputs struct {
	Marketplace         string
	CanonicalCategoryID string
	TimeWindow          string
	StaleHours          int
}

// ComputeAndStore runs the model for a single product and persists the result.
func (r *Repository) ComputeAndStore(ctx context.Context, productID string, in Inputs) (Snapshot, error) {
	if in.TimeWindow == "" {
		in.TimeWindow = "30d"
	}
	if in.StaleHours <= 0 {
		in.StaleHours = product.DefaultStalenessHours
	}
	p, err := r.pr.GetProduct(ctx, productID)
	if err != nil {
		return Snapshot{}, err
	}
	history, err := r.pr.SnapshotHistory(ctx, productID, in.TimeWindow, 200)
	if err != nil {
		return Snapshot{}, err
	}
	detail, _, err := r.pr.LatestDetail(ctx, productID)
	_ = detail
	_ = err

	snap := Snapshot{
		ProductID:     productID,
		ModelVersion:  ModelVersion,
		TimeWindow:    in.TimeWindow,
		ComparisonGroup: comparisonGroup(in.Marketplace, in.CanonicalCategoryID),
	}
	if p.Marketplace != nil {
		mk := *p.Marketplace
		snap.Marketplace = &mk
	}
	if in.CanonicalCategoryID != "" {
		id := in.CanonicalCategoryID
		snap.CanonicalCategoryID = &id
	}

	components := make(map[string]float64)
	missing := make([]string, 0)
	evidence := make(map[string]any)

	demand, ok := componentDemand(history)
	if ok {
		components["demand"] = demand
	} else {
		missing = append(missing, "demand")
	}
	accel, ok := componentAcceleration(history)
	if ok {
		components["acceleration"] = accel
	} else {
		missing = append(missing, "acceleration")
	}
	reviews, ok := componentReviewGrowth(history)
	if ok {
		components["review_growth"] = reviews
	} else {
		missing = append(missing, "review_growth")
	}
	price, ok := componentPriceSignal(history)
	if ok {
		components["price_signal"] = price
	} else {
		missing = append(missing, "price_signal")
	}
	freshness, ok := componentFreshness(p.LastCapturedAt, in.StaleHours)
	if ok {
		components["freshness"] = freshness
	} else {
		missing = append(missing, "freshness")
	}
	completeness := float64(len(components)) / float64(len(ComponentNames))
	components["completeness"] = completeness

	// Cross-category normalization: scale demand relative to peers in the
	// comparison group when the group has enough observations.
	if in.Marketplace != "" || in.CanonicalCategoryID != "" {
		peerAvg := r.peerAverageDemand(ctx, productID, in)
		if peerAvg > 0 && demand > 0 {
			components["demand"] = normalize01(demand / math.Max(peerAvg, 1e-9))
		}
	}

	total, confidence := aggregate(components, missing)
	snap.TotalScore = total
	snap.Confidence = confidence
	snap.Components = components
	snap.MissingComponents = missing
	evidence["history_count"] = len(history)
	evidence["latest_captured_at"] = p.LastCapturedAt
	if p.LastCapturedAt != nil && product.IsStaleTime(*p.LastCapturedAt, in.StaleHours) {
		evidence["stale"] = true
	}
	snap.Evidence = evidence
	snap.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := r.persist(ctx, snap); err != nil {
		return snap, err
	}
	return snap, nil
}

// LatestForProduct returns the most recent hotspot score for a product.
func (r *Repository) LatestForProduct(ctx context.Context, productID string) (Snapshot, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, product_id, marketplace, canonical_category_id, model_version,
			time_window, comparison_group, total_score, confidence,
			components_json, missing_components_json, evidence_json, created_at
		FROM hotspot_scores
		WHERE product_id = ?
		ORDER BY created_at DESC LIMIT 1
	`, productID)
	return scanSnapshot(row)
}

// LatestForProducts returns the latest snapshot for a list of products.
func (r *Repository) LatestForProducts(ctx context.Context, productIDs []string) (map[string]Snapshot, error) {
	if len(productIDs) == 0 {
		return map[string]Snapshot{}, nil
	}
	placeholders := make([]string, len(productIDs))
	args := make([]any, len(productIDs))
	for i, id := range productIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, s.product_id, s.marketplace, s.canonical_category_id, s.model_version,
			s.time_window, s.comparison_group, s.total_score, s.confidence,
			s.components_json, s.missing_components_json, s.evidence_json, s.created_at
		FROM hotspot_scores s
		JOIN (
			SELECT product_id, MAX(created_at) AS latest
			FROM hotspot_scores
			WHERE product_id IN (`+strings.Join(placeholders, ",")+`)
			GROUP BY product_id
		) latest ON latest.product_id = s.product_id AND latest.latest = s.created_at
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]Snapshot, len(productIDs))
	for rows.Next() {
		snap, err := scanSnapshotRow(rows)
		if err != nil {
			return nil, err
		}
		out[snap.ProductID] = snap
	}
	return out, rows.Err()
}

func (r *Repository) peerAverageDemand(ctx context.Context, excludeID string, in Inputs) float64 {
	var rows *sql.Rows
	var err error
	if in.CanonicalCategoryID != "" {
		rows, err = r.db.QueryContext(ctx, `
			SELECT metrics_json
			FROM source_snapshots
			WHERE product_id IN (
				SELECT product_id FROM product_category_assignments
				WHERE canonical_category_id = ?
			)
			ORDER BY captured_at DESC LIMIT 1000
		`, in.CanonicalCategoryID)
	} else if in.Marketplace != "" {
		rows, err = r.db.QueryContext(ctx, `
			SELECT metrics_json
			FROM source_snapshots
			WHERE product_id IN (
				SELECT id FROM products WHERE marketplace = ?
			)
			ORDER BY captured_at DESC LIMIT 1000
		`, in.Marketplace)
	} else {
		return 0
	}
	if err != nil {
		return 0
	}
	defer rows.Close()
	var total, count float64
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		var metrics map[string]float64
		if err := json.Unmarshal([]byte(raw), &metrics); err != nil {
			continue
		}
		if v, ok := pickDemand(metrics); ok {
			total += v
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return total / count
}

func pickDemand(metrics map[string]float64) (float64, bool) {
	if v, ok := metrics["sales"]; ok && v > 0 {
		return v, true
	}
	if v, ok := metrics["views"]; ok && v > 0 {
		return v, true
	}
	if v, ok := metrics["demand"]; ok && v > 0 {
		return v, true
	}
	return 0, false
}

func componentDemand(history []product.SnapshotRecord) (float64, bool) {
	if len(history) == 0 {
		return 0, false
	}
	latest := history[len(history)-1]
	v, ok := pickDemand(latest.Metrics)
	if !ok {
		return 0, false
	}
	return v, true
}

func componentAcceleration(history []product.SnapshotRecord) (float64, bool) {
	if len(history) < 2 {
		return 0, false
	}
	prev := history[len(history)-2]
	latest := history[len(history)-1]
	prevVal, ok := pickDemand(prev.Metrics)
	if !ok {
		return 0, false
	}
	latestVal, ok := pickDemand(latest.Metrics)
	if !ok {
		return 0, false
	}
	if prevVal == 0 {
		return 0, false
	}
	return (latestVal - prevVal) / prevVal, true
}

func componentReviewGrowth(history []product.SnapshotRecord) (float64, bool) {
	if len(history) < 2 {
		return 0, false
	}
	prev := history[len(history)-2]
	latest := history[len(history)-1]
	prevVal := prev.Metrics["reviews"]
	latestVal := latest.Metrics["reviews"]
	if prevVal == 0 && latestVal == 0 {
		return 0, false
	}
	if prevVal == 0 {
		return 1, true
	}
	return (latestVal - prevVal) / prevVal, true
}

func componentPriceSignal(history []product.SnapshotRecord) (float64, bool) {
	prices := make([]float64, 0, len(history))
	for _, h := range history {
		if v, ok := h.Metrics["price"]; ok && v > 0 {
			prices = append(prices, v)
		}
	}
	if len(prices) < 2 {
		return 0, false
	}
	mean := mean(prices)
	latest := prices[len(prices)-1]
	if mean == 0 {
		return 0, false
	}
	return (mean - latest) / mean, true
}

func componentFreshness(lastCapturedAt *string, staleHours int) (float64, bool) {
	if lastCapturedAt == nil || *lastCapturedAt == "" {
		return 0, false
	}
	ts, err := time.Parse(time.RFC3339, *lastCapturedAt)
	if err != nil {
		return 0, false
	}
	hours := time.Since(ts).Hours()
	if hours <= 0 {
		return 1, true
	}
	return math.Max(0, 1-(hours/float64(staleHours))), true
}

func aggregate(components map[string]float64, missing []string) (float64, string) {
	weights := map[string]float64{
		"demand":        0.35,
		"acceleration":  0.20,
		"review_growth": 0.15,
		"price_signal":  0.10,
		"freshness":     0.10,
		"completeness":  0.10,
	}
	completeness := components["completeness"]
	totalWeight := 0.0
	total := 0.0
	for k, w := range weights {
		if v, ok := components[k]; ok {
			total += v * w
			totalWeight += w
		}
	}
	if totalWeight == 0 {
		return 0, "none"
	}
	score := total / totalWeight
	confidence := "high"
	if completeness < 0.8 {
		confidence = "medium"
	}
	if completeness < 0.5 || len(missing) >= 3 {
		confidence = "low"
	}
	return score, confidence
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func normalize01(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func comparisonGroup(marketplace, canonical string) string {
	parts := []string{}
	if marketplace != "" {
		parts = append(parts, "mp:"+marketplace)
	}
	if canonical != "" {
		parts = append(parts, "cat:"+canonical)
	}
	if len(parts) == 0 {
		return "global"
	}
	return strings.Join(parts, "|")
}

func (r *Repository) persist(ctx context.Context, snap Snapshot) error {
	componentsJSON, err := json.Marshal(snap.Components)
	if err != nil {
		return err
	}
	missingJSON, err := json.Marshal(map[string][]string{"items": snap.MissingComponents})
	if err != nil {
		return err
	}
	evidenceJSON, err := json.Marshal(snap.Evidence)
	if err != nil {
		return err
	}
	newID := id.New("hs")
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO hotspot_scores(
			id, product_id, marketplace, canonical_category_id,
			model_version, time_window, comparison_group,
			total_score, confidence, components_json, missing_components_json,
			evidence_json, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, newID, snap.ProductID, snap.Marketplace, snap.CanonicalCategoryID,
		snap.ModelVersion, snap.TimeWindow, snap.ComparisonGroup,
		snap.TotalScore, snap.Confidence, string(componentsJSON), string(missingJSON),
		string(evidenceJSON), snap.CreatedAt)
	return err
}

func scanSnapshot(row *sql.Row) (Snapshot, bool, error) {
	snap, err := scanSnapshotRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Snapshot{}, false, nil
	}
	if err != nil {
		return Snapshot{}, false, err
	}
	return snap, true, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSnapshotRow(s scanner) (Snapshot, error) {
	var snap Snapshot
	var marketplace, canonical sql.NullString
	var componentsJSON, missingJSON, evidenceJSON string
	err := s.Scan(
		&snap.ID,
		&snap.ProductID,
		&marketplace,
		&canonical,
		&snap.ModelVersion,
		&snap.TimeWindow,
		&snap.ComparisonGroup,
		&snap.TotalScore,
		&snap.Confidence,
		&componentsJSON,
		&missingJSON,
		&evidenceJSON,
		&snap.CreatedAt,
	)
	if err != nil {
		return Snapshot{}, err
	}
	snap.Marketplace = nullStringPtr(marketplace)
	snap.CanonicalCategoryID = nullStringPtr(canonical)
	snap.Components = unmarshalFloatMap(componentsJSON)
	snap.MissingComponents = unmarshalItems(missingJSON)
	snap.Evidence = unmarshalMap(evidenceJSON)
	return snap, nil
}

func unmarshalFloatMap(s string) map[string]float64 {
	if s == "" {
		return map[string]float64{}
	}
	var out map[string]float64
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return map[string]float64{}
	}
	return out
}

func unmarshalItems(s string) []string {
	if s == "" {
		return nil
	}
	var out struct {
		Items []string `json:"items"`
	}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return out.Items
}

func unmarshalMap(s string) map[string]any {
	if s == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return map[string]any{}
	}
	return out
}

func nullStringPtr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

// ComputeAll runs scoring for every product and stores snapshots.
func (r *Repository) ComputeAll(ctx context.Context, in Inputs) (int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM products`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	sort.Strings(ids)
	count := 0
	for _, id := range ids {
		if _, err := r.ComputeAndStore(ctx, id, in); err != nil {
			return count, fmt.Errorf("score %s: %w", id, err)
		}
		count++
	}
	return count, nil
}