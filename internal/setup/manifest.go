package setup

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const ManifestFilename = "setup.yaml"

// LoadManifest reads a setup manifest from disk.
func LoadManifest(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	return &cfg, nil
}

// WriteManifest writes the setup manifest to disk.
func WriteManifest(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	header := "# Hosting platform setup manifest.\n" +
		"# Created by the setup wizard. Edit by hand or re-run `setup` to modify.\n" +
		"# Run `setup generate` to regenerate deployment files from this manifest.\n\n"

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	if err := os.WriteFile(path, []byte(header+string(data)), 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}
