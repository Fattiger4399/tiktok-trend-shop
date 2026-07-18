package apiv1

import (
	"database/sql"
	"net/http"
	"strings"

	"tiktok-trend-shop/internal/dossier"
)

// createDossierAsset attaches a promotional material item to a product. The
// kind-specific required fields map onto 400 field errors; the actor is the
// login username (单端模式：任何登录用户可写).
func (s *Server) createDossierAsset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteFieldError(w, "id", "id is required")
		return
	}
	var body struct {
		Kind    string `json:"kind"`
		URL     string `json:"url"`
		Content string `json:"content"`
		Source  string `json:"source"`
		Note    string `json:"note"`
	}
	if err := decodeJSON(r, &body); err != nil {
		WriteFieldError(w, "body", "invalid JSON body")
		return
	}
	input := dossier.AssetInput{
		ProductID: id,
		Kind:      strings.TrimSpace(body.Kind),
		URL:       strings.TrimSpace(body.URL),
		Content:   strings.TrimSpace(body.Content),
		Source:    strings.TrimSpace(body.Source),
		Note:      strings.TrimSpace(body.Note),
	}
	if field, message, ok := dossier.Validate(input); !ok {
		WriteFieldError(w, field, message)
		return
	}
	if _, err := s.product.GetProduct(r.Context(), id); err == sql.ErrNoRows {
		WriteNotFound(w, "product")
		return
	} else if err != nil {
		WriteInternal(w, err)
		return
	}
	if user, ok := UserFromContext(r.Context()); ok {
		input.CreatedBy = user.Username
	}
	asset, err := s.dossier.Add(r.Context(), input)
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, asset)
}

func (s *Server) listDossierAssets(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteFieldError(w, "id", "id is required")
		return
	}
	if _, err := s.product.GetProduct(r.Context(), id); err == sql.ErrNoRows {
		WriteNotFound(w, "product")
		return
	} else if err != nil {
		WriteInternal(w, err)
		return
	}
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	items, err := s.dossier.List(r.Context(), id, kind)
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"product_id": id, "items": items})
}

func (s *Server) deleteDossierAsset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	assetID := r.PathValue("asset_id")
	if id == "" || assetID == "" {
		WriteFieldError(w, "id", "id is required")
		return
	}
	if err := s.dossier.Delete(r.Context(), id, assetID); err == sql.ErrNoRows {
		WriteNotFound(w, "dossier_asset")
		return
	} else if err != nil {
		WriteInternal(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
