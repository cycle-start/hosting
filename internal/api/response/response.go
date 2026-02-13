package response

import (
	"encoding/json"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ErrorResponse is the standard error response body.
type ErrorResponse struct {
	Error string `json:"error"`
}

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorResponse{Error: message})
}

// PaginatedResponse wraps a list with pagination metadata.
type PaginatedResponse struct {
	Items      any    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

// WritePaginated writes a paginated JSON response.
func WritePaginated(w http.ResponseWriter, status int, items any, nextCursor string, hasMore bool) {
	WriteJSON(w, status, PaginatedResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	})
}
