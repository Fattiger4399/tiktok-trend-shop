package mediagen

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"tiktok-trend-shop/internal/product"
)

// Defaults for the generate request fields.
const (
	DefaultUsage   = "main"
	DefaultStyle   = "clean"
	DefaultQuality = "fast"
)

// Quality presets: fast uses the FLUX.2 Klein distilled recipe (4 steps,
// CFG=1), hq the full one (20 steps, CFG=5).
const (
	qualityFast = "fast"
	qualityHQ   = "hq"
)

// defaultPollInterval is how often the provider is polled for completion.
const defaultPollInterval = 2 * time.Second

// maxSellingPoints caps how many detail selling points enter the prompt.
const maxSellingPoints = 3

// Service orchestrates image generation against the product catalog.
type Service struct {
	products *product.Repository
	assets   *AssetStore
	tasks    *Repository
	provider Provider

	// PollInterval overrides how often the provider is polled; zero uses the
	// 2s default. Tests set it to a few milliseconds.
	PollInterval time.Duration
}

func NewService(products *product.Repository, assets *AssetStore, tasks *Repository, provider Provider) *Service {
	return &Service{products: products, assets: assets, tasks: tasks, provider: provider}
}

// Health reports provider availability.
func (s *Service) Health(ctx context.Context) HealthStatus {
	status := HealthStatus{Provider: s.provider.Name(), Available: true}
	if err := s.provider.Available(ctx); err != nil {
		status.Available = false
		status.Detail = err.Error()
	}
	return status
}

// Generate creates a queued image generation task for a product and starts a
// background goroutine that submits the job and polls until the image is
// stored as an asset. The returned task is still queued.
func (s *Service) Generate(ctx context.Context, productID, usage, style, quality string) (ImageGenTask, error) {
	profile, err := s.loadProfile(ctx, productID)
	if err != nil {
		return ImageGenTask{}, err
	}
	usage = normalize(usage, DefaultUsage)
	style = normalize(style, DefaultStyle)
	quality = normalize(quality, DefaultQuality)
	if quality != qualityHQ {
		quality = qualityFast
	}
	steps, cfg := qualityParams(quality)
	input := JobInput{
		Prompt: buildPrompt(profile, usage, style),
		Seed:   rand.Int64(),
		Width:  1024,
		Height: 1024,
		Steps:  steps,
		CFG:    cfg,
	}
	task, err := s.tasks.CreateTask(ctx, ImageGenTask{
		ProductID: productID,
		Usage:     usage,
		Style:     style,
		Quality:   quality,
		Prompt:    input.Prompt,
		Seed:      input.Seed,
		Width:     input.Width,
		Height:    input.Height,
	})
	if err != nil {
		return ImageGenTask{}, err
	}
	go s.run(task.ID, productID, input)
	return task, nil
}

// ListImages returns the generated image assets of a product, newest first.
func (s *Service) ListImages(ctx context.Context, productID string) ([]Asset, error) {
	return s.assets.ListByProduct(ctx, productID)
}

// ListTasks returns the newest image generation tasks of a product (max 20).
func (s *Service) ListTasks(ctx context.Context, productID string) ([]ImageGenTask, error) {
	return s.tasks.ListTasksByProduct(ctx, productID, 20)
}

// run drives one task to completion. It detaches from the request context and
// bounds the whole submit→poll cycle by the provider timeout.
func (s *Service) run(taskID, productID string, input JobInput) {
	timeout := providerTimeout(s.provider)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	fail := func(cause error) {
		_ = s.tasks.MarkFailed(context.Background(), taskID, cause)
	}
	promptID, err := s.provider.Submit(ctx, input)
	if err != nil {
		fail(err)
		return
	}
	if err := s.tasks.MarkRunning(context.Background(), taskID, promptID); err != nil {
		fail(err)
		return
	}
	ticker := time.NewTicker(s.pollInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			fail(fmt.Errorf("mediagen: generation timed out after %s", timeout))
			return
		case <-ticker.C:
			done, images, err := s.provider.Fetch(ctx, promptID)
			if err != nil {
				fail(err)
				return
			}
			if !done {
				continue
			}
			if len(images) == 0 {
				fail(errors.New("mediagen: provider finished without images"))
				return
			}
			firstAssetID := ""
			for i, image := range images {
				asset, err := s.assets.Save(KindGeneratedImage, image, productID, map[string]any{
					"task_id": taskID,
					"prompt":  input.Prompt,
					"seed":    input.Seed,
					"width":   input.Width,
					"height":  input.Height,
				})
				if err != nil {
					fail(err)
					return
				}
				if i == 0 {
					firstAssetID = asset.ID
				}
			}
			if err := s.tasks.MarkSucceeded(context.Background(), taskID, firstAssetID); err != nil {
				fail(err)
			}
			return
		}
	}
}

func (s *Service) pollInterval() time.Duration {
	if s.PollInterval > 0 {
		return s.PollInterval
	}
	return defaultPollInterval
}

// productProfile is the normalized slice of the product used for prompt
// building.
type productProfile struct {
	Title         string
	Brand         string
	SellingPoints []string
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
	}
	return profile, nil
}

// buildPrompt renders the English text-to-image prompt from the product
// profile and the client brief. Selling points are capped at the first three
// and the segment is omitted entirely when none exist.
func buildPrompt(profile productProfile, usage, style string) string {
	display := strings.TrimSpace(profile.Title)
	if brand := strings.TrimSpace(profile.Brand); brand != "" {
		display = brand + " " + display
	}
	parts := []string{"product marketing photo of " + display}
	points := make([]string, 0, maxSellingPoints)
	for _, point := range profile.SellingPoints {
		trimmed := strings.TrimSpace(point)
		if trimmed == "" || len(points) >= maxSellingPoints {
			continue
		}
		points = append(points, trimmed)
	}
	if len(points) > 0 {
		parts = append(parts, strings.Join(points, ", "))
	}
	parts = append(parts,
		usage+" use",
		style+" style",
		"professional e-commerce photography",
	)
	return strings.Join(parts, ", ")
}

// qualityParams maps the quality preset onto sampler steps and CFG scale.
func qualityParams(quality string) (steps int, cfg float64) {
	if quality == qualityHQ {
		return 20, 5
	}
	return 4, 1
}

func normalize(value, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}
