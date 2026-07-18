package mediagen

import (
	"context"
	"os"
	"time"
)

// Provider abstracts an image generation backend able to queue a text-to-image
// job and later deliver the rendered images.
type Provider interface {
	Name() string
	Available(ctx context.Context) error
	Submit(ctx context.Context, input JobInput) (promptID string, err error)
	Fetch(ctx context.Context, promptID string) (done bool, images [][]byte, err error)
}

// JobInput carries one rendered prompt plus the sampling parameters into the
// provider workflow.
type JobInput struct {
	Prompt string
	Seed   int64
	Width  int
	Height int
	Steps  int
	CFG    float64
}

// defaultTimeout bounds one full submit→poll cycle when the provider does not
// report its own timeout.
const defaultTimeout = 120 * time.Second

// timeoutReporter is implemented by providers that expose the timeout bounding
// a full generation cycle so the service can deadline the background worker.
type timeoutReporter interface {
	Timeout() time.Duration
}

// providerTimeout returns the generation cycle budget of the provider.
func providerTimeout(provider Provider) time.Duration {
	if reporter, ok := provider.(timeoutReporter); ok {
		if timeout := reporter.Timeout(); timeout > 0 {
			return timeout
		}
	}
	return defaultTimeout
}

// NewProviderFromEnv picks the image generation provider from the environment.
// With TTS_COMFYUI_BASE_URL set it returns a ComfyUI provider (ComfyUI itself
// defaults to http://127.0.0.1:8188) bounded by TTS_COMFYUI_TIMEOUT; otherwise
// it falls back to the deterministic MockProvider.
func NewProviderFromEnv() Provider {
	baseURL := os.Getenv("TTS_COMFYUI_BASE_URL")
	if baseURL == "" {
		return &MockProvider{}
	}
	return NewComfyUIProvider(baseURL, getEnvDuration("TTS_COMFYUI_TIMEOUT", defaultTimeout))
}

func getEnvDuration(name string, fallback time.Duration) time.Duration {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
