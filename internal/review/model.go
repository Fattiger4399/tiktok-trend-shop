// Package review implements the approval flow of material requests: approve
// or reject generated copy variants, keep the review history, and freeze the
// approved variant into an exportable delivery package.
package review

import (
	"encoding/json"

	"tiktok-trend-shop/internal/copygen"
	"tiktok-trend-shop/internal/request"
)

// Review actions recorded on review_events.
const (
	ActionApproved = "approved"
	ActionRejected = "rejected"
)

// ReviewEvent is one approval-flow entry of a material request. VariantID is
// set on approvals and nil on rejections.
type ReviewEvent struct {
	ID        string  `json:"id"`
	RequestID string  `json:"request_id"`
	Action    string  `json:"action"`
	Actor     string  `json:"actor"`
	Note      string  `json:"note"`
	VariantID *string `json:"variant_id"`
	CreatedAt string  `json:"created_at"`
}

// Delivery is the frozen export package of an approved request. Package holds
// the package_json snapshot (request + selected variant + product summary) as
// raw JSON so API responses embed it without double encoding.
type Delivery struct {
	ID        string          `json:"id"`
	RequestID string          `json:"request_id"`
	VariantID string          `json:"variant_id"`
	Actor     string          `json:"actor"`
	Package   json.RawMessage `json:"package"`
	CreatedAt string          `json:"created_at"`
}

// DeliveryPackage is the snapshot structure persisted in
// deliveries.package_json at delivery time.
type DeliveryPackage struct {
	Request request.MaterialRequest `json:"request"`
	Variant copygen.CopyVariant     `json:"variant"`
	Product ProductSummary          `json:"product"`
}

// ProductSummary carries the product identity fields needed for export.
type ProductSummary struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	ASIN        *string `json:"asin"`
	Marketplace *string `json:"marketplace"`
}
