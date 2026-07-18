package copygen

import (
	"context"
	"fmt"

	"tiktok-trend-shop/internal/product"
	"tiktok-trend-shop/internal/request"
)

// DefaultVariantCount is the number of copy variants generated per round when
// the caller does not ask for a specific count.
const DefaultVariantCount = 3

// StatusError reports that a request's current status does not allow copy
// generation. The API layer maps it to 409 Conflict.
type StatusError struct {
	Status string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("request status %q does not allow generation", e.Status)
}

// Service orchestrates copy generation against the material request
// lifecycle.
type Service struct {
	requests *request.Repository
	products *product.Repository
	variants *Repository
	provider Provider
}

func NewService(requests *request.Repository, products *product.Repository, variants *Repository, provider Provider) *Service {
	return &Service{requests: requests, products: products, variants: variants, provider: provider}
}

// GenerateForRequest produces a batch of copy variants for a material
// request. Only submitted or rejected requests may enter generation; the
// request is moved to generating, then to generated once the variants are
// persisted. On failure the request is reverted to submitted so the client
// can retry.
func (s *Service) GenerateForRequest(ctx context.Context, requestID string) ([]CopyVariant, error) {
	req, err := s.requests.Get(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if req.Status != request.StatusSubmitted && req.Status != request.StatusRejected {
		return nil, &StatusError{Status: req.Status}
	}
	if _, err := s.requests.UpdateStatus(ctx, requestID, request.StatusGenerating); err != nil {
		return nil, err
	}
	variants, err := s.generate(ctx, req)
	if err != nil {
		_ = s.forceStatus(ctx, requestID, request.StatusSubmitted)
		return nil, err
	}
	if _, err := s.requests.UpdateStatus(ctx, requestID, request.StatusGenerated); err != nil {
		_ = s.forceStatus(ctx, requestID, request.StatusSubmitted)
		return nil, err
	}
	return variants, nil
}

func (s *Service) generate(ctx context.Context, req request.MaterialRequest) ([]CopyVariant, error) {
	profile, err := s.loadProfile(ctx, req.ProductID)
	if err != nil {
		return nil, err
	}
	drafts, err := s.provider.Generate(ctx, GenInput{
		ProductTitle:  profile.Title,
		Brand:         profile.Brand,
		SellingPoints: profile.SellingPoints,
		Specs:         profile.Specs,
		ReviewSummary: profile.ReviewSummary,
		Usage:         req.Usage,
		Style:         req.Style,
		Focus:         req.Focus,
		Notes:         req.Notes,
		VariantCount:  DefaultVariantCount,
	})
	if err != nil {
		return nil, err
	}
	if len(drafts) == 0 {
		return nil, fmt.Errorf("copygen: provider %q returned no variants", s.provider.Name())
	}
	return s.variants.SaveVariants(ctx, req.ID, drafts, s.provider.Name(), providerModel(s.provider))
}

// PrefillForProduct drafts the brief fields of a material request for a
// product without persisting anything.
func (s *Service) PrefillForProduct(ctx context.Context, productID string) (PrefillSuggestion, error) {
	profile, err := s.loadProfile(ctx, productID)
	if err != nil {
		return PrefillSuggestion{}, err
	}
	return s.provider.Prefill(ctx, PrefillInput{
		ProductTitle:  profile.Title,
		Brand:         profile.Brand,
		SellingPoints: profile.SellingPoints,
		Specs:         profile.Specs,
		ReviewSummary: profile.ReviewSummary,
	})
}

// ListVariants returns the persisted variants of a request ordered by
// variant number.
func (s *Service) ListVariants(ctx context.Context, requestID string) ([]CopyVariant, error) {
	return s.variants.ListByRequest(ctx, requestID)
}

// productProfile is the normalized slice of the product detail snapshot used
// for copy generation.
type productProfile struct {
	Title         string
	Brand         string
	SellingPoints []string
	Specs         map[string]any
	ReviewSummary string
}

func (s *Service) loadProfile(ctx context.Context, productID string) (productProfile, error) {
	p, err := s.products.GetProduct(ctx, productID)
	if err != nil {
		return productProfile{}, err
	}
	profile := productProfile{Title: p.Title}
	detail, ok, err := s.products.LatestDetail(ctx, productID)
	if err != nil {
		return productProfile{}, err
	}
	if ok {
		if detail.Brand != nil {
			profile.Brand = *detail.Brand
		}
		profile.SellingPoints = detail.SellingPoints
		profile.Specs = detail.Specs
		if detail.ReviewSummary != nil {
			profile.ReviewSummary = *detail.ReviewSummary
		}
	}
	return profile, nil
}

// forceStatus resets a request's status directly, bypassing the transition
// table. It is only used to roll a failed generation back to submitted.
func (s *Service) forceStatus(ctx context.Context, requestID, status string) error {
	_, err := s.requests.DB().ExecContext(ctx, `
		UPDATE material_requests SET status = ?, updated_at = ?
		WHERE id = ?
	`, status, utcNow(), requestID)
	return err
}
