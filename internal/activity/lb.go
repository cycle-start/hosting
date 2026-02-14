package activity

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/rs/zerolog"
)

const (
	haproxyMapPath      = "/var/lib/haproxy/maps/fqdn-to-shard.map"
	haproxyRuntimeAddr  = "localhost:9999"
)

// NodeLB contains activities for managing the local HAProxy load balancer
// map files via the HAProxy Runtime API. These activities run on LB node-agents
// and always connect to localhost:9999.
type NodeLB struct {
	logger zerolog.Logger
}

// NewNodeLB creates a new NodeLB activity struct.
func NewNodeLB(logger zerolog.Logger) *NodeLB {
	return &NodeLB{logger: logger}
}

// SetLBMapEntryParams holds parameters for SetLBMapEntry.
type SetLBMapEntryParams struct {
	FQDN      string `json:"fqdn"`
	LBBackend string `json:"lb_backend"`
}

// DeleteLBMapEntryParams holds parameters for DeleteLBMapEntry.
type DeleteLBMapEntryParams struct {
	FQDN string `json:"fqdn"`
}

// SetLBMapEntry sets a mapping from an FQDN to an LB backend in the HAProxy map file
// via the Runtime API. It uses "set map" to update existing entries, falling back
// to "add map" for new entries.
func (a *NodeLB) SetLBMapEntry(ctx context.Context, params SetLBMapEntryParams) error {
	// Try "set map" first (updates existing entry).
	resp, err := haproxyCommand(haproxyRuntimeAddr, fmt.Sprintf("set map %s %s %s\n", haproxyMapPath, params.FQDN, params.LBBackend))
	if err != nil {
		return fmt.Errorf("set map entry %s -> %s: %w", params.FQDN, params.LBBackend, err)
	}

	if strings.Contains(strings.ToLower(resp), "not found") {
		// Entry doesn't exist yet — use "add map" to create it.
		resp, err = haproxyCommand(haproxyRuntimeAddr, fmt.Sprintf("add map %s %s %s\n", haproxyMapPath, params.FQDN, params.LBBackend))
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
func (a *NodeLB) DeleteLBMapEntry(ctx context.Context, params DeleteLBMapEntryParams) error {
	resp, err := haproxyCommand(haproxyRuntimeAddr, fmt.Sprintf("del map %s %s\n", haproxyMapPath, params.FQDN))
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
