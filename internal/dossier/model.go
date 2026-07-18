package dossier

import "strings"

// Asset kinds collected as selection evidence and AI-generation references.
const (
	KindImage = "image"
	KindText  = "text"
	KindLink  = "link"
)

// Asset is one promotional material item attached to a product dossier.
type Asset struct {
	ID        string `json:"id"`
	ProductID string `json:"product_id"`
	Kind      string `json:"kind"`
	URL       string `json:"url"`
	Content   string `json:"content"`
	Source    string `json:"source"`
	Note      string `json:"note"`
	CreatedBy string `json:"created_by"`
	CreatedAt string `json:"created_at"`
}

// AssetInput carries the client-supplied fields of a new asset.
type AssetInput struct {
	ProductID string
	Kind      string
	URL       string
	Content   string
	Source    string
	Note      string
	CreatedBy string
}

// ValidKind reports whether kind is one of the supported asset kinds.
func ValidKind(kind string) bool {
	switch kind {
	case KindImage, KindText, KindLink:
		return true
	}
	return false
}

// RequiresURL reports whether the kind must carry a url.
func RequiresURL(kind string) bool {
	return kind == KindImage || kind == KindLink
}

// Validate enforces the per-kind required fields on the input.
func Validate(input AssetInput) (field, message string, ok bool) {
	kind := strings.TrimSpace(input.Kind)
	if !ValidKind(kind) {
		return "kind", "kind must be one of image, text, link", false
	}
	if RequiresURL(kind) && strings.TrimSpace(input.URL) == "" {
		return "url", "url is required for image/link assets", false
	}
	if kind == KindText && strings.TrimSpace(input.Content) == "" {
		return "content", "content is required for text assets", false
	}
	return "", "", true
}
