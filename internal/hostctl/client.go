package hostctl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

type Response struct {
	StatusCode int
	Body       json.RawMessage
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Post(path string, body any) (*Response, error) {
	return c.do(http.MethodPost, path, body)
}

func (c *Client) Get(path string) (*Response, error) {
	return c.do(http.MethodGet, path, nil)
}

func (c *Client) Put(path string, body any) (*Response, error) {
	return c.do(http.MethodPut, path, body)
}

func (c *Client) Delete(path string) (*Response, error) {
	return c.do(http.MethodDelete, path, nil)
}

func (c *Client) do(method, path string, body any) (*Response, error) {
	url := c.BaseURL + path

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	r := &Response{
		StatusCode: resp.StatusCode,
		Body:       json.RawMessage(respBody),
	}

	if resp.StatusCode >= 400 {
		return r, fmt.Errorf("%s %s: status %d: %s", method, path, resp.StatusCode, string(respBody))
	}

	return r, nil
}

// Items extracts the "items" array from a paginated API response.
func (r *Response) Items() (json.RawMessage, error) {
	var page struct {
		Items json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(r.Body, &page); err != nil {
		return nil, fmt.Errorf("parse paginated response: %w", err)
	}
	return page.Items, nil
}
