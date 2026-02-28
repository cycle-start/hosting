package mcpserver

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level MCP server configuration loaded from mcp.yaml.
type Config struct {
	APIURL   string                    `yaml:"api_url"`
	SpecPath string                    `yaml:"spec_path"`
	Defaults map[string]MethodDefaults `yaml:"defaults"`
	Groups   map[string]GroupConfig    `yaml:"groups"`
	Overrides map[string]ToolOverride  `yaml:"overrides"`
}

// MethodDefaults defines default MCP annotations for an HTTP method.
type MethodDefaults struct {
	ReadOnly    *bool `yaml:"readonly"`
	Destructive *bool `yaml:"destructive"`
	Idempotent  *bool `yaml:"idempotent"`
}

// GroupConfig defines an MCP tool group.
type GroupConfig struct {
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
}

// ToolOverride allows per-tool customization.
type ToolOverride struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	ReadOnly    *bool  `yaml:"readonly"`
	Destructive *bool  `yaml:"destructive"`
	Idempotent  *bool  `yaml:"idempotent"`
}

// LoadConfig reads and parses the mcp.yaml configuration file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	return ParseConfig(data)
}

// ParseConfig parses mcp.yaml configuration from raw bytes.
func ParseConfig(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse mcp config: %w", err)
	}

	if cfg.APIURL == "" {
		cfg.APIURL = "http://127.0.0.1:8090"
	}
	if cfg.SpecPath == "" {
		cfg.SpecPath = "/docs/openapi.json"
	}

	return &cfg, nil
}

// tagToGroup builds a reverse mapping from OpenAPI tag to group name.
func (c *Config) tagToGroup() map[string]string {
	m := make(map[string]string)
	for group, gc := range c.Groups {
		for _, tag := range gc.Tags {
			m[tag] = group
		}
	}
	return m
}
