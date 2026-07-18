package request

// MaterialRequest is a client brief asking for promotional material of a
// product. AI copy generation and the approval flow attach to it later.
type MaterialRequest struct {
	ID        string `json:"id"`
	ProductID string `json:"product_id"`
	ClientID  string `json:"client_id"`
	Usage     string `json:"usage"`
	Style     string `json:"style"`
	Focus     string `json:"focus"`
	Notes     string `json:"notes"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// RequestInput carries the client-supplied fields of a new request.
type RequestInput struct {
	ProductID string
	ClientID  string
	Usage     string
	Style     string
	Focus     string
	Notes     string
}

// Request lifecycle states.
const (
	StatusSubmitted  = "submitted"
	StatusGenerating = "generating"
	StatusGenerated  = "generated"
	StatusApproved   = "approved"
	StatusRejected   = "rejected"
	StatusDelivered  = "delivered"
)

// transitions lists the legal status transitions. A rejected request may go
// back to generating for another round.
var transitions = map[string][]string{
	StatusSubmitted:  {StatusGenerating},
	StatusGenerating: {StatusGenerated},
	StatusGenerated:  {StatusApproved, StatusRejected},
	StatusApproved:   {StatusDelivered},
	StatusRejected:   {StatusGenerating},
}

// CanTransition reports whether moving from one status to another is legal.
func CanTransition(from, to string) bool {
	for _, allowed := range transitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}
