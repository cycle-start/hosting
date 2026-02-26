package activity

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/rs/zerolog"
)

// mapFileMu serializes writes to the on-disk map file.
var mapFileMu sync.Mutex

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
// to "add map" for new entries. The on-disk map file is also updated so entries
// survive HAProxy restarts.
func (a *NodeLB) SetLBMapEntry(ctx context.Context, params SetLBMapEntryParams) error {
	// Strip trailing dot from FQDN — HAProxy matches against the Host header
	// which never includes the DNS trailing dot.
	fqdn := strings.TrimSuffix(params.FQDN, ".")

	a.logger.Info().Str("fqdn", fqdn).Str("backend", params.LBBackend).Msg("setting LB map entry")

	// Try "set map" first (updates existing entry).
	resp, err := haproxyCommand(haproxyRuntimeAddr, fmt.Sprintf("set map %s %s %s\n", haproxyMapPath, fqdn, params.LBBackend))
	if err != nil {
		return fmt.Errorf("set map entry %s -> %s: %w", fqdn, params.LBBackend, err)
	}

	if strings.Contains(strings.ToLower(resp), "not found") {
		// Entry doesn't exist yet — use "add map" to create it.
		resp, err = haproxyCommand(haproxyRuntimeAddr, fmt.Sprintf("add map %s %s %s\n", haproxyMapPath, fqdn, params.LBBackend))
		if err != nil {
			return fmt.Errorf("add map entry %s -> %s: %w", fqdn, params.LBBackend, err)
		}
		resp = strings.TrimSpace(resp)
		if resp != "" && strings.Contains(strings.ToLower(resp), "err") {
			return fmt.Errorf("add map entry %s -> %s: %s", fqdn, params.LBBackend, resp)
		}
	}

	// Persist to on-disk map file so entries survive HAProxy restarts.
	if err := persistMapEntry(fqdn, params.LBBackend); err != nil {
		a.logger.Warn().Err(err).Str("fqdn", fqdn).Msg("failed to persist map entry to disk (runtime entry is set)")
	}

	return nil
}

// DeleteLBMapEntry removes an FQDN mapping from the HAProxy map file
// via the Runtime API. The on-disk map file is also updated.
func (a *NodeLB) DeleteLBMapEntry(ctx context.Context, params DeleteLBMapEntryParams) error {
	fqdn := strings.TrimSuffix(params.FQDN, ".")

	a.logger.Info().Str("fqdn", fqdn).Msg("deleting LB map entry")

	resp, err := haproxyCommand(haproxyRuntimeAddr, fmt.Sprintf("del map %s %s\n", haproxyMapPath, fqdn))
	if err != nil {
		return fmt.Errorf("del map entry %s: %w", fqdn, err)
	}

	// Ignore "not found" — the entry may already be gone.
	resp = strings.TrimSpace(resp)
	if resp != "" && !strings.Contains(strings.ToLower(resp), "not found") && strings.Contains(strings.ToLower(resp), "err") {
		return fmt.Errorf("del map entry %s: %s", fqdn, resp)
	}

	// Remove from on-disk map file.
	if err := removeMapEntry(fqdn); err != nil {
		a.logger.Warn().Err(err).Str("fqdn", fqdn).Msg("failed to remove map entry from disk (runtime entry is deleted)")
	}
	return nil
}

// persistMapEntry adds or updates an FQDN→backend mapping in the on-disk map file.
func persistMapEntry(fqdn, backend string) error {
	mapFileMu.Lock()
	defer mapFileMu.Unlock()

	entries, err := readMapFile(haproxyMapPath)
	if err != nil {
		return err
	}
	entries[fqdn] = backend
	return writeMapFile(haproxyMapPath, entries)
}

// removeMapEntry deletes an FQDN from the on-disk map file.
func removeMapEntry(fqdn string) error {
	mapFileMu.Lock()
	defer mapFileMu.Unlock()

	entries, err := readMapFile(haproxyMapPath)
	if err != nil {
		return err
	}
	delete(entries, fqdn)
	return writeMapFile(haproxyMapPath, entries)
}

// readMapFile reads the HAProxy map file into a map of fqdn→backend.
func readMapFile(path string) (map[string]string, error) {
	entries := make(map[string]string)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return entries, nil
		}
		return nil, fmt.Errorf("read map file: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			entries[parts[0]] = parts[1]
		}
	}
	return entries, nil
}

// writeMapFile atomically writes the map entries to disk.
func writeMapFile(path string, entries map[string]string) error {
	var buf strings.Builder
	for fqdn, backend := range entries {
		fmt.Fprintf(&buf, "%s %s\n", fqdn, backend)
	}
	return os.WriteFile(path, []byte(buf.String()), 0644)
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
