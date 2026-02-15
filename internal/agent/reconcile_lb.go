package agent

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/edvin/hosting/internal/model"
)

func (r *Reconciler) reconcileLB(ctx context.Context, ds *model.DesiredState) ([]DriftEvent, error) {
	var events []DriftEvent
	fixes := 0

	// Read current map file.
	mapFile := "/var/lib/haproxy/maps/fqdn-to-shard.map"
	currentMap := make(map[string]string)
	if f, err := os.Open(mapFile); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				currentMap[parts[0]] = parts[1]
			}
		}
		f.Close()
	}

	// Build desired map.
	desiredMap := make(map[string]string)
	for _, m := range ds.FQDNMappings {
		desiredMap[m.FQDN] = m.LBBackend
	}

	// Add missing entries.
	for fqdn, backend := range desiredMap {
		if _, ok := currentMap[fqdn]; !ok {
			if !r.circuitOpen && fixes < r.maxFixes {
				if err := haproxySetMap(mapFile, fqdn, backend); err == nil {
					events = append(events, DriftEvent{
						Timestamp: time.Now(), NodeID: r.nodeID, Kind: "lb_map",
						Resource: fqdn, Action: "auto_fixed",
						Detail: fmt.Sprintf("added missing map entry -> %s", backend),
					})
					fixes++
				} else {
					events = append(events, DriftEvent{
						Timestamp: time.Now(), NodeID: r.nodeID, Kind: "lb_map",
						Resource: fqdn, Action: "reported",
						Detail: fmt.Sprintf("failed to add map entry: %v", err),
					})
				}
			}
		}
	}

	// Remove orphaned entries (safe for LB -- stateless metadata).
	for fqdn := range currentMap {
		if _, ok := desiredMap[fqdn]; !ok {
			if !r.circuitOpen && fixes < r.maxFixes {
				if err := haproxyDelMap(mapFile, fqdn); err == nil {
					events = append(events, DriftEvent{
						Timestamp: time.Now(), NodeID: r.nodeID, Kind: "lb_map",
						Resource: fqdn, Action: "auto_fixed",
						Detail: "removed orphaned map entry",
					})
					fixes++
				} else {
					events = append(events, DriftEvent{
						Timestamp: time.Now(), NodeID: r.nodeID, Kind: "lb_map",
						Resource: fqdn, Action: "reported",
						Detail: fmt.Sprintf("failed to remove map entry: %v", err),
					})
				}
			}
		}
	}

	return events, nil
}

func haproxySetMap(mapFile, fqdn, backend string) error {
	return haproxyRuntimeCmd(fmt.Sprintf("set map %s %s %s\n", mapFile, fqdn, backend))
}

func haproxyDelMap(mapFile, fqdn string) error {
	return haproxyRuntimeCmd(fmt.Sprintf("del map %s %s\n", mapFile, fqdn))
}

func haproxyRuntimeCmd(cmd string) error {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:9999", 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	_, err = conn.Write([]byte(cmd))
	return err
}
