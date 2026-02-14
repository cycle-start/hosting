package mcpserver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/rs/zerolog"
)

// ProxyHandler creates MCP tool handlers that proxy to the REST API.
type ProxyHandler struct {
	apiURL string
	client *http.Client
	logger zerolog.Logger
}

// NewProxyHandler creates a new proxy handler targeting the given API URL.
func NewProxyHandler(apiURL string, logger zerolog.Logger) *ProxyHandler {
	return &ProxyHandler{
		apiURL: strings.TrimRight(apiURL, "/"),
		client: &http.Client{},
		logger: logger,
	}
}

// Handler returns an MCP tool handler function for the given operation.
func (p *ProxyHandler) Handler(op ToolOperation) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Build the URL by substituting path parameters
		url := p.apiURL + op.Path
		args := req.GetArguments()

		for _, param := range op.Parameters {
			if param.In == "path" {
				val, ok := args[param.Name]
				if !ok {
					return mcp.NewToolResultError(fmt.Sprintf("missing required path parameter: %s", param.Name)), nil
				}
				url = strings.ReplaceAll(url, "{"+param.Name+"}", fmt.Sprintf("%v", val))
				// Also handle chi-style params like {regionID}
				for _, part := range strings.Split(op.Path, "/") {
					if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
						paramName := part[1 : len(part)-1]
						if paramName == param.Name {
							continue
						}
					}
				}
			}
		}

		// Build query string from query parameters
		queryParts := []string{}
		for _, param := range op.Parameters {
			if param.In == "query" {
				val, ok := args[param.Name]
				if ok && val != nil && fmt.Sprintf("%v", val) != "" {
					queryParts = append(queryParts, fmt.Sprintf("%s=%v", param.Name, val))
				}
			}
		}
		if len(queryParts) > 0 {
			url += "?" + strings.Join(queryParts, "&")
		}

		// Build request body
		var bodyReader io.Reader
		if body, ok := args["body"]; ok && body != nil {
			bodyStr := fmt.Sprintf("%v", body)
			if bodyStr != "" {
				bodyReader = strings.NewReader(bodyStr)
			}
		}

		// Handle form data (for OIDC token endpoint)
		var contentType string
		hasFormData := false
		formParts := []string{}
		for _, param := range op.Parameters {
			if param.In == "formData" {
				hasFormData = true
				if val, ok := args[param.Name]; ok && val != nil {
					formParts = append(formParts, fmt.Sprintf("%s=%v", param.Name, val))
				}
			}
		}
		if hasFormData && len(formParts) > 0 {
			bodyReader = strings.NewReader(strings.Join(formParts, "&"))
			contentType = "application/x-www-form-urlencoded"
		}

		// Create HTTP request
		httpReq, err := http.NewRequestWithContext(ctx, op.Method, url, bodyReader)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build request: %s", err)), nil
		}

		// Set content type
		if contentType != "" {
			httpReq.Header.Set("Content-Type", contentType)
		} else if bodyReader != nil {
			httpReq.Header.Set("Content-Type", "application/json")
		}

		// Forward the API key from MCP session headers
		apiKey := req.Header.Get("X-API-Key")
		if apiKey == "" {
			// Also check Authorization header (some MCP clients use Bearer)
			auth := req.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				apiKey = strings.TrimPrefix(auth, "Bearer ")
			}
		}
		if apiKey != "" {
			httpReq.Header.Set("X-API-Key", apiKey)
		}

		p.logger.Debug().
			Str("method", op.Method).
			Str("url", url).
			Str("tool", req.Params.Name).
			Msg("proxying MCP tool call")

		// Execute request
		resp, err := p.client.Do(httpReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("API request failed: %s", err)), nil
		}
		defer resp.Body.Close()

		// Read response body
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read response: %s", err)), nil
		}

		// Return error for non-2xx responses
		if resp.StatusCode >= 400 {
			return mcp.NewToolResultError(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody))), nil
		}

		// Return empty success for 204 No Content
		if resp.StatusCode == http.StatusNoContent {
			return mcp.NewToolResultText(`{"status":"success"}`), nil
		}

		// Return the JSON response as text
		return mcp.NewToolResultText(string(respBody)), nil
	}
}
