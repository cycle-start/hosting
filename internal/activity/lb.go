package activity

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

const (
	haproxyMapPath       = "/var/lib/haproxy/maps/fqdn-to-shard.map"
	defaultHAProxyAdmin  = "localhost:9999"
)

// LB contains activities for managing the cluster-level HAProxy load balancer
// map files via the HAProxy Runtime API over TCP.
type LB struct {
	db DB
}

// NewLB creates a new LB activity struct.
func NewLB(db DB) *LB {
	return &LB{db: db}
}

// SetLBMapEntryParams holds parameters for SetLBMapEntry.
type SetLBMapEntryParams struct {
	ClusterID string `json:"cluster_id"`
	FQDN      string `json:"fqdn"`
	LBBackend string `json:"lb_backend"`
}

// DeleteLBMapEntryParams holds parameters for DeleteLBMapEntry.
type DeleteLBMapEntryParams struct {
	ClusterID string `json:"cluster_id"`
	FQDN      string `json:"fqdn"`
}

// SetLBMapEntry sets a mapping from an FQDN to an LB backend in the HAProxy map file
// via the Runtime API. It uses "set map" to update existing entries, falling back
// to "add map" for new entries.
func (a *LB) SetLBMapEntry(ctx context.Context, params SetLBMapEntryParams) error {
	addr, err := a.resolveHAProxyAddr(ctx, params.ClusterID)
	if err != nil {
		return fmt.Errorf("resolve haproxy: %w", err)
	}

	// Try "set map" first (updates existing entry).
	resp, err := haproxyCommand(addr, fmt.Sprintf("set map %s %s %s\n", haproxyMapPath, params.FQDN, params.LBBackend))
	if err != nil {
		return fmt.Errorf("set map entry %s -> %s: %w", params.FQDN, params.LBBackend, err)
	}

	if strings.Contains(strings.ToLower(resp), "not found") {
		// Entry doesn't exist yet — use "add map" to create it.
		resp, err = haproxyCommand(addr, fmt.Sprintf("add map %s %s %s\n", haproxyMapPath, params.FQDN, params.LBBackend))
		if err != nil {
			return fmt.Errorf("add map entry %s -> %s: %w", params.FQDN, params.LBBackend, err)
		}
		resp = strings.TrimSpace(resp)
		if resp != "" && strings.Contains(strings.ToLower(resp), "err") {
			return fmt.Errorf("add map entry %s -> %s: %s", params.FQDN, params.LBBackend, resp)
		}
	}

	return nil
}

// DeleteLBMapEntry removes an FQDN mapping from the HAProxy map file
// via the Runtime API.
func (a *LB) DeleteLBMapEntry(ctx context.Context, params DeleteLBMapEntryParams) error {
	addr, err := a.resolveHAProxyAddr(ctx, params.ClusterID)
	if err != nil {
		return fmt.Errorf("resolve haproxy: %w", err)
	}

	resp, err := haproxyCommand(addr, fmt.Sprintf("del map %s %s\n", haproxyMapPath, params.FQDN))
	if err != nil {
		return fmt.Errorf("del map entry %s: %w", params.FQDN, err)
	}

	// Ignore "not found" — the entry may already be gone.
	resp = strings.TrimSpace(resp)
	if resp != "" && !strings.Contains(strings.ToLower(resp), "not found") && strings.Contains(strings.ToLower(resp), "err") {
		return fmt.Errorf("del map entry %s: %s", params.FQDN, resp)
	}
	return nil
}

// resolveHAProxyAddr reads the HAProxy admin address from the cluster config JSON.
// Falls back to defaultHAProxyAdmin if not set.
func (a *LB) resolveHAProxyAddr(ctx context.Context, clusterID string) (string, error) {
	var configRaw json.RawMessage
	err := a.db.QueryRow(ctx,
		`SELECT config FROM clusters WHERE id = $1`, clusterID,
	).Scan(&configRaw)
	if err != nil {
		return "", fmt.Errorf("get cluster %s: %w", clusterID, err)
	}

	var cfg struct {
		HAProxyAdminAddr string `json:"haproxy_admin_addr"`
	}
	if json.Unmarshal(configRaw, &cfg) == nil && cfg.HAProxyAdminAddr != "" {
		return cfg.HAProxyAdminAddr, nil
	}

	return defaultHAProxyAdmin, nil
}

// haproxyCommand sends a command to HAProxy's Runtime API via TCP and returns
// the response line.
func haproxyCommand(addr, cmd string) (string, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return "", fmt.Errorf("connect to haproxy at %s: %w", addr, err)
	}
	defer conn.Close()

	if _, err := fmt.Fprint(conn, cmd); err != nil {
		return "", fmt.Errorf("send command: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return strings.Join(lines, "\n"), scanner.Err()
}
