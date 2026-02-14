package mcpserver

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// SwaggerSpec represents a Swagger 2.0 specification (subset we care about).
type SwaggerSpec struct {
	BasePath    string                            `json:"basePath"`
	Paths       map[string]map[string]Operation   `json:"paths"`
	Definitions map[string]json.RawMessage        `json:"definitions"`
}

// Operation represents a single API operation.
type Operation struct {
	Tags        []string    `json:"tags"`
	Summary     string      `json:"summary"`
	Description string      `json:"description"`
	OperationID string      `json:"operationId"`
	Parameters  []Parameter `json:"parameters"`
	Responses   map[string]json.RawMessage `json:"responses"`
}

// Parameter represents an API parameter.
type Parameter struct {
	Name        string          `json:"name"`
	In          string          `json:"in"`
	Required    bool            `json:"required"`
	Type        string          `json:"type"`
	Description string          `json:"description"`
	Default     any             `json:"default"`
	Schema      json.RawMessage `json:"schema"`
	Enum        []any           `json:"enum"`
	Format      string          `json:"format"`
}

// ToolOperation holds the data needed to proxy a tool call.
type ToolOperation struct {
	Method     string
	Path       string // URL path template with {param} placeholders
	Parameters []Parameter
}

// ParseSpec parses a Swagger 2.0 JSON spec.
func ParseSpec(data []byte) (*SwaggerSpec, error) {
	var spec SwaggerSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse swagger spec: %w", err)
	}
	return &spec, nil
}

// BuildTools generates MCP tools grouped by the config's group definitions.
// Returns a map of group name → []ServerTool, and a map of tool name → ToolOperation for proxying.
func BuildTools(spec *SwaggerSpec, cfg *Config, proxyFn func(op ToolOperation) server.ToolHandlerFunc) (map[string][]server.ServerTool, map[string]ToolOperation) {
	tagMap := cfg.tagToGroup()
	groups := make(map[string][]server.ServerTool)
	operations := make(map[string]ToolOperation)

	for path, methods := range spec.Paths {
		for method, op := range methods {
			method = strings.ToUpper(method)

			// Determine group from first tag
			group := ""
			if len(op.Tags) > 0 {
				group = tagMap[op.Tags[0]]
			}
			if group == "" {
				continue // skip operations not in any group
			}

			// Derive tool name
			toolName := deriveName(method, path, op)

			// Apply overrides
			override, hasOverride := cfg.Overrides[toolName]
			if hasOverride && override.Name != "" {
				toolName = override.Name
			}

			// Build description
			desc := op.Description
			if desc == "" {
				desc = op.Summary
			}
			if hasOverride && override.Description != "" {
				desc = override.Description
			}

			// Build MCP tool parameters from API parameters
			toolOpts := []mcp.ToolOption{
				mcp.WithDescription(desc),
			}

			// Add annotations
			toolOpts = append(toolOpts, buildAnnotations(method, cfg, override, hasOverride)...)

			// Add parameters
			fullPath := spec.BasePath + path
			toolOpts = append(toolOpts, buildParams(op.Parameters)...)

			tool := server.ServerTool{
				Tool: mcp.NewTool(toolName, toolOpts...),
				Handler: proxyFn(ToolOperation{
					Method:     method,
					Path:       fullPath,
					Parameters: op.Parameters,
				}),
			}

			groups[group] = append(groups[group], tool)
			operations[toolName] = ToolOperation{
				Method:     method,
				Path:       fullPath,
				Parameters: op.Parameters,
			}
		}
	}

	return groups, operations
}

// buildAnnotations creates MCP annotation options from config defaults and overrides.
func buildAnnotations(method string, cfg *Config, override ToolOverride, hasOverride bool) []mcp.ToolOption {
	var opts []mcp.ToolOption

	defaults := cfg.Defaults[method]

	readOnly := defaults.ReadOnly
	destructive := defaults.Destructive
	idempotent := defaults.Idempotent

	if hasOverride {
		if override.ReadOnly != nil {
			readOnly = override.ReadOnly
		}
		if override.Destructive != nil {
			destructive = override.Destructive
		}
		if override.Idempotent != nil {
			idempotent = override.Idempotent
		}
	}

	if readOnly != nil {
		opts = append(opts, mcp.WithReadOnlyHintAnnotation(*readOnly))
	}
	if destructive != nil {
		opts = append(opts, mcp.WithDestructiveHintAnnotation(*destructive))
	}
	if idempotent != nil {
		opts = append(opts, mcp.WithIdempotentHintAnnotation(*idempotent))
	}

	return opts
}

// buildParams converts API parameters to MCP tool parameter options.
func buildParams(params []Parameter) []mcp.ToolOption {
	var opts []mcp.ToolOption

	for _, p := range params {
		switch p.In {
		case "path":
			popts := paramOpts(p)
			opts = append(opts, mcp.WithString(p.Name, popts...))

		case "query":
			popts := paramOpts(p)
			switch p.Type {
			case "integer", "number":
				opts = append(opts, mcp.WithNumber(p.Name, popts...))
			case "boolean":
				opts = append(opts, mcp.WithBoolean(p.Name, popts...))
			default:
				opts = append(opts, mcp.WithString(p.Name, popts...))
			}

		case "body":
			// Body is passed as a single JSON string parameter
			bodyDesc := p.Description
			if bodyDesc == "" {
				bodyDesc = "Request body (JSON object)"
			}
			popts := []mcp.PropertyOption{
				mcp.Description(bodyDesc),
			}
			if p.Required {
				popts = append(popts, mcp.Required())
			}
			opts = append(opts, mcp.WithString("body", popts...))

		case "formData":
			popts := paramOpts(p)
			opts = append(opts, mcp.WithString(p.Name, popts...))
		}
	}

	return opts
}

// paramOpts builds PropertyOption slice from a Parameter.
func paramOpts(p Parameter) []mcp.PropertyOption {
	var opts []mcp.PropertyOption

	desc := p.Description
	if desc == "" {
		desc = p.Name
	}
	opts = append(opts, mcp.Description(desc))

	if p.Required {
		opts = append(opts, mcp.Required())
	}

	if len(p.Enum) > 0 {
		var vals []string
		for _, v := range p.Enum {
			vals = append(vals, fmt.Sprintf("%v", v))
		}
		opts = append(opts, mcp.Enum(vals...))
	}

	return opts
}

// deriveName generates a tool name from the HTTP method and path.
func deriveName(method, path string, op Operation) string {
	// Clean up path
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	// Separate resource segments from parameter segments
	var resources []string
	for _, p := range parts {
		if !strings.HasPrefix(p, "{") {
			resources = append(resources, p)
		}
	}

	if len(resources) == 0 {
		return strings.ToLower(method)
	}

	// Normalize hyphens to underscores
	for i := range resources {
		resources[i] = strings.ReplaceAll(resources[i], "-", "_")
	}

	lastRes := resources[len(resources)-1]
	endsWithParam := strings.HasPrefix(parts[len(parts)-1], "{")

	switch method {
	case "GET":
		if endsWithParam {
			return "get_" + singularize(lastRes)
		}
		// Special cases for non-collection GETs
		if lastRes == "stats" && len(resources) >= 2 {
			return "get_" + resources[len(resources)-2] + "_stats"
		}
		if lastRes == "config" && len(resources) >= 2 {
			return "get_" + resources[len(resources)-2] + "_config"
		}
		if lastRes == "autoreply" || lastRes == "resource_summary" {
			parent := findParentResource(parts, resources)
			return "get_" + singularize(parent) + "_" + lastRes
		}
		if lastRes == "search" {
			return "search"
		}
		// Collection endpoint
		return "list_" + lastRes

	case "POST":
		if !endsWithParam && len(parts) >= 2 {
			prevPart := parts[len(parts)-2]
			if strings.HasPrefix(prevPart, "{") {
				// POST /resources/{id}/action or POST /parent/{id}/children
				if looksLikeCollection(lastRes) {
					return "create_" + singularize(lastRes)
				}
				// Action endpoint
				parent := findParentResource(parts, resources)
				return lastRes + "_" + singularize(parent)
			}
		}
		return "create_" + singularize(lastRes)

	case "PUT":
		if !endsWithParam && len(resources) >= 2 {
			parent := findParentResource(parts, resources)
			return "set_" + singularize(parent) + "_" + lastRes
		}
		return "update_" + singularize(lastRes)

	case "DELETE":
		if !endsWithParam && len(resources) >= 2 {
			parent := findParentResource(parts, resources)
			return "delete_" + singularize(parent) + "_" + lastRes
		}
		return "delete_" + singularize(lastRes)
	}

	return strings.ToLower(method) + "_" + lastRes
}

// findParentResource finds the resource segment before the last parameter.
func findParentResource(parts, resources []string) string {
	if len(resources) >= 2 {
		return resources[len(resources)-2]
	}
	// Fallback: look through all parts
	for i := len(parts) - 1; i >= 0; i-- {
		if !strings.HasPrefix(parts[i], "{") {
			seg := strings.ReplaceAll(parts[i], "-", "_")
			return seg
		}
	}
	return "unknown"
}

// looksLikeCollection returns true if the segment name looks plural.
func looksLikeCollection(s string) bool {
	return strings.HasSuffix(s, "s") ||
		strings.HasSuffix(s, "es") ||
		strings.HasSuffix(s, "keys") ||
		strings.HasSuffix(s, "users")
}

// singularize performs a simple English singularization.
func singularize(s string) string {
	// Handle common patterns in this API
	exceptions := map[string]string{
		"aliases":          "alias",
		"addresses":        "address",
		"lb_addresses":     "lb_address",
		"audit_logs":       "audit_log",
		"api_keys":         "api_key",
		"sftp_keys":        "sftp_key",
		"access_keys":      "access_key",
		"s3_buckets":       "s3_bucket",
		"s3_access_keys":   "s3_access_key",
		"email_accounts":   "email_account",
		"email_aliases":    "email_alias",
		"email_forwards":   "email_forward",
		"email_autoreplies": "email_autoreply",
		"autoreplies":      "autoreply",
		"database_users":   "database_user",
		"valkey_instances":  "valkey_instance",
		"valkey_users":     "valkey_user",
		"zone_records":     "zone_record",
		"login_sessions":   "login_session",
		"certificates":     "certificate",
		"runtimes":         "runtime",
		"clusters":         "cluster",
		"regions":          "region",
		"shards":           "shard",
		"nodes":            "node",
		"tenants":          "tenant",
		"webroots":         "webroot",
		"fqdns":            "fqdn",
		"zones":            "zone",
		"brands":           "brand",
		"databases":        "database",
		"backups":          "backup",
		"forwards":         "forward",
		"users":            "user",
		"records":          "record",
		"instances":        "instance",
		"buckets":          "bucket",
		"keys":             "key",
	}

	if singular, ok := exceptions[s]; ok {
		return singular
	}

	// Generic: remove trailing 's'
	if strings.HasSuffix(s, "ies") {
		return s[:len(s)-3] + "y"
	}
	if strings.HasSuffix(s, "ses") || strings.HasSuffix(s, "xes") {
		return s[:len(s)-2]
	}
	if strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss") {
		return s[:len(s)-1]
	}
	return s
}
