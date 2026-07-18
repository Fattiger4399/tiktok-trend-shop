package review

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"tiktok-trend-shop/internal/copygen"
	"tiktok-trend-shop/internal/product"
	"tiktok-trend-shop/internal/request"
)

// StatusError reports that a request's current status does not allow the
// attempted review action. The API layer maps it to 409 Conflict.
type StatusError struct {
	Status string
	Action string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("request status %q does not allow %s", e.Status, e.Action)
}

// VariantError reports that the selected copy variant is missing or does not
// belong to the request. The API layer maps it to a 400 field error on
// variant_id.
type VariantError struct {
	VariantID string
	RequestID string
}

func (e *VariantError) Error() string {
	return fmt.Sprintf("variant %q does not belong to request %q", e.VariantID, e.RequestID)
}

// Service orchestrates the approval flow against the material request
// lifecycle.
type Service struct {
	requests *request.Repository
	products *product.Repository
	variants *copygen.Repository
	repo     *Repository
}

func NewService(requests *request.Repository, products *product.Repository, variants *copygen.Repository, repo *Repository) *Service {
	return &Service{requests: requests, products: products, variants: variants, repo: repo}
}

// Approve marks a generated request as approved for the selected variant and
// records the review event.
func (s *Service) Approve(ctx context.Context, requestID, variantID, actor, note string) (ReviewEvent, error) {
	req, err := s.requests.Get(ctx, requestID)
	if err != nil {
		return ReviewEvent{}, err
	}
	if req.Status != request.StatusGenerated {
		return ReviewEvent{}, &StatusError{Status: req.Status, Action: "approve"}
	}
	variant, err := s.variantForRequest(ctx, req, variantID)
	if err != nil {
		return ReviewEvent{}, err
	}
	event, err := s.repo.AddEvent(ctx, EventInput{
		RequestID: req.ID,
		Action:    ActionApproved,
		Actor:     actor,
		Note:      note,
		VariantID: &variant.ID,
	})
	if err != nil {
		return ReviewEvent{}, err
	}
	if _, err := s.requests.UpdateStatus(ctx, requestID, request.StatusApproved); err != nil {
		return ReviewEvent{}, err
	}
	return event, nil
}

// Reject sends a generated request back with a mandatory reason so another
// generation round can start. The review event records the reason as note.
func (s *Service) Reject(ctx context.Context, requestID, actor, note string) (ReviewEvent, error) {
	if strings.TrimSpace(note) == "" {
		return ReviewEvent{}, fmt.Errorf("note is required")
	}
	req, err := s.requests.Get(ctx, requestID)
	if err != nil {
		return ReviewEvent{}, err
	}
	if req.Status != request.StatusGenerated {
		return ReviewEvent{}, &StatusError{Status: req.Status, Action: "reject"}
	}
	event, err := s.repo.AddEvent(ctx, EventInput{
		RequestID: req.ID,
		Action:    ActionRejected,
		Actor:     actor,
		Note:      note,
	})
	if err != nil {
		return ReviewEvent{}, err
	}
	if _, err := s.requests.UpdateStatus(ctx, requestID, request.StatusRejected); err != nil {
		return ReviewEvent{}, err
	}
	return event, nil
}

// Deliver freezes the approved request with the selected variant into an
// exportable package and moves the request to delivered. A request can be
// delivered only once; a second attempt reports ErrAlreadyDelivered (409).
func (s *Service) Deliver(ctx context.Context, requestID, variantID, actor string) (Delivery, error) {
	req, err := s.requests.Get(ctx, requestID)
	if err != nil {
		return Delivery{}, err
	}
	if req.Status != request.StatusApproved {
		return Delivery{}, &StatusError{Status: req.Status, Action: "deliver"}
	}
	variant, err := s.variantForRequest(ctx, req, variantID)
	if err != nil {
		return Delivery{}, err
	}
	p, err := s.products.GetProduct(ctx, req.ProductID)
	if err != nil {
		return Delivery{}, err
	}
	packageJSON, err := json.Marshal(DeliveryPackage{
		Request: req,
		Variant: variant,
		Product: ProductSummary{
			ID:          p.ID,
			Title:       p.Title,
			ASIN:        p.ASIN,
			Marketplace: p.Marketplace,
		},
	})
	if err != nil {
		return Delivery{}, err
	}
	delivery, err := s.repo.CreateDelivery(ctx, DeliveryInput{
		RequestID:   req.ID,
		VariantID:   variant.ID,
		Actor:       actor,
		PackageJSON: string(packageJSON),
	})
	if err != nil {
		return Delivery{}, err
	}
	if _, err := s.requests.UpdateStatus(ctx, requestID, request.StatusDelivered); err != nil {
		return Delivery{}, err
	}
	return delivery, nil
}

// ListEvents returns the review history of a request in chronological order.
func (s *Service) ListEvents(ctx context.Context, requestID string) ([]ReviewEvent, error) {
	return s.repo.ListEvents(ctx, requestID)
}

// GetDelivery returns the delivery of a request, or sql.ErrNoRows.
func (s *Service) GetDelivery(ctx context.Context, requestID string) (Delivery, error) {
	return s.repo.GetDelivery(ctx, requestID)
}

// GetDeliveryByID returns one delivery by id, or sql.ErrNoRows.
func (s *Service) GetDeliveryByID(ctx context.Context, deliveryID string) (Delivery, error) {
	return s.repo.GetDeliveryByID(ctx, deliveryID)
}

// ListDeliveries returns a page of deliveries plus the total count.
func (s *Service) ListDeliveries(ctx context.Context, page, pageSize int) ([]Delivery, int, error) {
	return s.repo.ListDeliveries(ctx, page, pageSize)
}

// variantForRequest loads the variant and verifies it belongs to the request.
func (s *Service) variantForRequest(ctx context.Context, req request.MaterialRequest, variantID string) (copygen.CopyVariant, error) {
	variant, err := s.variants.Get(ctx, variantID)
	if err == sql.ErrNoRows {
		return copygen.CopyVariant{}, &VariantError{VariantID: variantID, RequestID: req.ID}
	}
	if err != nil {
		return copygen.CopyVariant{}, err
	}
	if variant.RequestID != req.ID {
		return copygen.CopyVariant{}, &VariantError{VariantID: variantID, RequestID: req.ID}
	}
	return variant, nil
}
