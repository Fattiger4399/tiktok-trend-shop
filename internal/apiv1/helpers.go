// Package apiv1 implements the versioned /api/v1 JSON API.
//
// The package exposes a small set of helpers for response envelopes, pagination,
// validation, and error formatting so all routes share consistent behavior.
// The legacy /healthz and /products handlers remain in the parent httpapi
// package for backward compatibility.
package apiv1

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// MaxPageSize bounds the number of items returned per page.
const MaxPageSize = 100

// DefaultPageSize is the default number of items per page.
const DefaultPageSize = 20

// APIError represents a structured error response.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

type errorEnvelope struct {
	Error APIError `json:"error"`
}

// ListResponse is the standard paginated list envelope.
type ListResponse struct {
	Items      any             `json:"items"`
	Pagination Pagination      `json:"pagination"`
	Effective  map[string]any  `json:"effective,omitempty"`
	Warnings   []APIError      `json:"warnings,omitempty"`
}

// Pagination describes page-level metadata.
type Pagination struct {
	Page        int  `json:"page"`
	PageSize    int  `json:"page_size"`
	TotalItems  int  `json:"total_items"`
	TotalPages  int  `json:"total_pages"`
	HasNext     bool `json:"has_next"`
	HasPrevious bool `json:"has_previous"`
}

// WriteJSON serializes value with the given status code.
func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// WriteError writes a structured error envelope.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, errorEnvelope{Error: APIError{Code: code, Message: message}})
}

// WriteFieldError writes a validation error for a specific field.
func WriteFieldError(w http.ResponseWriter, field, message string) {
	WriteJSON(w, http.StatusBadRequest, errorEnvelope{
		Error: APIError{Code: "validation_error", Message: message, Field: field},
	})
}

// WriteNotFound writes a not-found error.
func WriteNotFound(w http.ResponseWriter, resource string) {
	WriteError(w, http.StatusNotFound, "not_found", fmt.Sprintf("%s not found", resource))
}

// WriteInternal writes a generic internal error.
func WriteInternal(w http.ResponseWriter, err error) {
	WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
}

// ErrInvalidParam is returned when a query parameter fails validation.
var ErrInvalidParam = errors.New("invalid query parameter")

// ParsePage parses a positive page number, returning 1 for invalid input.
func ParsePage(raw string) int {
	if raw == "" {
		return 1
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return 1
	}
	return v
}

// ParsePageSize returns a page size clamped to the allowed range.
func ParsePageSize(raw string) int {
	if raw == "" {
		return DefaultPageSize
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return DefaultPageSize
	}
	if v > MaxPageSize {
		return MaxPageSize
	}
	return v
}

// ParseTimeWindow maps a window string to one of the supported buckets.
// Supported values are "24h", "7d", "30d", "90d", and "all".
func ParseTimeWindow(raw, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return fallback
	case "24h", "7d", "30d", "90d", "all":
		return strings.ToLower(raw)
	default:
		return fallback
	}
}

// ParseSort validates the sort key against the allowed set.
func ParseSort(raw string, allowed map[string]string, fallback string) string {
	if raw == "" {
		return fallback
	}
	v, ok := allowed[strings.ToLower(raw)]
	if !ok {
		return fallback
	}
	return v
}

// ParseDirection normalizes a sort direction.
func ParseDirection(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "asc":
		return "ASC"
	default:
		return "DESC"
	}
}

// PaginationFor builds a Pagination block from the raw inputs and total count.
func PaginationFor(page, pageSize, total int) Pagination {
	if total < 0 {
		total = 0
	}
	totalPages := total / pageSize
	if total%pageSize != 0 {
		totalPages++
	}
	return Pagination{
		Page:        page,
		PageSize:    pageSize,
		TotalItems:  total,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
	}
}