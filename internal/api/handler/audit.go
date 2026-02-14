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

// List godoc
//
//	@Summary		List audit logs
//	@Description	Returns a paginated list of audit log entries. Supports filtering by resource_type, HTTP method (action), and date range (date_from/date_to). Each entry includes the acting API key, HTTP method, path, resource affected, status code, request body, and timestamp.
//	@Tags			Audit Logs
//	@Security		ApiKeyAuth
//	@Param			cursor			query		string	false	"Pagination cursor"
//	@Param			limit			query		int		false	"Page size (default 50)"
//	@Param			sort			query		string	false	"Sort field (method, resource_type, created_at)"
//	@Param			order			query		string	false	"Sort order (asc, desc)"
//	@Param			search			query		string	false	"Search in resource_type or method"
//	@Param			resource_type	query		string	false	"Filter by resource type"
//	@Param			action			query		string	false	"Filter by HTTP method"
//	@Param			date_from		query		string	false	"Filter by start date"
//	@Param			date_to			query		string	false	"Filter by end date"
//	@Success		200				{object}	response.PaginatedResponse{items=[]handler.AuditLog}
//	@Failure		500				{object}	response.ErrorResponse
//	@Router			/audit-logs [get]
func (h *Audit) List(w http.ResponseWriter, r *http.Request) {
	params := request.ParseListParams(r, "created_at")

	resourceType := r.URL.Query().Get("resource_type")
	action := r.URL.Query().Get("action")
	dateFrom := r.URL.Query().Get("date_from")
	dateTo := r.URL.Query().Get("date_to")

	query := `SELECT id, api_key_id, method, path, resource_type, resource_id, status_code, request_body, created_at
              FROM audit_logs WHERE 1=1`
	args := []any{}
	argIdx := 1

	if params.Search != "" {
		query += fmt.Sprintf(` AND (resource_type ILIKE $%d OR method ILIKE $%d)`, argIdx, argIdx+1)
		args = append(args, "%"+params.Search+"%", "%"+params.Search+"%")
		argIdx += 2
	}
	if resourceType != "" {
		query += fmt.Sprintf(` AND resource_type = $%d`, argIdx)
		args = append(args, resourceType)
		argIdx++
	}
	if action != "" {
		query += fmt.Sprintf(` AND method = $%d`, argIdx)
		args = append(args, action)
		argIdx++
	}
	if dateFrom != "" {
		query += fmt.Sprintf(` AND created_at >= $%d`, argIdx)
		args = append(args, dateFrom)
		argIdx++
	}
	if dateTo != "" {
		query += fmt.Sprintf(` AND created_at <= $%d`, argIdx)
		args = append(args, dateTo)
		argIdx++
	}

	if params.Cursor != "" {
		query += fmt.Sprintf(` AND id > $%d`, argIdx)
		args = append(args, params.Cursor)
		argIdx++
	}

	sortCol := "created_at"
	switch params.Sort {
	case "method":
		sortCol = "method"
	case "resource_type":
		sortCol = "resource_type"
	case "created_at":
		sortCol = "created_at"
	}
	order := "DESC"
	if params.Order == "asc" {
		order = "ASC"
	}
	query += fmt.Sprintf(` ORDER BY %s %s`, sortCol, order)
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, params.Limit+1)

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

	hasMore := len(logs) > params.Limit
	if hasMore {
		logs = logs[:params.Limit]
	}
	var nextCursor string
	if hasMore && len(logs) > 0 {
		nextCursor = logs[len(logs)-1].ID
	}

	response.WritePaginated(w, http.StatusOK, logs, nextCursor, hasMore)
}
