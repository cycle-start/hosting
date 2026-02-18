package response

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

// WriteServiceError maps well-known service errors to appropriate HTTP status
// codes. Falls back to 500 Internal Server Error for unrecognized errors.
func WriteServiceError(w http.ResponseWriter, err error) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			WriteError(w, http.StatusConflict, err.Error())
			return
		case "23503": // foreign_key_violation
			WriteError(w, http.StatusBadRequest, err.Error())
			return
		case "23502": // not_null_violation
			WriteError(w, http.StatusBadRequest, err.Error())
			return
		case "23514": // check_violation
			WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	WriteError(w, http.StatusInternalServerError, err.Error())
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
