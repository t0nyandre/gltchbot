// Package pagination provides utilities for handling paginated API requests and responses.
// It supports limit/offset pagination with sensible defaults and maximum limits.
package pagination

import (
	"net/http"
	"strconv"

	"github.com/t0nyandre/gltchbot/internal/api/response"
)

// DefaultLimit is the default number of items per page.
const DefaultLimit = 50

// MaxLimit is the maximum allowed limit to prevent excessive data retrieval.
const MaxLimit = 100

// PaginationParams represents pagination parameters from query string.
type PaginationParams struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// ParseQuery extracts limit and offset from HTTP request query parameters.
// If limit or offset are invalid or missing, default values are used.
// Returns PaginationParams with validated values.
func ParseQuery(r *http.Request) PaginationParams {
	limit := DefaultLimit
	offset := 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 {
			if val > MaxLimit {
				limit = MaxLimit
			} else {
				limit = val
			}
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if val, err := strconv.Atoi(offsetStr); err == nil && val >= 0 {
			offset = val
		}
	}

	return PaginationParams{
		Limit:  limit,
		Offset: offset,
	}
}

// PaginationResponse represents a paginated API response.
type PaginationResponse struct {
	Data       any                `json:"data"`
	Pagination PaginationMetadata `json:"pagination"`
}

// PaginationMetadata contains metadata about the pagination.
type PaginationMetadata struct {
	Total      int  `json:"total"`
	Limit      int  `json:"limit"`
	Offset     int  `json:"offset"`
	HasMore    bool `json:"has_more"`
	NextOffset *int `json:"next_offset,omitempty"`
}

// NewResponse creates a new PaginationResponse with the given data, total count, and pagination parameters.
func NewResponse(data any, total int, params PaginationParams) PaginationResponse {
	hasMore := total > params.Offset+params.Limit
	var nextOffset *int
	if hasMore {
		no := params.Offset + params.Limit
		nextOffset = &no
	}
	return PaginationResponse{
		Data: data,
		Pagination: PaginationMetadata{
			Total:      total,
			Limit:      params.Limit,
			Offset:     params.Offset,
			HasMore:    hasMore,
			NextOffset: nextOffset,
		},
	}
}

// WritePaginatedResponse writes a paginated response with status 200 OK.
func WritePaginatedResponse(w http.ResponseWriter, data any, total int, params PaginationParams) {
	resp := NewResponse(data, total, params)
	response.OK(w, resp)
}

// NextOffset returns the next offset value for pagination, or -1 if there are no more items.
func NextOffset(total int, params PaginationParams) int {
	if total > params.Offset+params.Limit {
		return params.Offset + params.Limit
	}
	return -1
}