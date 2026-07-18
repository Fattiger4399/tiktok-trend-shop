package apiv1

import (
	"database/sql"
	"errors"
	"io"
	"net/http"
	"strings"
)

// mediagenHealth reports image provider availability. Any logged-in user may
// check it so the frontend can grey out the generate entry when the local
// ComfyUI is not running.
func (s *Server) mediagenHealth(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, s.mediagen.Health(r.Context()))
}

// generateProductImage queues one marketing image generation task for the
// product and answers 202 with the queued task; completion is tracked through
// the images listing endpoint.
func (s *Server) generateProductImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteFieldError(w, "id", "id is required")
		return
	}
	var body struct {
		Usage   string `json:"usage"`
		Style   string `json:"style"`
		Quality string `json:"quality"`
	}
	// All fields are optional; an empty body takes the defaults.
	if err := decodeJSON(r, &body); err != nil && !errors.Is(err, io.EOF) {
		WriteFieldError(w, "body", "invalid JSON body")
		return
	}
	task, err := s.mediagen.Generate(r.Context(), id,
		strings.TrimSpace(body.Usage), strings.TrimSpace(body.Style), strings.TrimSpace(body.Quality))
	if err == sql.ErrNoRows {
		WriteNotFound(w, "product")
		return
	}
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusAccepted, task)
}

// listProductImages returns the generated image assets of a product together
// with its newest image generation tasks.
func (s *Server) listProductImages(w http.ResponseWriter, r *http.Request) {
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
	assets, err := s.mediagen.ListImages(r.Context(), id)
	if err != nil {
		WriteInternal(w, err)
		return
	}
	tasks, err := s.mediagen.ListTasks(r.Context(), id)
	if err != nil {
		WriteInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"assets": assets, "tasks": tasks})
}
