package copygen

import (
	"context"
	"os"
	"time"
)

// Provider abstracts an LLM backend able to draft copy variants and prefill
// request briefs from a product profile.
type Provider interface {
	Name() string
	Generate(ctx context.Context, input GenInput) ([]VariantDraft, error)
	Prefill(ctx context.Context, input PrefillInput) (PrefillSuggestion, error)
}

// GenInput carries the product profile and the client brief into generation.
type GenInput struct {
	ProductTitle  string
	Brand         string
	SellingPoints []string
	Specs         map[string]any
	ReviewSummary string
	Usage         string
	Style         string
	Focus         string
	Notes         string
	VariantCount  int
}

// VariantDraft is one provider-produced copy variant before persistence.
type VariantDraft struct {
	Hook     string   `json:"hook"`
	Body     string   `json:"body"`
	Caption  string   `json:"caption"`
	Hashtags []string `json:"hashtags"`
}

// PrefillInput carries the product profile into brief prefill.
type PrefillInput struct {
	ProductTitle  string
	Brand         string
	SellingPoints []string
	Specs         map[string]any
	ReviewSummary string
}

// PrefillSuggestion is the AI-suggested draft of a material request brief.
type PrefillSuggestion struct {
	Usage string `json:"usage"`
	Style string `json:"style"`
	Focus string `json:"focus"`
	Notes string `json:"notes"`
}

// modelReporter is implemented by providers that expose the model name used
// for generation so it can be recorded on variants.
type modelReporter interface {
	Model() string
}

// providerModel returns the model name reported by the provider, if any.
func providerModel(provider Provider) string {
	if reporter, ok := provider.(modelReporter); ok {
		return reporter.Model()
	}
	return ""
}

// NewProviderFromEnv picks the LLM provider from the environment. With
// TTS_LLM_API_KEY set it returns an OpenAI-compatible provider configured by
// TTS_LLM_BASE_URL, TTS_LLM_MODEL and TTS_LLM_TIMEOUT; otherwise it falls
// back to the deterministic MockProvider.
func NewProviderFromEnv() Provider {
	apiKey := os.Getenv("TTS_LLM_API_KEY")
	if apiKey == "" {
		return MockProvider{}
	}
	return NewOpenAIProvider(
		getEnv("TTS_LLM_BASE_URL", "https://api.openai.com/v1"),
		apiKey,
		getEnv("TTS_LLM_MODEL", "gpt-4o-mini"),
		getEnvDuration("TTS_LLM_TIMEOUT", 30*time.Second),
	)
}

func getEnv(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
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
