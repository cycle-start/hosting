package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	configDirName = "hosting"
	profilesDir   = "profiles"
	stateFile     = "state.json"
)

// Profile represents a saved WireGuard profile associated with a tenant.
type Profile struct {
	Name     string `json:"name"`
	TenantID string `json:"tenant_id"`
	FilePath string `json:"file_path"` // path to the .conf file
}

// State holds the active profile selection.
type State struct {
	ActiveProfile string `json:"active_profile"`
}

// configDir returns the base config directory (~/.config/hosting/).
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		xdgConfig = filepath.Join(home, ".config")
	}

	return filepath.Join(xdgConfig, configDirName), nil
}

// ensureConfigDir creates the config directory structure if needed.
func ensureConfigDir() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Join(dir, profilesDir), 0700); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}

	return dir, nil
}

// Import copies a WireGuard config file into the profile store.
// The name parameter is used as the profile name. If empty, it's derived from the filename.
// The tenantID parameter associates the profile with a tenant for context switching.
func Import(configPath, name, tenantID string) (*Profile, error) {
	dir, err := ensureConfigDir()
	if err != nil {
		return nil, err
	}

	// Validate the config.
	cfg, err := ParseConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	_ = cfg // validated

	// Derive name from filename if not provided.
	if name == "" {
		base := filepath.Base(configPath)
		name = strings.TrimSuffix(base, filepath.Ext(base))
	}

	// Sanitize name.
	name = sanitizeName(name)

	// Copy the config file.
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	destPath := filepath.Join(dir, profilesDir, name+".conf")
	if err := os.WriteFile(destPath, data, 0600); err != nil {
		return nil, fmt.Errorf("write profile: %w", err)
	}

	// Save profile metadata.
	profile := &Profile{
		Name:     name,
		TenantID: tenantID,
		FilePath: destPath,
	}

	metaPath := filepath.Join(dir, profilesDir, name+".json")
	metaData, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal profile metadata: %w", err)
	}
	if err := os.WriteFile(metaPath, metaData, 0600); err != nil {
		return nil, fmt.Errorf("write profile metadata: %w", err)
	}

	return profile, nil
}

// List returns all saved profiles.
func ListProfiles() ([]Profile, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}

	pDir := filepath.Join(dir, profilesDir)
	entries, err := os.ReadDir(pDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read profiles directory: %w", err)
	}

	var profiles []Profile
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(pDir, entry.Name()))
		if err != nil {
			continue
		}

		var p Profile
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		profiles = append(profiles, p)
	}

	return profiles, nil
}

// LoadProfile loads a profile by name.
func LoadProfile(name string) (*Profile, *WireGuardConfig, error) {
	dir, err := configDir()
	if err != nil {
		return nil, nil, err
	}

	metaPath := filepath.Join(dir, profilesDir, name+".json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, nil, fmt.Errorf("profile %q not found: %w", name, err)
	}

	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, nil, fmt.Errorf("parse profile metadata: %w", err)
	}

	cfg, err := ParseConfig(p.FilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("parse profile config: %w", err)
	}

	return &p, cfg, nil
}

// DeleteProfile removes a saved profile.
func DeleteProfile(name string) error {
	dir, err := configDir()
	if err != nil {
		return err
	}

	pDir := filepath.Join(dir, profilesDir)
	os.Remove(filepath.Join(pDir, name+".conf"))
	os.Remove(filepath.Join(pDir, name+".json"))

	// If this was the active profile, clear it.
	state, _ := loadState()
	if state != nil && state.ActiveProfile == name {
		state.ActiveProfile = ""
		saveState(state)
	}

	return nil
}

// SetActive sets the active profile.
func SetActive(name string) error {
	// Verify profile exists.
	_, _, err := LoadProfile(name)
	if err != nil {
		return err
	}

	state := &State{ActiveProfile: name}
	return saveState(state)
}

// GetActive returns the currently active profile name.
func GetActive() (string, error) {
	state, err := loadState()
	if err != nil {
		return "", nil // no state file = no active profile
	}
	return state.ActiveProfile, nil
}

func loadState() (*State, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filepath.Join(dir, stateFile))
	if err != nil {
		return nil, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

func saveState(state *State) error {
	dir, err := ensureConfigDir()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, stateFile), data, 0600)
}

func sanitizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, name)
	return strings.Trim(name, "-")
}
