package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/edvin/hosting/internal/api/response"
	"github.com/go-chi/chi/v5"
)

// Logs proxies log queries to Loki.
type Logs struct {
	lokiURL       string
	tenantLokiURL string
	client        *http.Client
}

// NewLogs creates a new Logs handler.
func NewLogs(lokiURL, tenantLokiURL string) *Logs {
	return &Logs{
		lokiURL:       strings.TrimRight(lokiURL, "/"),
		tenantLokiURL: strings.TrimRight(tenantLokiURL, "/"),
		client:        &http.Client{Timeout: 30 * time.Second},
	}
}

// LogEntry is a single log line with its stream labels.
type LogEntry struct {
	Timestamp string            `json:"timestamp"`
	Line      string            `json:"line"`
	Labels    map[string]string `json:"labels"`
}

// LogQueryResponse is the response from the logs endpoint.
type LogQueryResponse struct {
	Entries []LogEntry `json:"entries"`
}

// Query godoc
//
//	@Summary		Query logs
//	@Description	Query Loki for log entries matching a LogQL query
//	@Tags			Logs
//	@Security		ApiKeyAuth
//	@Param			query	query		string	true	"LogQL query"
//	@Param			start	query		string	false	"Start time (RFC3339 or relative like '1h')"
//	@Param			end		query		string	false	"End time (RFC3339, default now)"
//	@Param			limit	query		int		false	"Max entries (default 500, max 5000)"
//	@Success		200		{object}	LogQueryResponse
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		502		{object}	response.ErrorResponse
//	@Router			/logs [get]
func (h *Logs) Query(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		response.WriteError(w, http.StatusBadRequest, "query parameter is required")
		return
	}

	h.queryLoki(w, r, h.lokiURL, query)
}

// TenantLogs godoc
//
//	@Summary		Query tenant logs
//	@Description	Query access, error, and application logs for a specific tenant from the tenant Loki instance
//	@Tags			Tenants
//	@Security		ApiKeyAuth
//	@Param			tenantID   path  string true  "Tenant ID"
//	@Param			log_type   query string false "Log type filter (access, error, php-error, php-slow, app, cron)"
//	@Param			webroot_id  query string false "Filter by webroot ID"
//	@Param			cron_job_id query string false "Filter by cron job ID (for log_type=cron)"
//	@Param			start      query string false "Start time (RFC3339 or relative like '1h')"
//	@Param			end        query string false "End time (RFC3339, default now)"
//	@Param			limit      query int    false "Max entries (default 500, max 5000)"
//	@Success		200 {object} LogQueryResponse
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		502 {object} response.ErrorResponse
//	@Router			/tenants/{tenantID}/logs [get]
func (h *Logs) TenantLogs(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	if tenantID == "" {
		response.WriteError(w, http.StatusBadRequest, "tenantID is required")
		return
	}

	// Validate log_type if provided
	logType := r.URL.Query().Get("log_type")
	if logType != "" {
		validTypes := map[string]bool{
			"access":    true,
			"error":     true,
			"php-error": true,
			"php-slow":  true,
			"app":       true,
			"cron":      true,
		}
		if !validTypes[logType] {
			response.WriteError(w, http.StatusBadRequest, "invalid log_type: must be one of access, error, php-error, php-slow, app, cron")
			return
		}
	}

	// Build LogQL query
	query := fmt.Sprintf(`{tenant_id="%s"`, tenantID)
	if logType != "" {
		query += fmt.Sprintf(`, log_type="%s"`, logType)
	}
	if webrootID := r.URL.Query().Get("webroot_id"); webrootID != "" {
		query += fmt.Sprintf(`, webroot_id="%s"`, webrootID)
	}
	if cronJobID := r.URL.Query().Get("cron_job_id"); cronJobID != "" {
		query += fmt.Sprintf(`, cron_job_id="%s"`, cronJobID)
	}
	query += "}"

	h.queryLoki(w, r, h.tenantLokiURL, query)
}

// DeleteTenantLogs godoc
//
//	@Summary		Delete tenant logs
//	@Description	Delete all logs for a specific tenant from the tenant Loki instance
//	@Tags			Tenants
//	@Security		ApiKeyAuth
//	@Param			tenantID path  string true  "Tenant ID"
//	@Param			start    query string false "Start time (RFC3339, default 0)"
//	@Param			end      query string false "End time (RFC3339, default now)"
//	@Success		204
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		502 {object} response.ErrorResponse
//	@Router			/tenants/{tenantID}/logs [delete]
func (h *Logs) DeleteTenantLogs(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	if tenantID == "" {
		response.WriteError(w, http.StatusBadRequest, "tenantID is required")
		return
	}

	query := fmt.Sprintf(`{tenant_id="%s"}`, tenantID)

	now := time.Now()
	start := "0"
	end := fmt.Sprintf("%d", now.UnixNano())

	if s := r.URL.Query().Get("start"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			start = fmt.Sprintf("%d", t.UnixNano())
		}
	}
	if e := r.URL.Query().Get("end"); e != "" {
		if t, err := time.Parse(time.RFC3339, e); err == nil {
			end = fmt.Sprintf("%d", t.UnixNano())
		}
	}

	deleteURL := fmt.Sprintf("%s/loki/api/v1/delete?query=%s&start=%s&end=%s",
		h.tenantLokiURL,
		urlEncode(query),
		start,
		end,
	)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, deleteURL, nil)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to create loki delete request")
		return
	}

	resp, err := h.client.Do(req)
	if err != nil {
		response.WriteError(w, http.StatusBadGateway, "failed to delete logs from loki: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		response.WriteError(w, http.StatusBadGateway, fmt.Sprintf("loki delete returned %d: %s", resp.StatusCode, string(body)))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// queryLoki executes a LogQL query_range request against the given Loki URL and writes the response.
func (h *Logs) queryLoki(w http.ResponseWriter, r *http.Request, lokiBaseURL, query string) {
	limit := 500
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 5000 {
		limit = 5000
	}

	now := time.Now()
	start := now.Add(-1 * time.Hour)
	end := now

	if s := r.URL.Query().Get("start"); s != "" {
		if t, err := parseTime(s, now); err == nil {
			start = t
		}
	}
	if e := r.URL.Query().Get("end"); e != "" {
		if t, err := time.Parse(time.RFC3339, e); err == nil {
			end = t
		}
	}

	lokiURL := fmt.Sprintf("%s/loki/api/v1/query_range?query=%s&start=%d&end=%d&limit=%d&direction=backward",
		lokiBaseURL,
		urlEncode(query),
		start.UnixNano(),
		end.UnixNano(),
		limit,
	)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, lokiURL, nil)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to create loki request")
		return
	}

	resp, err := h.client.Do(req)
	if err != nil {
		response.WriteError(w, http.StatusBadGateway, "failed to query loki: "+err.Error())
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		response.WriteError(w, http.StatusBadGateway, "failed to read loki response")
		return
	}

	if resp.StatusCode != http.StatusOK {
		response.WriteError(w, http.StatusBadGateway, fmt.Sprintf("loki returned %d: %s", resp.StatusCode, string(body)))
		return
	}

	var lokiResp lokiQueryRangeResponse
	if err := json.Unmarshal(body, &lokiResp); err != nil {
		response.WriteError(w, http.StatusBadGateway, "failed to parse loki response")
		return
	}

	entries := flattenStreams(lokiResp)

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp < entries[j].Timestamp
	})

	response.WriteJSON(w, http.StatusOK, LogQueryResponse{Entries: entries})
}

// parseTime parses either an RFC3339 timestamp or a relative duration like "1h", "30m", "7d".
func parseTime(s string, now time.Time) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return time.Time{}, fmt.Errorf("invalid time: %s", s)
	}

	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time: %s", s)
	}

	var d time.Duration
	switch unit {
	case 'm':
		d = time.Duration(num) * time.Minute
	case 'h':
		d = time.Duration(num) * time.Hour
	case 'd':
		d = time.Duration(num) * 24 * time.Hour
	default:
		return time.Time{}, fmt.Errorf("unknown unit: %c", unit)
	}

	return now.Add(-d), nil
}

func urlEncode(s string) string {
	// Use net/url-style encoding for query parameter
	var b strings.Builder
	for _, c := range s {
		switch {
		case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~':
			b.WriteRune(c)
		default:
			encoded := fmt.Sprintf("%c", c)
			for i := 0; i < len(encoded); i++ {
				b.WriteString(fmt.Sprintf("%%%02X", encoded[i]))
			}
		}
	}
	return b.String()
}

// Loki response types

type lokiQueryRangeResponse struct {
	Data lokiData `json:"data"`
}

type lokiData struct {
	ResultType string       `json:"resultType"`
	Result     []lokiStream `json:"result"`
}

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"` // [timestamp_ns, line]
}

func flattenStreams(resp lokiQueryRangeResponse) []LogEntry {
	var entries []LogEntry
	for _, stream := range resp.Data.Result {
		for _, v := range stream.Values {
			if len(v) < 2 {
				continue
			}
			ts := v[0]
			line := v[1]

			// Convert nanosecond timestamp to RFC3339Nano
			if nsec, err := strconv.ParseInt(ts, 10, 64); err == nil {
				ts = time.Unix(0, nsec).UTC().Format(time.RFC3339Nano)
			}

			entries = append(entries, LogEntry{
				Timestamp: ts,
				Line:      line,
				Labels:    stream.Stream,
			})
		}
	}
	return entries
}
