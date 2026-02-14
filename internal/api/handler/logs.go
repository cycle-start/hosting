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
)

// Logs proxies log queries to Loki.
type Logs struct {
	lokiURL string
	client  *http.Client
}

// NewLogs creates a new Logs handler.
func NewLogs(lokiURL string) *Logs {
	return &Logs{
		lokiURL: strings.TrimRight(lokiURL, "/"),
		client:  &http.Client{Timeout: 30 * time.Second},
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
		h.lokiURL,
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
