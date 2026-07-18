// Package copygen generates promotional copy variants for material requests
// through a pluggable LLM provider and persists them for the approval flow.
package copygen

// PromptVersion is recorded on every variant so prompt iterations stay
// traceable. Bump it when the generation prompt changes materially.
const PromptVersion = "1.0"

// CopyVariant is one generated copy draft persisted for a material request.
// Hashtags is persisted as a JSON array string in the hashtags column.
type CopyVariant struct {
	ID            string   `json:"id"`
	RequestID     string   `json:"request_id"`
	VariantNo     int      `json:"variant_no"`
	Hook          string   `json:"hook"`
	Body          string   `json:"body"`
	Caption       string   `json:"caption"`
	Hashtags      []string `json:"hashtags"`
	Provider      string   `json:"provider"`
	Model         string   `json:"model"`
	PromptVersion string   `json:"prompt_version"`
	CreatedAt     string   `json:"created_at"`
}
