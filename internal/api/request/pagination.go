package request

import (
	"net/http"
	"strconv"
)

// Pagination holds parsed pagination parameters.
type Pagination struct {
	Limit  int
	Cursor string
}

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// ParsePagination extracts limit and cursor from query parameters.
func ParsePagination(r *http.Request) Pagination {
	p := Pagination{
		Limit:  DefaultLimit,
		Cursor: r.URL.Query().Get("cursor"),
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			p.Limit = limit
		}
	}

	if p.Limit > MaxLimit {
		p.Limit = MaxLimit
	}

	return p
}
