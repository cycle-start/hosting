package cli

import (
	"bufio"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
)

// WireGuardConfig represents a parsed WireGuard configuration file.
type WireGuardConfig struct {
	// Interface section
	PrivateKey string
	Address    netip.Prefix

	// Peer section
	PublicKey           string
	PresharedKey        string
	Endpoint            string
	AllowedIPs          []netip.Prefix
	PersistentKeepalive int

	// Service metadata (parsed from comments)
	Services []ServiceEntry
}

// ServiceEntry represents a service reachable through the tunnel.
type ServiceEntry struct {
	Type    string // "mysql" or "valkey"
	Address string // IPv6 ULA address
}

// DefaultPort returns the default local port for a service type.
func (s ServiceEntry) DefaultPort() int {
	switch s.Type {
	case "mysql":
		return 3306
	case "valkey":
		return 6379
	default:
		return 0
	}
}

// RemotePort returns the port the service listens on remotely.
func (s ServiceEntry) RemotePort() int {
	return s.DefaultPort()
}

// ParseConfig reads and parses a WireGuard config file.
func ParseConfig(path string) (*WireGuardConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()

	return ParseConfigReader(bufio.NewScanner(f))
}

// ParseConfigString parses a WireGuard config from a string.
func ParseConfigString(data string) (*WireGuardConfig, error) {
	return ParseConfigReader(bufio.NewScanner(strings.NewReader(data)))
}

// ParseConfigReader parses a WireGuard config from a scanner.
func ParseConfigReader(scanner *bufio.Scanner) (*WireGuardConfig, error) {
	cfg := &WireGuardConfig{}
	section := ""
	inServices := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Parse service metadata from comments.
		if strings.HasPrefix(line, "# hosting-cli:services") {
			inServices = true
			continue
		}
		if inServices && strings.HasPrefix(line, "# ") {
			parts := strings.SplitN(strings.TrimPrefix(line, "# "), "=", 2)
			if len(parts) == 2 {
				cfg.Services = append(cfg.Services, ServiceEntry{
					Type:    strings.TrimSpace(parts[0]),
					Address: strings.TrimSpace(parts[1]),
				})
			}
			continue
		}
		if inServices && !strings.HasPrefix(line, "#") {
			inServices = false
		}

		// Skip empty lines and other comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Section headers.
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.Trim(line, "[]"))
			continue
		}

		// Key-value pairs.
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)

		switch section {
		case "interface":
			switch key {
			case "PrivateKey":
				cfg.PrivateKey = val
			case "Address":
				prefix, err := netip.ParsePrefix(val)
				if err != nil {
					return nil, fmt.Errorf("parse address %q: %w", val, err)
				}
				cfg.Address = prefix
			}
		case "peer":
			switch key {
			case "PublicKey":
				cfg.PublicKey = val
			case "PresharedKey":
				cfg.PresharedKey = val
			case "Endpoint":
				cfg.Endpoint = val
			case "AllowedIPs":
				for _, cidr := range strings.Split(val, ",") {
					cidr = strings.TrimSpace(cidr)
					prefix, err := netip.ParsePrefix(cidr)
					if err != nil {
						return nil, fmt.Errorf("parse allowed IP %q: %w", cidr, err)
					}
					cfg.AllowedIPs = append(cfg.AllowedIPs, prefix)
				}
			case "PersistentKeepalive":
				n, err := strconv.Atoi(val)
				if err != nil {
					return nil, fmt.Errorf("parse keepalive %q: %w", val, err)
				}
				cfg.PersistentKeepalive = n
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	if cfg.PrivateKey == "" {
		return nil, fmt.Errorf("missing PrivateKey in [Interface]")
	}
	if cfg.PublicKey == "" {
		return nil, fmt.Errorf("missing PublicKey in [Peer]")
	}

	return cfg, nil
}
