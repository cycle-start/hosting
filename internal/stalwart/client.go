package stalwart

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{httpClient: &http.Client{}}
}

func (c *Client) CreateDomain(ctx context.Context, baseURL, adminToken, domain string) error {
	payload := map[string]any{
		"type": "domain",
		"name": domain,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal create domain: %w", err)
	}

	url := fmt.Sprintf("%s/api/principal", baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create domain request: %w", err)
	}
	req.SetBasicAuth("admin", adminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("create domain: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create domain %s: status %d: %s", domain, resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *Client) DeleteDomain(ctx context.Context, baseURL, adminToken, domain string) error {
	url := fmt.Sprintf("%s/api/principal/%s", baseURL, domain)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("delete domain request: %w", err)
	}
	req.SetBasicAuth("admin", adminToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete domain: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete domain %s: status %d: %s", domain, resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) CreateAccount(ctx context.Context, baseURL, adminToken string, params CreateAccountParams) error {
	payload := map[string]any{
		"type":        "individual",
		"name":        params.Address,
		"description": params.DisplayName,
		"quota":       params.QuotaBytes,
		"secrets":     []string{params.Password},
		"emails":      []string{params.Address},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal create account: %w", err)
	}

	url := fmt.Sprintf("%s/api/principal", baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create account request: %w", err)
	}
	req.SetBasicAuth("admin", adminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("create account: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create account %s: status %d: %s", params.Address, resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *Client) DeleteAccount(ctx context.Context, baseURL, adminToken, address string) error {
	url := fmt.Sprintf("%s/api/principal/%s", baseURL, address)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("delete account request: %w", err)
	}
	req.SetBasicAuth("admin", adminToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete account %s: status %d: %s", address, resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) UpdateAccount(ctx context.Context, baseURL, adminToken, name string, ops []PatchOp) error {
	body, err := json.Marshal(ops)
	if err != nil {
		return fmt.Errorf("marshal patch ops: %w", err)
	}

	url := fmt.Sprintf("%s/api/principal/%s", baseURL, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("update account request: %w", err)
	}
	req.SetBasicAuth("admin", adminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("update account: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update account %s: status %d: %s", name, resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *Client) GetAccount(ctx context.Context, baseURL, adminToken, address string) (*Account, error) {
	url := fmt.Sprintf("%s/api/principal/%s", baseURL, address)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("get account request: %w", err)
	}
	req.SetBasicAuth("admin", adminToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get account %s: status %d: %s", address, resp.StatusCode, string(body))
	}

	var result struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Quota       int64  `json:"quota"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode account: %w", err)
	}

	return &Account{
		Address:     result.Name,
		DisplayName: result.Description,
		QuotaBytes:  result.Quota,
	}, nil
}

// GetPrincipalID retrieves the numeric principal ID for a given account name
// from the Stalwart admin API. This ID is needed to derive the JMAP account ID
// (Crockford base32 encoded).
func (c *Client) GetPrincipalID(ctx context.Context, baseURL, adminToken, accountName string) (uint32, error) {
	url := fmt.Sprintf("%s/api/principal/%s", baseURL, accountName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("get principal id request: %w", err)
	}
	req.SetBasicAuth("admin", adminToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("get principal id: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("get principal id %s: status %d: %s", accountName, resp.StatusCode, string(body))
	}

	var result struct {
		Data struct {
			ID uint32 `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode principal id: %w", err)
	}
	return result.Data.ID, nil
}
