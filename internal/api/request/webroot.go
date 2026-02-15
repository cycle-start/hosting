package request

import (
	"encoding/json"
	"fmt"
	"strings"
)

type CreateWebroot struct {
	Runtime        string          `json:"runtime" validate:"required,oneof=php php-worker node python ruby static"`
	RuntimeVersion string          `json:"runtime_version" validate:"required"`
	RuntimeConfig  json.RawMessage `json:"runtime_config"`
	PublicFolder   string          `json:"public_folder"`
	FQDNs          []CreateFQDNNested `json:"fqdns" validate:"omitempty,dive"`
}

type UpdateWebroot struct {
	Runtime        string          `json:"runtime" validate:"omitempty,oneof=php php-worker node python ruby static"`
	RuntimeVersion string          `json:"runtime_version"`
	RuntimeConfig  json.RawMessage `json:"runtime_config"`
	PublicFolder   *string         `json:"public_folder"`
}

// ValidateWorkerConfig validates runtime_config for php-worker webroots.
func ValidateWorkerConfig(config json.RawMessage) error {
	if len(config) == 0 || string(config) == "{}" || string(config) == "null" {
		return fmt.Errorf("runtime_config with command is required for php-worker runtime")
	}

	var wc struct {
		Command    string `json:"command"`
		NumProcs   int    `json:"num_procs"`
		StopSignal string `json:"stop_signal"`
	}
	if err := json.Unmarshal(config, &wc); err != nil {
		return fmt.Errorf("invalid runtime_config: %w", err)
	}

	if wc.Command == "" {
		return fmt.Errorf("command is required for php-worker runtime")
	}

	if len(wc.Command) > 1024 {
		return fmt.Errorf("command must be 1024 characters or less")
	}

	// Must start with php
	first := strings.Fields(wc.Command)[0]
	if first != "php" && !strings.HasPrefix(first, "/usr/bin/php") {
		return fmt.Errorf("command must start with 'php' for php-worker runtime")
	}

	// No shell metacharacters
	for _, ch := range []string{";", "|", "&", "$(", "`", ">", "<"} {
		if strings.Contains(wc.Command, ch) {
			return fmt.Errorf("command must not contain shell metacharacters")
		}
	}

	if wc.NumProcs < 0 || wc.NumProcs > 8 {
		return fmt.Errorf("num_procs must be between 0 and 8")
	}

	if wc.StopSignal != "" {
		valid := map[string]bool{"TERM": true, "INT": true, "QUIT": true, "USR1": true, "USR2": true}
		if !valid[wc.StopSignal] {
			return fmt.Errorf("stop_signal must be one of: TERM, INT, QUIT, USR1, USR2")
		}
	}

	return nil
}
