// Package mediagen generates product marketing images through a pluggable
// image generation provider (local ComfyUI by default) and persists the
// produced files as content-addressed assets.
package mediagen

// Image generation task lifecycle statuses.
const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

// KindGeneratedImage is the asset kind produced by the image generation
// pipeline.
const KindGeneratedImage = "generated-image"

// Asset is one generated file persisted on disk and indexed in the assets
// table. URI is the public path served by the /assets/ static route.
type Asset struct {
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	Backend     string         `json:"backend"`
	URI         string         `json:"uri"`
	ContentType string         `json:"content_type"`
	ByteSize    int64          `json:"byte_size"`
	Checksum    string         `json:"checksum"`
	ProductID   *string        `json:"product_id,omitempty"`
	RequestID   *string        `json:"request_id,omitempty"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   string         `json:"created_at"`
}

// ImageGenTask tracks one image generation request through the provider
// queue. ComfyPromptID, AssetID, LastError and CompletedAt stay empty until
// the background worker reaches the matching stage.
type ImageGenTask struct {
	ID            string  `json:"id"`
	ProductID     string  `json:"product_id"`
	Status        string  `json:"status"`
	Usage         string  `json:"usage"`
	Style         string  `json:"style"`
	Quality       string  `json:"quality"`
	Prompt        string  `json:"prompt"`
	Seed          int64   `json:"seed"`
	Width         int     `json:"width"`
	Height        int     `json:"height"`
	ComfyPromptID *string `json:"comfy_prompt_id,omitempty"`
	AssetID       *string `json:"asset_id,omitempty"`
	LastError     *string `json:"last_error,omitempty"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
	CompletedAt   *string `json:"completed_at,omitempty"`
}

// HealthStatus reports provider availability for the health endpoint. Detail
// carries the failure reason when the provider is unavailable.
type HealthStatus struct {
	Provider  string `json:"provider"`
	Available bool   `json:"available"`
	Detail    string `json:"detail,omitempty"`
}
