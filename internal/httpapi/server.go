package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"tiktok-trend-shop/internal/product"
)

type Server struct {
	repo *product.Repository
}

func New(repo *product.Repository) http.Handler {
	server := &Server{repo: repo}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.health)
	mux.HandleFunc("GET /products", server.products)
	return jsonMiddleware(mux)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) products(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r)
	results, err := s.repo.ListProducts(context.Background(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"count":   len(results),
		"results": results,
	})
}

func parseLimit(r *http.Request) int {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"message": message,
			"status":  status,
		},
	})
}
