package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/rs/zerolog"
)

const phpPoolTemplate = `[{{ .TenantName }}]
user = {{ .TenantName }}
group = {{ .TenantName }}

listen = /run/php/{{ .TenantName }}-php{{ .Version }}.sock
listen.owner = www-data
listen.group = www-data
listen.mode = 0660

pm = dynamic
pm.max_children = {{ .MaxChildren }}
pm.start_servers = {{ .StartServers }}
pm.min_spare_servers = {{ .MinSpareServers }}
pm.max_spare_servers = {{ .MaxSpareServers }}
pm.max_requests = {{ .MaxRequests }}

php_admin_value[error_log] = /var/log/hosting/{{ .TenantID }}/php-error.log
php_admin_value[slowlog] = /var/log/hosting/{{ .TenantID }}/php-slow.log
php_admin_value[request_slowlog_timeout] = 5s
php_admin_flag[log_errors] = on
php_admin_value[open_basedir] = /var/www/storage/{{ .TenantName }}/:/tmp/
{{ range .PHPValues }}php_value[{{ .Key }}] = {{ .Value }}
{{ end }}{{ range .PHPAdminValues }}php_admin_value[{{ .Key }}] = {{ .Value }}
{{ end }}{{ range .EnvVars }}env[{{ .Key }}] = {{ .Value }}
{{ end }}`

var phpPoolTmpl = template.Must(template.New("phppool").Parse(phpPoolTemplate))

// phpBlocklistedAdminKeys are php_admin_value keys that cannot be overridden
// because they are managed by the platform for security.
var phpBlocklistedAdminKeys = map[string]bool{
	"open_basedir":      true,
	"error_log":         true,
	"slowlog":           true,
	"disable_functions": true,
	"doc_root":          true,
}

// PHPRuntimeConfig represents the parsed runtime_config JSON for PHP webroots.
type PHPRuntimeConfig struct {
	PM             *PHPPMConfig      `json:"pm,omitempty"`
	PHPValues      map[string]string `json:"php_values,omitempty"`
	PHPAdminValues map[string]string `json:"php_admin_values,omitempty"`
}

// PHPPMConfig holds PHP-FPM process manager settings.
type PHPPMConfig struct {
	MaxChildren    *int `json:"max_children,omitempty"`
	StartServers   *int `json:"start_servers,omitempty"`
	MinSpareServers *int `json:"min_spare_servers,omitempty"`
	MaxSpareServers *int `json:"max_spare_servers,omitempty"`
	MaxRequests    *int `json:"max_requests,omitempty"`
}

// ValidatePHPRuntimeConfig parses and validates runtime_config for a PHP webroot.
// Returns a non-nil error if validation fails.
func ValidatePHPRuntimeConfig(raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
		return nil
	}

	var cfg PHPRuntimeConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("invalid php runtime_config: %w", err)
	}

	if cfg.PM != nil {
		if err := validatePMRange("max_children", cfg.PM.MaxChildren, 1, 200); err != nil {
			return err
		}
		if err := validatePMRange("start_servers", cfg.PM.StartServers, 1, 200); err != nil {
			return err
		}
		if err := validatePMRange("min_spare_servers", cfg.PM.MinSpareServers, 1, 200); err != nil {
			return err
		}
		if err := validatePMRange("max_spare_servers", cfg.PM.MaxSpareServers, 1, 200); err != nil {
			return err
		}
		if err := validatePMRange("max_requests", cfg.PM.MaxRequests, 0, 100000); err != nil {
			return err
		}
	}

	for key := range cfg.PHPAdminValues {
		if phpBlocklistedAdminKeys[key] {
			return fmt.Errorf("php_admin_values key %q is managed by the platform and cannot be overridden", key)
		}
	}

	return nil
}

func validatePMRange(name string, val *int, min, max int) error {
	if val == nil {
		return nil
	}
	if *val < min || *val > max {
		return fmt.Errorf("pm.%s must be between %d and %d, got %d", name, min, max, *val)
	}
	return nil
}

// kvPair is used to pass sorted key-value pairs into the template.
type kvPair struct {
	Key   string
	Value string
}

type phpPoolData struct {
	TenantName      string
	TenantID        string
	Version         string
	MaxChildren     int
	StartServers    int
	MinSpareServers int
	MaxSpareServers int
	MaxRequests     int
	PHPValues       []kvPair
	PHPAdminValues  []kvPair
	EnvVars         []kvPair
}

// PHP manages PHP-FPM pool configuration and lifecycle.
type PHP struct {
	logger zerolog.Logger
	svcMgr ServiceManager
}

// NewPHP creates a new PHP runtime manager.
func NewPHP(logger zerolog.Logger, svcMgr ServiceManager) *PHP {
	return &PHP{
		logger: logger.With().Str("runtime", "php").Logger(),
		svcMgr: svcMgr,
	}
}

func (p *PHP) poolConfigPath(webroot *WebrootInfo) string {
	version := webroot.RuntimeVersion
	if version == "" {
		version = "8.5"
	}
	return filepath.Join("/etc/php", version, "fpm/pool.d", webroot.TenantName+".conf")
}

func (p *PHP) fpmServiceName(webroot *WebrootInfo) string {
	version := webroot.RuntimeVersion
	if version == "" {
		version = "8.5"
	}
	return fmt.Sprintf("php%s-fpm", version)
}

// Configure generates and writes a PHP-FPM pool configuration file for the tenant.
func (p *PHP) Configure(ctx context.Context, webroot *WebrootInfo) error {
	version := webroot.RuntimeVersion
	if version == "" {
		version = "8.5"
	}

	// Parse runtime config.
	var cfg PHPRuntimeConfig
	if webroot.RuntimeConfig != "" && webroot.RuntimeConfig != "{}" {
		if err := json.Unmarshal([]byte(webroot.RuntimeConfig), &cfg); err != nil {
			return fmt.Errorf("parse php runtime config: %w", err)
		}
	}

	data := phpPoolData{
		TenantName:      webroot.TenantName,
		TenantID:        webroot.TenantName,
		Version:         version,
		MaxChildren:     intOrDefault(cfg.PM, func(pm *PHPPMConfig) *int { return pm.MaxChildren }, 5),
		StartServers:    intOrDefault(cfg.PM, func(pm *PHPPMConfig) *int { return pm.StartServers }, 2),
		MinSpareServers: intOrDefault(cfg.PM, func(pm *PHPPMConfig) *int { return pm.MinSpareServers }, 1),
		MaxSpareServers: intOrDefault(cfg.PM, func(pm *PHPPMConfig) *int { return pm.MaxSpareServers }, 3),
		MaxRequests:     intOrDefault(cfg.PM, func(pm *PHPPMConfig) *int { return pm.MaxRequests }, 500),
		PHPValues:       sortedKVPairs(cfg.PHPValues),
		PHPAdminValues:  sortedKVPairs(cfg.PHPAdminValues),
		EnvVars:         sortedKVPairs(webroot.EnvVars),
	}

	var buf bytes.Buffer
	if err := phpPoolTmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("render php pool template: %w", err)
	}

	configPath := p.poolConfigPath(webroot)

	p.logger.Info().
		Str("tenant", webroot.TenantName).
		Str("version", version).
		Str("path", configPath).
		Msg("writing PHP-FPM pool config")

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("create php pool config dir: %w", err)
	}

	if err := os.WriteFile(configPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write php pool config: %w", err)
	}

	return nil
}

// Start reloads PHP-FPM to pick up the new pool configuration.
func (p *PHP) Start(ctx context.Context, webroot *WebrootInfo) error {
	service := p.fpmServiceName(webroot)
	p.logger.Info().
		Str("tenant", webroot.TenantName).
		Str("service", service).
		Msg("reloading PHP-FPM to start pool")

	// PHP-FPM uses USR2 for graceful reload.
	return p.svcMgr.Signal(ctx, service, "USR2")
}

// Stop removes the pool configuration and reloads PHP-FPM.
func (p *PHP) Stop(ctx context.Context, webroot *WebrootInfo) error {
	configPath := p.poolConfigPath(webroot)
	service := p.fpmServiceName(webroot)

	p.logger.Info().
		Str("tenant", webroot.TenantName).
		Str("path", configPath).
		Msg("removing PHP-FPM pool config and reloading")

	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove php pool config: %w", err)
	}

	return p.svcMgr.Signal(ctx, service, "USR2")
}

// Reload triggers a graceful reload of the PHP-FPM service.
func (p *PHP) Reload(ctx context.Context, webroot *WebrootInfo) error {
	service := p.fpmServiceName(webroot)
	p.logger.Info().
		Str("tenant", webroot.TenantName).
		Str("service", service).
		Msg("reloading PHP-FPM")

	return p.svcMgr.Signal(ctx, service, "USR2")
}

// Remove removes the pool configuration and reloads PHP-FPM.
func (p *PHP) Remove(ctx context.Context, webroot *WebrootInfo) error {
	return p.Stop(ctx, webroot)
}

// ReloadAll reloads all installed PHP-FPM services by scanning /etc/php/*/fpm/.
func (p *PHP) ReloadAll(ctx context.Context) error {
	versionDirs, err := filepath.Glob("/etc/php/*/fpm")
	if err != nil {
		return fmt.Errorf("glob php fpm dirs: %w", err)
	}
	for _, dir := range versionDirs {
		version := filepath.Base(filepath.Dir(dir))
		service := fmt.Sprintf("php%s-fpm", version)
		p.logger.Info().Str("service", service).Msg("reloading PHP-FPM")
		if err := p.svcMgr.Signal(ctx, service, "USR2"); err != nil {
			return fmt.Errorf("reload %s: %w", service, err)
		}
	}
	return nil
}

// CleanOrphanedPools removes PHP-FPM pool configs that are not in the expected
// set. It scans all /etc/php/*/fpm/pool.d/ directories. Returns the list of
// removed filenames. Does NOT reload PHP-FPM (caller handles that).
func (p *PHP) CleanOrphanedPools(expectedPools map[string]bool) ([]string, error) {
	versionDirs, err := filepath.Glob("/etc/php/*/fpm/pool.d")
	if err != nil {
		return nil, fmt.Errorf("glob php pool dirs: %w", err)
	}

	var removed []string
	for _, poolDir := range versionDirs {
		entries, err := os.ReadDir(poolDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".conf") {
				continue
			}
			// Skip the default www.conf pool.
			if name == "www.conf" {
				continue
			}
			if expectedPools[name] {
				continue
			}

			confPath := filepath.Join(poolDir, name)
			p.logger.Warn().Str("path", confPath).Str("filename", name).Msg("removing orphaned PHP-FPM pool config")
			if err := os.Remove(confPath); err != nil {
				return removed, fmt.Errorf("remove orphaned pool %s: %w", confPath, err)
			}
			removed = append(removed, confPath)
		}
	}

	return removed, nil
}

func intOrDefault(pm *PHPPMConfig, getter func(*PHPPMConfig) *int, def int) int {
	if pm == nil {
		return def
	}
	v := getter(pm)
	if v == nil {
		return def
	}
	return *v
}

func sortedKVPairs(m map[string]string) []kvPair {
	if len(m) == 0 {
		return nil
	}
	pairs := make([]kvPair, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, kvPair{Key: k, Value: v})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].Key < pairs[j].Key })
	return pairs
}
