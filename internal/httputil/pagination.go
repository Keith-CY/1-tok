package httputil

import (
	"net/http"
	"strconv"
)

// DefaultPageLimit is the default number of items per page.
const DefaultPageLimit = 50

// MaxPageLimit is the maximum allowed page size.
const MaxPageLimit = 200

// Pagination holds parsed limit/offset parameters.
type Pagination struct {
	Limit  int
	Offset int
}

// ParsePagination extracts limit and offset from query parameters.
// Missing or invalid values fall back to defaults (limit=50, offset=0).
// Limit is capped at MaxPageLimit.
func ParsePagination(r *http.Request) Pagination {
	limit := intParam(r, "limit", DefaultPageLimit)
	if limit > MaxPageLimit {
		limit = MaxPageLimit
	} else if limit <= 0 {
		limit = DefaultPageLimit
	}

	offset := intParam(r, "offset", 0)
	if offset < 0 {
		offset = 0
	}

	return Pagination{Limit: limit, Offset: offset}
}

// Apply returns at most p.Limit items starting at p.Offset from the slice.
// If offset exceeds the length, an empty slice is returned.
//
// NOTE: This is application-level pagination, suitable for the in-memory
// store. For production postgres usage, pagination should be pushed down
// to the repository layer with LIMIT/OFFSET SQL queries. See issue #59
// for the full database-level pagination plan.
func Apply[T any](items []T, p Pagination) []T {
	if p.Offset >= len(items) {
		return []T{}
	}
	end := p.Offset + p.Limit
	if end > len(items) {
		end = len(items)
	}
	return items[p.Offset:end]
}

func intParam(r *http.Request, key string, fallback int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

// PaginatedResponse is a standardized paginated response.
type PaginatedResponse[T any] struct {
	Data       []T            `json:"data"`
	Pagination PaginationInfo `json:"pagination"`
}

// PaginationInfo holds pagination metadata.
type PaginationInfo struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// NewPaginatedResponse creates a paginated response from a slice and pagination params.
func NewPaginatedResponse[T any](items []T, page Pagination) PaginatedResponse[T] {
	total := len(items)
	paged := Apply(items, page)
	return PaginatedResponse[T]{
		Data: paged,
		Pagination: PaginationInfo{
			Total:  total,
			Limit:  page.Limit,
			Offset: page.Offset,
		},
	}
}
