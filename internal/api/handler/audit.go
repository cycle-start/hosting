package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
)

// AuditLog represents an audit log entry.
type AuditLog struct {
	ID           string          `json:"id"`
	APIKeyID     *string         `json:"api_key_id,omitempty"`
	Method       string          `json:"method"`
	Path         string          `json:"path"`
	ResourceType *string         `json:"resource_type,omitempty"`
	ResourceID   *string         `json:"resource_id,omitempty"`
	StatusCode   int             `json:"status_code"`
	RequestBody  json.RawMessage `json:"request_body,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

type Audit struct {
	pool *pgxpool.Pool
}

func NewAudit(pool *pgxpool.Pool) *Audit {
	return &Audit{pool: pool}
}

func (h *Audit) List(w http.ResponseWriter, r *http.Request) {
	pg := request.ParsePagination(r)

	resourceType := r.URL.Query().Get("resource_type")

	query := `SELECT id, api_key_id, method, path, resource_type, resource_id, status_code, request_body, created_at
              FROM audit_logs WHERE 1=1`
	args := []any{}
	argIdx := 1

	if resourceType != "" {
		query += fmt.Sprintf(` AND resource_type = $%d`, argIdx)
		args = append(args, resourceType)
		argIdx++
	}

	if pg.Cursor != "" {
		query += fmt.Sprintf(` AND id > $%d`, argIdx)
		args = append(args, pg.Cursor)
		argIdx++
	}

	query += ` ORDER BY created_at DESC`
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, pg.Limit+1)

	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		if err := rows.Scan(&l.ID, &l.APIKeyID, &l.Method, &l.Path, &l.ResourceType, &l.ResourceID, &l.StatusCode, &l.RequestBody, &l.CreatedAt); err != nil {
			response.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		logs = append(logs, l)
	}

	hasMore := len(logs) > pg.Limit
	if hasMore {
		logs = logs[:pg.Limit]
	}
	var nextCursor string
	if hasMore && len(logs) > 0 {
		nextCursor = logs[len(logs)-1].ID
	}

	response.WritePaginated(w, http.StatusOK, logs, nextCursor, hasMore)
}
