package request

import "net/http"

// ListParams holds pagination, search, filter, and sort parameters.
type ListParams struct {
	Limit    int
	Cursor   string
	Search   string
	Status   string
	Sort     string
	Order    string // "asc" or "desc"
	BrandIDs []string // populated from auth context, not query params
}

// ParseListParams extracts list parameters from the query string.
// defaultSort specifies which field to sort by when none is provided.
func ParseListParams(r *http.Request, defaultSort string) ListParams {
	pg := ParsePagination(r)
	order := stringOr(r.URL.Query().Get("order"), "desc")
	if order != "asc" && order != "desc" {
		order = "desc"
	}
	return ListParams{
		Limit:  pg.Limit,
		Cursor: pg.Cursor,
		Search: r.URL.Query().Get("search"),
		Status: r.URL.Query().Get("status"),
		Sort:   stringOr(r.URL.Query().Get("sort"), defaultSort),
		Order:  order,
	}
}

func stringOr(val, fallback string) string {
	if val != "" {
		return val
	}
	return fallback
}
