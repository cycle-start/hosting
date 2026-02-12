package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/edvin/hosting/internal/deployer"
	"github.com/edvin/hosting/internal/model"
)

const (
	defaultHAProxyContainer = "hosting-haproxy"
	haproxyMapPath          = "/var/lib/haproxy/maps/fqdn-to-shard.map"
	haproxySockPath         = "/var/run/haproxy/admin.sock"
)

// LB contains activities for managing the cluster-level HAProxy load balancer
// map files via the HAProxy Runtime API (socat).
type LB struct {
	deployer deployer.Deployer
	db       DB
}

// NewLB creates a new LB activity struct.
func NewLB(d deployer.Deployer, db DB) *LB {
	return &LB{deployer: d, db: db}
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
	host, container, err := a.resolveHAProxy(ctx, params.ClusterID)
	if err != nil {
		return fmt.Errorf("resolve haproxy: %w", err)
	}

	// Try "set map" first (updates existing entry).
	socatCmd := fmt.Sprintf("set map %s %s %s", haproxyMapPath, params.FQDN, params.LBBackend)
	result, err := a.deployer.ExecInContainer(ctx, host, container, []string{
		"sh", "-c", fmt.Sprintf("echo '%s' | socat stdio %s", socatCmd, haproxySockPath),
	})
	if err != nil {
		return fmt.Errorf("set map entry %s -> %s: %w", params.FQDN, params.LBBackend, err)
	}

	output := strings.TrimSpace(result.Stderr + result.Stdout)
	if strings.Contains(strings.ToLower(output), "not found") || result.ExitCode != 0 {
		// Entry doesn't exist yet — use "add map" to create it.
		addCmd := fmt.Sprintf("add map %s %s %s", haproxyMapPath, params.FQDN, params.LBBackend)
		result, err = a.deployer.ExecInContainer(ctx, host, container, []string{
			"sh", "-c", fmt.Sprintf("echo '%s' | socat stdio %s", addCmd, haproxySockPath),
		})
		if err != nil {
			return fmt.Errorf("add map entry %s -> %s: %w", params.FQDN, params.LBBackend, err)
		}
		if result.ExitCode != 0 {
			return fmt.Errorf("add map entry %s -> %s: haproxy returned exit %d: %s",
				params.FQDN, params.LBBackend, result.ExitCode, strings.TrimSpace(result.Stderr+result.Stdout))
		}
	}

	return nil
}

// DeleteLBMapEntry removes an FQDN mapping from the HAProxy map file
// via the Runtime API.
func (a *LB) DeleteLBMapEntry(ctx context.Context, params DeleteLBMapEntryParams) error {
	host, container, err := a.resolveHAProxy(ctx, params.ClusterID)
	if err != nil {
		return fmt.Errorf("resolve haproxy: %w", err)
	}

	socatCmd := fmt.Sprintf("del map %s %s", haproxyMapPath, params.FQDN)
	result, err := a.deployer.ExecInContainer(ctx, host, container, []string{
		"sh", "-c", fmt.Sprintf("echo '%s' | socat stdio %s", socatCmd, haproxySockPath),
	})
	if err != nil {
		return fmt.Errorf("del map entry %s: %w", params.FQDN, err)
	}
	// Ignore "not found" errors — the entry may already be gone.
	output := strings.TrimSpace(result.Stderr + result.Stdout)
	if result.ExitCode != 0 && !strings.Contains(strings.ToLower(output), "not found") {
		return fmt.Errorf("del map entry %s: haproxy returned exit %d: %s",
			params.FQDN, result.ExitCode, output)
	}
	return nil
}

// resolveHAProxy determines the HAProxy container name and a host machine to
// exec into for the given cluster. It reads haproxy_container from the cluster
// config JSON, falling back to the default.
func (a *LB) resolveHAProxy(ctx context.Context, clusterID string) (*model.HostMachine, string, error) {
	// Get the cluster to read config.
	var cluster model.Cluster
	err := a.db.QueryRow(ctx,
		`SELECT id, region_id, name, config, status, spec, created_at, updated_at
		 FROM clusters WHERE id = $1`, clusterID,
	).Scan(&cluster.ID, &cluster.RegionID, &cluster.Name, &cluster.Config,
		&cluster.Status, &cluster.Spec, &cluster.CreatedAt, &cluster.UpdatedAt)
	if err != nil {
		return nil, "", fmt.Errorf("get cluster %s: %w", clusterID, err)
	}

	containerName := defaultHAProxyContainer
	var cfg struct {
		HAProxyContainer string `json:"haproxy_container"`
	}
	if json.Unmarshal(cluster.Config, &cfg) == nil && cfg.HAProxyContainer != "" {
		containerName = cfg.HAProxyContainer
	}

	// Get any active host machine in the cluster for Docker access.
	var host model.HostMachine
	err = a.db.QueryRow(ctx,
		`SELECT id, cluster_id, hostname, ip_address::text, docker_host, ca_cert_pem, client_cert_pem, client_key_pem, capacity, roles, status, created_at, updated_at
		 FROM host_machines WHERE cluster_id = $1 AND status = $2
		 LIMIT 1`, clusterID, model.StatusActive,
	).Scan(&host.ID, &host.ClusterID, &host.Hostname, &host.IPAddress, &host.DockerHost,
		&host.CACertPEM, &host.ClientCertPEM, &host.ClientKeyPEM,
		&host.Capacity, &host.Roles, &host.Status, &host.CreatedAt, &host.UpdatedAt)
	if err != nil {
		return nil, "", fmt.Errorf("find active host for cluster %s: %w", clusterID, err)
	}

	return &host, containerName, nil
}
