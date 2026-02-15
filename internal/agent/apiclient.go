package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"github.com/edvin/hosting/internal/model"
)

// APIClient communicates with core-api internal endpoints.
type APIClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
	logger     zerolog.Logger
	etag       string // cached ETag for desired-state
}

// NewAPIClient creates a new API client for core-api.
func NewAPIClient(baseURL, token string, logger zerolog.Logger) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger.With().Str("component", "api-client").Logger(),
	}
}

// GetDesiredState fetches the desired state for a node. Returns nil if not modified (304).
func (c *APIClient) GetDesiredState(ctx context.Context, nodeID string) (*model.DesiredState, error) {
	url := fmt.Sprintf("%s/internal/v1/nodes/%s/desired-state", c.baseURL, nodeID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if c.etag != "" {
		req.Header.Set("If-None-Match", c.etag)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch desired state: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("desired state API returned %d: %s", resp.StatusCode, string(body))
	}

	if etag := resp.Header.Get("ETag"); etag != "" {
		c.etag = etag
	}

	var ds model.DesiredState
	if err := json.NewDecoder(resp.Body).Decode(&ds); err != nil {
		return nil, fmt.Errorf("decode desired state: %w", err)
	}
	return &ds, nil
}

// ReportHealth sends a health report to core-api.
func (c *APIClient) ReportHealth(ctx context.Context, nodeID string, health *model.NodeHealth) error {
	return c.postJSON(ctx, fmt.Sprintf("/internal/v1/nodes/%s/health", nodeID), health)
}

// ReportDriftEvents sends drift events to core-api.
func (c *APIClient) ReportDriftEvents(ctx context.Context, nodeID string, events []DriftEvent) error {
	payload := struct {
		Events []DriftEvent `json:"events"`
	}{Events: events}
	return c.postJSON(ctx, fmt.Sprintf("/internal/v1/nodes/%s/drift-events", nodeID), payload)
}

func (c *APIClient) postJSON(ctx context.Context, path string, payload any) error {
	url := c.baseURL + path
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
